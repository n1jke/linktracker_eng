package consumer

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/n1jke/linktracker/config"
)

var Module = fx.Module(
	"kafka-consumer",
	fx.Provide(
		fx.Private,
		NewConfig,
	),
	fx.Provide(
		NewKafkaConsumer,
	),
	fx.Invoke(RegisterLifecycle),
)

func RegisterLifecycle(lc fx.Lifecycle, consumer *KafkaConsumer, logger *slog.Logger, cfg *config.AppConfig) {
	var cancel context.CancelFunc

	done := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context

			ctx, cancel = context.WithCancel(context.Background())

			go func() {
				defer close(done)

				if err := consumer.Consume(ctx); err != nil {
					logger.Error("consumer stopped", slog.Any("err", err))
					cancel()
				}
			}()

			logger.Info("consumer started")

			return nil
		},

		OnStop: func(ctx context.Context) error {
			logger.Info("consumer stopping")

			ctxShutdown, cancelShutdown := context.WithTimeout(ctx, cfg.Timeouts.Shutdown)
			defer cancelShutdown()

			if err := consumer.Stop(); err != nil {
				logger.Error("stop consumer", slog.Any("err", err))
			}

			cancel()

			select {
			case <-done:
				logger.Info("consumer stopped")
			case <-ctxShutdown.Done():
				logger.Warn("consumer stop deadline exceeded", slog.Any("err", ctxShutdown.Err()))
			}

			return nil
		},
	})
}

func NewConfig(cfg *config.AppConfig) *KafkaConfig {
	return &KafkaConfig{
		Topic:             cfg.Kafka.RawUpdatesTopic,
		DLQTopic:          cfg.Kafka.RawDLQTopic,
		ConsumerGroup:     cfg.Kafka.RawConsumerGroup,
		Brokers:           cfg.Kafka.BootstrapServers,
		username:          cfg.Kafka.Username,
		password:          cfg.Kafka.Password,
		SchemaRegistryURL: cfg.Kafka.SchemaRegistryURL,
		RetryAttempts:     cfg.Kafka.ConsumerAttempts,
	}
}
