package interceptors

import (
	"context"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/n1jke/linktracker/internal/infrastructure/transport"
)

const scope = "grpc"

func UnaryLimitInterceptor(rps, burst int) grpc.UnaryServerInterceptor {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if limiter.Allow() {
			return handler(ctx, req)
		}

		return nil, status.Error(codes.ResourceExhausted, "too many requests")
	}
}

func UnaryRateInterceptor(m transport.RateMetrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		defer m.AddRate(info.FullMethod)

		return handler(ctx, req)
	}
}

func ClientUnaryDurationInterceptor(m transport.DurationMetrics) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		defer m.ObserveDuration(scope, method, time.Now())
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
