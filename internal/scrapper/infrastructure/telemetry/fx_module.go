package telemetry

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/infrastructure/transport"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/crawlers"
	qb "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository/query_builder"
	rawsql "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository/sql"
	cache "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository/valkey"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/scheduler"
)

var Module = fx.Module(
	"scrapper-metrics",
	fx.Provide(
		NewPrometheusRegistry,
		ProvideScrapperMetricsPusher,
	),
	fx.Provide(
		fx.Annotate(
			NewPrometheusRecorder,
			fx.As(new(transport.RateMetrics)),
			fx.As(new(application.MetricsRecorder)),
			fx.As(new(crawlers.MetricsRecorder)),
			fx.As(new(rawsql.MetricsRecorder)),
			fx.As(new(qb.MetricsRecorder)),
			fx.As(new(cache.MetricsRecorder)),
		),
	),
)

func NewPrometheusRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	return reg
}

func ProvideScrapperMetricsPusher(cfg *config.AppConfig, reg *prometheus.Registry, logger *slog.Logger) scheduler.Pusher {
	return transport.NewPushPublisher(cfg.Scrapper.PushGatewayURL, "scrapper", reg, logger)
}
