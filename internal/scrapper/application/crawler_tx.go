package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

func (cs *CrawlerService) processLinkTx(ctx context.Context, link *domain.TrackedLink,
	userUpdate map[int64]time.Time, updates []*ResourceShot,
) error {
	var errRepo error

	for userID, updatedAt := range userUpdate {
		if err := cs.subsRepo.UpdateSubscriptionLastUpdate(ctx, userID, link.ID(), updatedAt); err != nil {
			cs.logger.Error("update subscription last_update", slog.Int64("user_id", userID), slog.String("link", link.Path()), slog.Any("err", err))
			errRepo = errors.Join(errRepo, err)
		}
	}

	if errRepo != nil {
		return fmt.Errorf("last update in repo for %s: %w", link.Path(), errRepo)
	}

	if err := cs.outboxRepo.AddBatch(ctx, updates); err != nil {
		cs.logger.Error("send batch to outbox", slog.String("link", link.Path()), slog.Any("err", err))
		return fmt.Errorf("send batch to outbox: %w", err)
	}

	return nil
}

func (cs *CrawlerService) processErrLinkTx(ctx context.Context, shot *ResourceShot) error {
	if err := cs.outboxRepo.AddBatch(ctx, []*ResourceShot{shot}); err != nil {
		return fmt.Errorf("send update for link %s: %w", shot.URL, err)
	}

	return nil
}
