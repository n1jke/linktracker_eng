package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/bot/application"
	consumer "github.com/n1jke/linktracker_eng/internal/bot/infrastructure/kafka"
	"github.com/n1jke/linktracker_eng/internal/bot/infrastructure/scheduler"
)

var Module = fx.Module(
	"bot-repositories",
	fx.Provide(
		ProvidePool,
		fx.Annotate(
			NewInMemoryChatStateRepository,
			fx.As(new(application.ChatStateRepository)),
		),
		fx.Annotate(
			NewInboxRepo,
			fx.As(new(consumer.InboxRepository)),
			fx.As(new(scheduler.InboxRepository)),
		),
	),
	fx.Invoke(RegisterLifecycle),
)

func ProvidePool(cfg *config.AppConfig) (*pgxpool.Pool, error) {
	connConfig, err := pgxpool.ParseConfig(cfg.DB.ConnectionString())
	if err != nil {
		return nil, err
	}

	connConfig.MaxConns = 5
	connConfig.MinConns = 1
	connConfig.MaxConnIdleTime = 500 * time.Millisecond
	connConfig.MaxConnLifetime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func RegisterLifecycle(lc fx.Lifecycle, pool *pgxpool.Pool) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return nil
		},
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})
}
