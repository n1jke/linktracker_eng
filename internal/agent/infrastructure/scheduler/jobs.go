package scheduler

import (
	"context"
	"fmt"
	"log/slog"
)

func (s *Sentinel) relayOutbox(ctx context.Context) error {
	return s.tx.WithTransaction(ctx, s.relayOutboxTx)
}

func (s *Sentinel) relayOutboxTx(ctx context.Context) error {
	records, err := s.outbox.FetchPending(ctx, s.outboxBatchSize)
	if err != nil {
		s.logger.Error("fetch pending outbox", slog.Any("err", err))
		return fmt.Errorf("fetch pending outbox: %w", err)
	}

	for i := range records {
		err := s.publisher.Publish(ctx, records[i])
		if err != nil {
			s.logger.Warn("publish processed update", slog.Any("err", err))
		}

		if dbErr := s.outbox.UpdateStatus(ctx, records[i].ID, err); dbErr != nil {
			s.logger.Error("update outbox status", slog.Any("err", dbErr))
		}
	}

	return nil
}

func (s *Sentinel) relayInbox(ctx context.Context) error {
	return s.tx.WithTransaction(ctx, s.relayInboxTx)
}

func (s *Sentinel) relayInboxTx(ctx context.Context) error {
	records, err := s.inbox.FetchPending(ctx, s.groupWindow)
	if err != nil {
		s.logger.Error("fetch pending inbox", slog.Any("err", err))
		return fmt.Errorf("fetch pending inbox: %w", err)
	}

	if err := s.service.HandleUpdatesWindow(ctx, records); err != nil {
		s.logger.Error("handle window update", slog.Any("err", err))
		return fmt.Errorf("handle window update: %w", err)
	}

	if err := s.inbox.MarkProcessed(ctx, records); err != nil {
		s.logger.Error("update inbox status", slog.Any("err", err))
		return fmt.Errorf("update inbox status: %w", err)
	}

	return nil
}

func (s *Sentinel) cleanOutbox(ctx context.Context) error {
	_, err := s.outbox.Cleanup(ctx, s.outboxClean)
	if err != nil {
		s.logger.Error("outbox cleanup", slog.Any("err", err))
		return fmt.Errorf("outbox cleanup: %w", err)
	}

	return nil
}

func (s *Sentinel) cleanInbox(ctx context.Context) error {
	_, err := s.inbox.Cleanup(ctx, s.inboxClean)
	if err != nil {
		s.logger.Error("inbox cleanup", slog.Any("err", err))
		return fmt.Errorf("inbox cleanup: %w", err)
	}

	return nil
}
