package telemetry

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/fx"

	"github.com/n1jke/linktracker/config"
	"github.com/n1jke/linktracker/internal/agent/infrastructure/ai"
	"github.com/n1jke/linktracker/internal/agent/infrastructure/scheduler"
	"github.com/n1jke/linktracker/internal/infrastructure/transport"
)

var Module = fx.Module("agent-metrics",
	fx.Provide(
		NewPrometheusRegistry,
		ProvideAgentMetricsPusher,
	),
	fx.Provide(
		fx.Annotate(
			NewPrometheusRecorder,
			fx.As(new(ai.MetricsRecorder)),
		),
	),
)

func NewPrometheusRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	return reg
}

func ProvideAgentMetricsPusher(cfg *config.AppConfig, reg *prometheus.Registry, logger *slog.Logger) scheduler.Pusher {
	return transport.NewPushPublisher(cfg.Scrapper.PushGatewayURL, "agent", reg, logger)
}
