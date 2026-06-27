package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/n1jke/linktracker_eng/internal/agent/application"
)

type OutboxRepository interface {
	FetchPending(ctx context.Context, limit int) ([]*application.ProcessedUpdate, error)
	UpdateStatus(ctx context.Context, id int64, errIn error) error
	Cleanup(ctx context.Context, gap time.Duration) (int64, error)
}

type InboxRepository interface {
	FetchPending(ctx context.Context, window time.Duration) ([]*application.RawUpdate, error)
	MarkProcessed(ctx context.Context, updates []*application.RawUpdate) error
	Cleanup(ctx context.Context, gap time.Duration) (int64, error)
}

type Transactor interface {
	WithTransaction(ctx context.Context, txFunc func(ctx context.Context) error) error
}

type Publisher interface {
	Publish(ctx context.Context, update *application.ProcessedUpdate) error
}

type Pusher interface {
	Push(ctx context.Context) error
}

type Config struct {
	GroupWindow     time.Duration
	OutboxBatchSize int
	OutboxRelay     time.Duration
	InboxRelay      time.Duration
	OutboxClean     time.Duration
	InboxClean      time.Duration
	MetricsPush     time.Duration
}

type Sentinel struct {
	logger    *slog.Logger
	service   *application.AgentService
	publisher Publisher
	tx        Transactor
	outbox    OutboxRepository
	inbox     InboxRepository
	pusher    Pusher
	scheduler gocron.Scheduler

	groupWindow     time.Duration
	outboxBatchSize int
	outboxRelay     time.Duration
	inboxRelay      time.Duration
	outboxClean     time.Duration
	inboxClean      time.Duration
	metricsPush     time.Duration
}

func NewSentinel(logger *slog.Logger, service *application.AgentService, publisher Publisher, tx Transactor,
	outbox OutboxRepository, inbox InboxRepository, p Pusher, cfg *Config,
) (*Sentinel, error) {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	return &Sentinel{
		logger:          logger.With("module", "scheduler"),
		service:         service,
		publisher:       publisher,
		tx:              tx,
		outbox:          outbox,
		inbox:           inbox,
		pusher:          p,
		scheduler:       scheduler,
		groupWindow:     cfg.GroupWindow,
		outboxBatchSize: cfg.OutboxBatchSize,
		outboxRelay:     cfg.OutboxRelay,
		inboxRelay:      cfg.InboxRelay,
		outboxClean:     cfg.OutboxClean,
		inboxClean:      cfg.InboxClean,
		metricsPush:     cfg.MetricsPush,
	}, nil
}

func (s *Sentinel) Start(ctx context.Context) error {
	_, err := s.scheduler.NewJob(
		gocron.DurationJob(s.outboxRelay),
		gocron.NewTask(s.relayOutbox, ctx),
		gocron.WithName("outbox-relay"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(s.inboxRelay),
		gocron.NewTask(s.relayInbox, ctx),
		gocron.WithName("inbox-relay"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(s.outboxClean),
		gocron.NewTask(s.cleanOutbox, ctx),
		gocron.WithName("outbox-clean"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(s.inboxClean),
		gocron.NewTask(s.cleanInbox, ctx),
		gocron.WithName("inbox-clean"),
		gocron.WithContext(ctx),
	)
	if err != nil {
		return err
	}

	_, err = s.scheduler.NewJob(
		gocron.DurationJob(s.metricsPush),
		gocron.NewTask(s.pusher.Push, ctx),
		gocron.WithName("metrics push"),
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
