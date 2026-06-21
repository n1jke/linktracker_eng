package repository

import (
	"context"
	"sync"

	"github.com/segmentio/kafka-go"
)

type IdempotencyTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func NewIdempotencyTracker() *IdempotencyTracker {
	return &IdempotencyTracker{
		seen: make(map[string]struct{}),
	}
}

func (t *IdempotencyTracker) CheckAndStore(_ context.Context, msg *kafka.Message) (bool, error) {
	key := checkHeader(msg.Headers, "idempotency-key")
	if key == "" {
		return false, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.seen[key]; ok {
		return true, nil
	}

	t.seen[key] = struct{}{}

	return false, nil
}

func checkHeader(headers []kafka.Header, key string) string {
	for i := range headers {
		if headers[i].Key == key {
			return string(headers[i].Value)
		}
	}

	return ""
}
