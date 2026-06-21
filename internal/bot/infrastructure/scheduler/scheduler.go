package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"

	consumer "github.com/n1jke/linktracker/internal/bot/infrastructure/kafka"
)

type Transactor interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type InboxRepository interface {
	FetchPending(ctx context.Context, batchSize int) ([]*consumer.Update, error)
	MarkProcessed(ctx context.Context, updates []*consumer.Update) error
	Cleanup(ctx context.Context, gap time.Duration) error
}

type BotNotifier interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

type MetricsRecorder interface {
	AddNotificationCount(count int)
}

type Pusher interface {
	Push(ctx context.Context) error
}

type Config struct {
	InboxBatchSize int
	InboxRelay     time.Duration
	InboxClean     time.Duration
	MetricsPush    time.Duration
}

type Sentinel struct {
	logger    *slog.Logger
	notifier  BotNotifier
	inbox     InboxRepository
	tx        Transactor
	scheduler gocron.Scheduler
	metrics   MetricsRecorder
	pusher    Pusher

	inboxBatchSize int
	inboxRelay     time.Duration
	inboxClean     time.Duration
	metricsPush    time.Duration
}

func NewSentinel(logger *slog.Logger, notifier BotNotifier, tx Transactor, inbox InboxRepository, cfg *Config,
	m MetricsRecorder, pusher Pusher,
) (*Sentinel, error) {
	scheduler, err := gocron.NewScheduler(
		gocron.WithGlobalJobOptions(
			gocron.WithEventListeners(
				gocron.BeforeJobRuns(func(_ uuid.UUID, jobName string) {
					logger.Info("job started", slog.String("job", jobName))
				}),

				gocron.AfterJobRunsWithError(func(_ uuid.UUID, jobName string, err error) {
					logger.Error("job failed", slog.String("job", jobName), slog.Any("err", err))
				}),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	return &Sentinel{
		logger:         logger.With("module", "scheduler"),
		notifier:       notifier,
		inbox:          inbox,
		tx:             tx,
		scheduler:      scheduler,
		metrics:        m,
		pusher:         pusher,
		inboxBatchSize: cfg.InboxBatchSize,
		inboxRelay:     cfg.InboxRelay,
		inboxClean:     cfg.InboxClean,
		metricsPush:    cfg.MetricsPush,
	}, nil
}

func (s *Sentinel) Start(ctx context.Context) error {
	_, err := s.scheduler.NewJob(gocron.DurationJob(s.inboxRelay),
		gocron.NewTask(s.relayInbox, ctx),
		gocron.WithName("inbox-relay"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(gocron.DurationJob(s.inboxClean),
		gocron.NewTask(s.cleanInbox, ctx),
		gocron.WithName("inbox-clean"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(gocron.DurationJob(s.metricsPush),
		gocron.NewTask(s.pusher.Push, ctx),
		gocron.WithName("metrics-push"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	s.scheduler.Start()

	return nil
}

func (s *Sentinel) Stop() error {
	return s.scheduler.Shutdown()
}

func (s *Sentinel) relayInbox(ctx context.Context) error {
	return s.tx.WithTransaction(ctx, s.relayInboxTx)
}

func (s *Sentinel) relayInboxTx(ctx context.Context) error {
	updates, err := s.inbox.FetchPending(ctx, s.inboxBatchSize)
	if err != nil {
		s.logger.Error("fetch pending inbox", slog.Any("err", err))
		return fmt.Errorf("fetch pending inbox: %w", err)
	}

	received := s.sendTelegram(ctx, updates)

	if err := s.inbox.MarkProcessed(ctx, received); err != nil {
		s.logger.Error("update inbox status", slog.Any("err", err))
		return fmt.Errorf("update inbox status: %w", err)
	}

	return nil
}

func (s *Sentinel) sendTelegram(ctx context.Context, updates []*consumer.Update) []*consumer.Update {
	success := make([]*consumer.Update, 0, len(updates))
	sendCount := 0

	for _, u := range updates {
		for _, chatID := range u.ChatIDs {
			_, sendErr := s.notifier.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   u.Description,
			})
			if sendErr != nil {
				s.logger.Warn("fail to send message", slog.Int64("chat_id", chatID), slog.Int64("update_id", u.ID), slog.Any("err", sendErr))
				break
			}

			sendCount++
		}

		success = append(success, u)
	}

	s.metrics.AddNotificationCount(sendCount)

	return success
}

func (s *Sentinel) cleanInbox(ctx context.Context) error {
	if err := s.inbox.Cleanup(ctx, s.inboxClean); err != nil {
		s.logger.Error("inbox cleaning", slog.Any("err", err))
		return fmt.Errorf("inbox cleaning: %w", err)
	}

	return nil
}
