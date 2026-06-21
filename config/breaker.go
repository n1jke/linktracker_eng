package config

import (
	"fmt"
	"time"
)

type BreakerConfig struct {
	MaxRequests          uint32        `env:"CB_MAX_REQUESTS" envDefault:"5"`
	Interval             time.Duration `env:"CB_INTERVAL" envDefault:"10s"`
	BucketPeriod         time.Duration `env:"CB_BUCKET_PERIOD" envDefault:"1s"`
	Timeout              time.Duration `env:"CB_TIMEOUT" envDefault:"1s"`
	FailureRateThreshold float64       `env:"CB_FAILURE_RATE_THRESHOLD" envDefault:"10"`
}

func (c *BreakerConfig) Validate() error {
	if c.Interval < 0 {
		return fmt.Errorf("interval: %v", c.Interval)
	}

	if c.BucketPeriod < 0 {
		return fmt.Errorf("bucket period: %v", c.BucketPeriod)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout: %v", c.Timeout)
	}

	if c.BucketPeriod > 0 && c.Interval < c.BucketPeriod {
		return fmt.Errorf("interval %v < bucket period %v", c.Interval, c.BucketPeriod)
	}

	if c.FailureRateThreshold < 0 || c.FailureRateThreshold > 100 {
		return fmt.Errorf("failure rate threshold: %.2f (must be 0-100)", c.FailureRateThreshold)
	}

	return nil
}
