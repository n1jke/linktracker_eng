package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/n1jke/linktracker_eng/internal/bot/application"
	"github.com/n1jke/linktracker_eng/pkg/retry"
)

const ServiceUnavailable = "Сервис временно недоступен, попробуйте позже"

type HandlerFactory struct {
	logger  *slog.Logger
	service CommandService
	retrier *retry.Retrier
}

type CommandService interface {
	Handle(ctx context.Context, text string, userID int64) (string, error)
	SupportedCommands() []application.CommandInfo
}

func NewHandlerFactory(logger *slog.Logger, service CommandService, retrier *retry.Retrier) *HandlerFactory {
	return &HandlerFactory{
		logger:  logger.With("module", "telegram-handlers"),
		service: service,
		retrier: retrier,
	}
}

func (f HandlerFactory) Handle() bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if find := checkNilMsg(update); find {
			f.logger.Warn("received nil message")
			return
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		response, err := f.service.Handle(ctx, text, chatID)
		if err != nil {
			f.logger.Error("handle message", slog.String("msg", text), slog.Any("err", err))

			err = f.retrier.Do(ctx, func() error {
				_, err := b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   ServiceUnavailable,
				})

				return err
			}, isRetryableTelegramErr)
			if err != nil {
				f.logger.Error("send `internal error` message", slog.Int64("chat_id", chatID), slog.Any("err", err))
			}

			return
		}

		err = f.retrier.Do(ctx, func() error {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   response,
			})

			return err
		}, isRetryableTelegramErr)
		if err != nil {
			f.logger.Error("send message", slog.Int64("chat_id", chatID), slog.Any("err", err))
		}
	}
}

func checkNilMsg(update *models.Update) bool {
	if update == nil || update.Message == nil {
		return true
	}

	return false
}
