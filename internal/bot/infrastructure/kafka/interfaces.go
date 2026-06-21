package consumer

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/segmentio/kafka-go"
)

//go:generate mockgen -source interfaces.go -destination=mocks/mock.go -package=mocks

type IdempotencyRepository interface {
	CheckAndStore(ctx context.Context, msg *kafka.Message) (bool, error)
}

type BotNotifier interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}
