package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
)

var Module = fx.Module(
	"scheduler",
	fx.Provide(
		fx.Private,
		NewConfig,
	),
	fx.Provide(
		NewSentinel,
	),
	fx.Invoke(RegisterLifecycle),
)

func RegisterLifecycle(lc fx.Lifecycle, s *Sentinel, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ctx := context.Background()

			err := s.Start(ctx)
			if err != nil {
				logger.Error("start scheduler", slog.Any("err", err))
				return fmt.Errorf("start scheduler: %w", err)
			}

			logger.Info("scheduler started")

			return nil
		},

		OnStop: func(_ context.Context) error {
			logger.Info("scheduler stopping")

			if err := s.Stop(); err != nil {
				logger.Error("stop scheduler", slog.Any("err", err))
				return fmt.Errorf("stop scheduler: %w", err)
			}

			return nil
		},
	})
}

func NewConfig(cfg *config.AppConfig) *Config {
	return &Config{
		InboxBatchSize: cfg.Bot.Scheduler.InboxBatchSize,
		InboxRelay:     cfg.Bot.Scheduler.InboxRelay,
		InboxClean:     cfg.Bot.Scheduler.InboxClean,
		MetricsPush:    cfg.Bot.Scheduler.MetricsPush,
	}
}
