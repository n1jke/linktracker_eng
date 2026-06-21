package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/n1jke/linktracker/config"
	"github.com/n1jke/linktracker/pkg/retry"
)

var ErrTokenRequire = errors.New("token is required")

type Sender struct {
	bot     *bot.Bot
	service CommandService
	retrier *retry.Retrier
	logger  *slog.Logger
}

func NewTelegramBot(cfg *config.BotConfig, service CommandService, retrier *retry.Retrier, logger *slog.Logger) (*Sender, error) {
	token := cfg.TelegramToken
	if token == "" {
		return nil, ErrTokenRequire
	}

	b, err := bot.New(token)
	if err != nil {
		return nil, err
	}

	return &Sender{
		bot:     b,
		service: service,
		retrier: retrier,
		logger:  logger,
	}, nil
}

func (t *Sender) Setup(ctx context.Context) error {
	hf := NewHandlerFactory(t.logger, t.service, t.retrier)
	handler := hf.Handle()

	commands := make([]models.BotCommand, 0, 5)

	for _, v := range t.service.SupportedCommands() {
		t.bot.RegisterHandler(bot.HandlerTypeMessageText, v.Name, bot.MatchTypeCommand, handler)

		commands = append(commands, models.BotCommand{
			Command:     v.Name,
			Description: v.Description,
		})
	}

	// default/fallback handler
	t.bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, handler)

	go func() {
		err := t.retrier.Do(ctx, func() error {
			_, err := t.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
				Commands: commands,
				Scope:    &models.BotCommandScopeDefault{},
			})

			return err
		}, isRetryableTelegramErr)
		if err != nil {
			t.logger.Warn("fail to set bot commands, continue without command menu", slog.Any("err", err))
		}
	}()

	return nil
}

func (t *Sender) Start(ctx context.Context) {
	t.bot.Start(ctx)
}

func (t *Sender) Bot() *bot.Bot {
	return t.bot
}

func isRetryableTelegramErr(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "500") || strings.Contains(msg, "503")
}
