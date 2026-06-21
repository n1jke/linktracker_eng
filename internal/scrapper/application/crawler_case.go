package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type MetricsRecorder interface {
	SetLinksOnTrack(count int)
}

type ResourceShot struct {
	ID          int64
	URL         string
	EventType   string
	Description string
	Author      string
	ChatIDs     []int64
	UpdatedAt   time.Time
}

type CrawlerService struct {
	logger      *slog.Logger
	tx          Transactor
	subsRepo    SubscriptionsRepository
	linksRepo   LinksRepository
	outboxRepo  OutboxRepository
	crawler     SourceCrawler
	metrics     MetricsRecorder
	workerCount int
	batchSize   int
}

func NewCrawlerService(logger *slog.Logger, tx Transactor, subsRepo SubscriptionsRepository, linksRepo LinksRepository,
	outboxRepo OutboxRepository, crawler SourceCrawler, m MetricsRecorder, workerCount, batchSize int,
) *CrawlerService {
	logger = logger.With("module", "crawler")

	return &CrawlerService{
		logger:      logger,
		tx:          tx,
		subsRepo:    subsRepo,
		linksRepo:   linksRepo,
		outboxRepo:  outboxRepo,
		crawler:     crawler,
		metrics:     m,
		workerCount: workerCount,
		batchSize:   batchSize,
	}
}

func (cs *CrawlerService) NotifySubscribers(ctx context.Context) error {
	cs.logger.Info("notify subscribers start")

	var ptr *time.Time

	linksCount := 0

	for {
		links, nextPtr, err := cs.linksRepo.GetLinksBatch(ctx, cs.batchSize, ptr)
		if err != nil {
			cs.logger.Error("get links batch", slog.Any("err", err))
			return err
		}

		linksCount += len(links)
		cs.processLinks(ctx, links)

		if nextPtr == nil || len(links) < cs.batchSize {
			break
		}

		ptr = nextPtr
	}

	cs.metrics.SetLinksOnTrack(linksCount)
	cs.logger.Info("notify subscribers finished")

	return nil
}

func (cs *CrawlerService) processLinks(ctx context.Context, links []*domain.TrackedLink) {
	jobs := make(chan *domain.TrackedLink)

	// consumers(worker pool)
	wg := new(sync.WaitGroup)
	for range cs.workerCount {
		wg.Go(func() {
			for link := range jobs {
				err := cs.processLink(ctx, link)
				if err != nil {
					cs.logger.Error("process link", slog.String("link", link.Path()), slog.Any("err", err))
				}
			}
		})
	}

	// producer
	go func() {
		defer close(jobs)

		for i := range links {
			select {
			case jobs <- links[i]:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

func (cs *CrawlerService) processLink(ctx context.Context, link *domain.TrackedLink) error {
	subs, err := cs.subsRepo.GetSubscriptionsByResourceID(ctx, link.ID())
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get subscribers for link %s: %w", link.Path(), err)
		}

		return nil
	}

	chatIDs := make([]int64, 0, len(subs))
	for i := range subs {
		chatIDs = append(chatIDs, subs[i].UserID())
	}

	shots, err := cs.getResourceShot(ctx, link, chatIDs)
	if err != nil {
		errShot := &ResourceShot{
			URL:         link.Path(),
			Description: "some errors occurred while crawl your link subscription",
			ChatIDs:     chatIDs,
		}
		err := cs.tx.WithTransaction(ctx, func(ctx context.Context) error {
			return cs.processErrLinkTx(ctx, errShot)
		})

		return err
	}

	if len(shots) == 0 {
		return nil
	}

	userUpdate := make(map[int64]time.Time)
	updates := make([]*ResourceShot, 0, len(shots))

	for i := range shots {
		chatIDs = make([]int64, 0, len(subs))
		for j := range subs {
			userID := subs[j].UserID()
			if !shots[i].UpdatedAt.After(subs[j].LastUpdate()) {
				continue
			}

			chatIDs = append(chatIDs, userID)
			if shots[i].UpdatedAt.After(userUpdate[userID]) {
				userUpdate[userID] = shots[i].UpdatedAt
			}
		}

		if len(chatIDs) == 0 {
			continue
		}

		shots[i].ChatIDs = chatIDs
		updates = append(updates, shots[i])
	}

	if len(updates) == 0 {
		return nil
	}

	if err := cs.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return cs.processLinkTx(ctx, link, userUpdate, updates)
	}); err != nil {
		return err
	}

	return nil
}

func (cs *CrawlerService) getResourceShot(ctx context.Context, link *domain.TrackedLink, chatIDs []int64) ([]*ResourceShot, error) {
	cs.logger.Info("crawl resource start", slog.String("link", link.Path()))

	shots, err := cs.crawler.SearchResource(ctx, *link)
	if err != nil {
		cs.logger.Error("crawl resource", slog.String("link", link.Path()), slog.Any("err", err))
		return nil, err
	}

	for i := range shots {
		shots[i].ChatIDs = chatIDs
	}

	cs.logger.Info("resource crawled", slog.String("link", link.Path()))

	return shots, nil
}
