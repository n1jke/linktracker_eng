package producer

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/n1jke/linktracker/config"
	"github.com/n1jke/linktracker/internal/agent/infrastructure/scheduler"
)

var Module = fx.Module(
	"kafka-producer",
	fx.Provide(
		fx.Private,
		NewConfig,
	),
	fx.Provide(
		NewKafkaProducer,
		func(p *KafkaProducer) scheduler.Publisher { return p },
	),
	fx.Invoke(RegisterLifecycle),
)

func RegisterLifecycle(lc fx.Lifecycle, p *KafkaProducer, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return nil
		},
		OnStop: func(_ context.Context) error {
			logger.Info("producer stopping")

			if err := p.Close(); err != nil {
				logger.Error("stop producer", slog.Any("err", err))
				return fmt.Errorf("stop producer: %w", err)
			}

			return nil
		},
	})
}

func NewConfig(cfg *config.AppConfig) *KafkaConfig {
	return &KafkaConfig{
		Attempts:          cfg.Kafka.ProducerAttempts,
		BatchSize:         cfg.Kafka.ProducerBatchSize,
		Topic:             cfg.Kafka.UpdatesTopic,
		Brokers:           cfg.Kafka.BootstrapServers,
		username:          cfg.Kafka.Username,
		password:          cfg.Kafka.Password,
		SchemaRegistryURL: cfg.Kafka.SchemaRegistryURL,
	}
}
