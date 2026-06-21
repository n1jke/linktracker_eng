package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/n1jke/linktracker/internal/infrastructure/transport"
)

type Func func(http.Handler) http.Handler

const scope = "http"

func LogMiddleware(logger *slog.Logger) Func {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()

			next.ServeHTTP(w, r)

			logger.Info(
				"http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("duration", time.Since(started)),
			)
		})
	}
}

func LimitMiddleware(rps, burst int) Func {
	limitter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limitter.Allow() {
				next.ServeHTTP(w, r)
				return
			}

			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
}

func RateMiddleware(m transport.RateMetrics) Func {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer m.AddRate(r.Pattern)

			next.ServeHTTP(w, r)
		})
	}
}

type ClientDurationTransport struct {
	Next    http.RoundTripper
	Metrics transport.DurationMetrics
}

func (l *ClientDurationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	defer l.Metrics.ObserveDuration(scope, req.URL.Path, time.Now())
	return l.Next.RoundTrip(req)
}
