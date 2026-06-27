package telemetry

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/bot/application"
	"github.com/n1jke/linktracker_eng/internal/bot/infrastructure/scheduler"
	"github.com/n1jke/linktracker_eng/internal/infrastructure/transport"
)

var Module = fx.Module(
	"bot-metrics",
	fx.Provide(
		NewPrometheusRegistry,
		ProvideBotMetricsPusher,
	),
	fx.Provide(
		fx.Annotate(
			NewPrometheusRecorder,
			fx.As(new(transport.DurationMetrics)),
			fx.As(new(scheduler.MetricsRecorder)),
			fx.As(new(application.MetricsRecorder)),
		),
	),
)

func NewPrometheusRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	return reg
}

func ProvideBotMetricsPusher(cfg *config.AppConfig, reg *prometheus.Registry, logger *slog.Logger) scheduler.Pusher {
	return transport.NewPushPublisher(cfg.Bot.PushGatewayURL, "bot", reg, logger)
}
