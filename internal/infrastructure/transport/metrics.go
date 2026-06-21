package transport

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type DurationMetrics interface {
	ObserveDuration(scope, scopeType string, start time.Time)
}

type RateMetrics interface {
	AddRate(source string)
}

type GatewayPusher struct {
	logger     *slog.Logger
	pusher     *push.Pusher
	GatewayURL string
	Job        string
}

func NewPushPublisher(gateway, job string, reg *prometheus.Registry, logger *slog.Logger) *GatewayPusher {
	pusher := push.New(gateway, job).Gatherer(reg)

	return &GatewayPusher{
		logger: logger.With(slog.String("module", "push-publisher")),
		pusher: pusher,
	}
}

func (p *GatewayPusher) Push(ctx context.Context) error {
	if err := p.pusher.PushContext(ctx); err != nil {
		p.logger.Error("push metrics to gateway", slog.Any("err", err))
		return fmt.Errorf("push metrics: %w", err)
	}

	p.logger.Info("metrics pushed")

	return nil
}
