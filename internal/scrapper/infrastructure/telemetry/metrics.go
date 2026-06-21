package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusRecorder struct {
	linksOnTrackTotal    prometheus.Gauge
	requestDurationTotal *prometheus.HistogramVec
	apiRequestsTotal     *prometheus.CounterVec
	registry             *prometheus.Registry
}

func NewPrometheusRecorder(reg *prometheus.Registry) *PrometheusRecorder {
	r := &PrometheusRecorder{
		linksOnTrackTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "links_on_track_total",
			Help: "total number of tracked links",
		}),

		requestDurationTotal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "request_duration_ms_total",
			Help:    "duration of operations in ms",
			Buckets: []float64{100, 250, 375, 500, 750, 1000, 1250, 1500, 2000},
		}, []string{"scope", "scope_type"}),

		apiRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "total number of requests",
		}, []string{"source"}),

		registry: reg,
	}

	reg.MustRegister(
		r.linksOnTrackTotal,
		r.requestDurationTotal,
		r.apiRequestsTotal,
	)

	return r
}

func (p *PrometheusRecorder) Registry() *prometheus.Registry {
	return p.registry
}

func (p *PrometheusRecorder) SetLinksOnTrack(count int) {
	p.linksOnTrackTotal.Set(float64(count))
}

func (p *PrometheusRecorder) Observe(scope, scopeType string, start time.Time) {
	p.requestDurationTotal.WithLabelValues(scope, scopeType).Observe(float64(time.Since(start).Milliseconds()))
}

func (p *PrometheusRecorder) AddRate(source string) {
	p.apiRequestsTotal.WithLabelValues(source).Inc()
}
