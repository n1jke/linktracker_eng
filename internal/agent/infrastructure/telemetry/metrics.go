package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusRecorder struct {
	requestDurationTotal *prometheus.HistogramVec
	registry             *prometheus.Registry
}

func NewPrometheusRecorder(reg *prometheus.Registry) *PrometheusRecorder {
	r := &PrometheusRecorder{
		requestDurationTotal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "request_duration_ms_total",
			Help:    "duration of operations in ms",
			Buckets: []float64{100, 250, 375, 500, 750, 1000, 1250, 1500, 2000},
		}, []string{"scope", "scope_type"}),

		registry: reg,
	}

	reg.MustRegister(
		r.requestDurationTotal,
	)

	return r
}

func (p *PrometheusRecorder) Registry() *prometheus.Registry {
	return p.registry
}

func (p *PrometheusRecorder) Observe(scope, scopeType string, start time.Time) {
	p.requestDurationTotal.WithLabelValues(scope, scopeType).Observe(float64(time.Since(start).Milliseconds()))
}
