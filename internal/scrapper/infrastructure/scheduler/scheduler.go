package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository"
)

type OutboxRepository interface {
	FetchPending(ctx context.Context, limit int) ([]*repository.OutboxRecord, error)
	UpdateStatus(ctx context.Context, id int64, errIn error) error
	Cleanup(ctx context.Context, gap time.Duration) (int64, error)
}

type Transactor interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type Notifier interface {
	SendUpdate(ctx context.Context, update *application.ResourceShot) error
}

type Pusher interface {
	Push(ctx context.Context) error
}

type Runner interface {
	Start(context.Context) error
	Stop() error
}

type Config struct {
	BatchSize   int
	CrawlGap    time.Duration
	KafkaGap    time.Duration
	CleanGap    time.Duration
	MetricsPush time.Duration
}

type ScheduleUpdates struct {
	logger     *slog.Logger
	service    *application.CrawlerService
	outboxRepo OutboxRepository
	tx         Transactor
	notifier   Notifier
	scheduler  gocron.Scheduler
	pusher     Pusher

	batchSize   int
	crawlGap    time.Duration
	kafkaGap    time.Duration
	cleanGap    time.Duration
	metricsPush time.Duration
}

func NewScheduleUpdates(logger *slog.Logger, service *application.CrawlerService, repo OutboxRepository,
	tx Transactor, notifier Notifier, pusher Pusher, cfg Config,
) (*ScheduleUpdates, error) {
	logger = logger.With("module", "scheduler")

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &ScheduleUpdates{
		logger:      logger,
		service:     service,
		outboxRepo:  repo,
		tx:          tx,
		notifier:    notifier,
		scheduler:   scheduler,
		pusher:      pusher,
		batchSize:   cfg.BatchSize,
		crawlGap:    cfg.CrawlGap,
		kafkaGap:    cfg.KafkaGap,
		cleanGap:    cfg.CleanGap,
		metricsPush: cfg.MetricsPush,
	}, nil
}

func (s *ScheduleUpdates) Start(ctx context.Context) error {
	_, err := s.scheduler.NewJob(gocron.DurationJob(s.crawlGap),
		gocron.NewTask(s.service.NotifySubscribers, ctx),
		gocron.WithName("service-crawl"),
		gocron.WithContext(ctx))
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(gocron.DurationJob(s.kafkaGap),
		gocron.NewTask(s.relayOutbox, ctx),
		gocron.WithName("outbox-relay"),
		gocron.WithContext(ctx),
		gocron.WithSingletonMode(gocron.LimitModeReschedule))
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(gocron.DurationJob(s.cleanGap),
		gocron.NewTask(s.cleanOutbox, ctx),
		gocron.WithName("outbox-clean"),
		gocron.WithContext(ctx))
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(gocron.DurationJob(s.metricsPush),
		gocron.NewTask(s.pusher.Push, ctx),
		gocron.WithName("metrics-push"),
		gocron.WithContext(ctx))
	if err != nil {
		return err
	}

	s.scheduler.Start()

	return nil
}

func (s *ScheduleUpdates) Stop() error {
	return s.scheduler.Shutdown()
}

func (s *ScheduleUpdates) relayOutbox(ctx context.Context) error {
	return s.tx.WithTransaction(ctx, s.relayOutboxTx)
}

func (s *ScheduleUpdates) relayOutboxTx(ctx context.Context) error {
	records, err := s.outboxRepo.FetchPending(ctx, s.batchSize)
	if err != nil {
		s.logger.Error("fetch pending outbox", slog.Any("err", err))
		return fmt.Errorf("fetch pending outbox: %w", err)
	}

	for i := range records {
		err := s.notifier.SendUpdate(ctx, records[i].Shot)
		if dbErr := s.outboxRepo.UpdateStatus(ctx, int64(records[i].ID), err); dbErr != nil {
			s.logger.Error("update outbox status", slog.Any("err", dbErr))
		}
	}

	return nil
}

func (s *ScheduleUpdates) cleanOutbox(ctx context.Context) error {
	_, err := s.outboxRepo.Cleanup(ctx, s.cleanGap)
	if err != nil {
		s.logger.Error("outbox cleanup", slog.Any("err", err))
		return fmt.Errorf("outbox cleanup: %w", err)
	}

	return nil
}
