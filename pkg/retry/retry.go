package retry

import (
	"context"
	"math/rand/v2"
	"time"
)

const (
	MaxAttempts int           = 3
	BaseDelay   time.Duration = 200 * time.Millisecond
	MaxDelay    time.Duration = 1 * time.Second
)

type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type Retrier struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func NewRetrier(cfg *Config) *Retrier {
	return &Retrier{
		maxAttempts: cfg.MaxAttempts,
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
	}
}

func (r *Retrier) Do(ctx context.Context, fn func() error, retryable func(error) bool) error {
	var err error

	for i := 0; i < r.maxAttempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = fn()
		if err == nil {
			return nil
		}

		if !retryable(err) {
			return err
		}

		time.Sleep(backoffDelay(r.baseDelay, r.maxDelay, i))
	}

	return err
}

func (r *Retrier) DoWithCode(ctx context.Context, fn func() (int, error), retryable func(int, error) bool) error {
	var (
		err  error
		code int
	)

	for i := 0; i < r.maxAttempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		code, err = fn()
		if err == nil {
			return nil
		}

		if !retryable(code, err) {
			return err
		}

		time.Sleep(backoffDelay(r.baseDelay, r.maxDelay, i))
	}

	return err
}

func backoffDelay(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	delay := min(baseDelay<<attempt, maxDelay)

	half := delay / 2

	//nolint:gosec // math/rand/v2 enough because only for jitter in backoff
	jitter := time.Duration(rand.Int64N(int64(half)))

	return half + jitter
}
