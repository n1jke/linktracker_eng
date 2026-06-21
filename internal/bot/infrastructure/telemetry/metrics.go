package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusRecorder struct {
	commandRequestsTotal  *prometheus.CounterVec
	commandDurationTotal  *prometheus.HistogramVec
	sendNotificationCount prometheus.Counter
	registry              *prometheus.Registry
}

func NewPrometheusRecorder(reg *prometheus.Registry) *PrometheusRecorder {
	r := &PrometheusRecorder{
		commandRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "command_requests_total",
			Help: "total number of processed bot commands",
		}, []string{"command"}),

		commandDurationTotal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "command_duration_ms_total",
			Help:    "duration of scrapper calls in ms",
			Buckets: []float64{100, 250, 375, 500, 750, 1000, 1250, 1500, 2000},
		}, []string{"scope", "scope_type"}),

		sendNotificationCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sent_notification_total",
			Help: "total number of delivered notifications",
		}),

		registry: reg,
	}

	reg.MustRegister(
		r.commandRequestsTotal,
		r.commandDurationTotal,
		r.sendNotificationCount,
	)

	return r
}

func (p *PrometheusRecorder) Registry() *prometheus.Registry {
	return p.registry
}

func (p *PrometheusRecorder) AddCommandRate(cmd string) {
	p.commandRequestsTotal.WithLabelValues(cmd).Inc()
}

func (p *PrometheusRecorder) ObserveDuration(scope, scopeType string, start time.Time) {
	p.commandDurationTotal.WithLabelValues(scope, scopeType).Observe(float64(time.Since(start).Milliseconds()))
}

func (p *PrometheusRecorder) AddNotificationCount(count int) {
	p.sendNotificationCount.Add(float64(count))
}
