package consumer

import (
	"context"

	"github.com/segmentio/kafka-go"

	"github.com/n1jke/linktracker/internal/agent/application"
)

//go:generate mockgen -source interfaces.go -destination=mocks/mocks.go -package=mocks

type IdempotencyRepository interface {
	CheckAndStore(ctx context.Context, msg *kafka.Message) (bool, error)
}

type Agent interface {
	HandleUpdate(ctx context.Context, raw *application.RawUpdate) error
}
