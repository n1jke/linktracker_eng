package botclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"

	bothttp "github.com/n1jke/linktracker/internal/infrastructure/transport/http/bot"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/pkg/retry"
)

var ErrUnexpectedCode = errors.New("unexpected returned code")

type BotClient struct {
	client    *bothttp.ClientWithResponses
	logger    *slog.Logger
	timeout   time.Duration
	retrier   *retry.Retrier
	retryable func(int, error) bool
	cb        *gobreaker.CircuitBreaker[struct{}]
}

func NewBotClient(botURL string, logger *slog.Logger, timeout time.Duration, retryCfg *retry.Config,
	retrCodes []int, cbCfg *gobreaker.Settings,
) (*BotClient, error) {
	logger = logger.With("module", "bot-client")

	client, err := bothttp.NewClientWithResponses(botURL)
	if err != nil {
		logger.Error("create bot client", slog.String("err", err.Error()))
		return nil, err
	}

	return &BotClient{
		client:    client,
		logger:    logger,
		timeout:   timeout,
		retrier:   retry.NewRetrier(retryCfg),
		retryable: retry.NewHTTPRetryableFunc(retrCodes),
		cb:        gobreaker.NewCircuitBreaker[struct{}](*cbCfg),
	}, nil
}

func (b *BotClient) SendUpdate(ctx context.Context, update *application.ResourceShot) error {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	req := bothttp.LinkUpdate{
		Description: &update.Description,
		Id:          &update.ID,
		TgChatIds:   &update.ChatIDs,
		Url:         &update.URL,
	}

	var (
		resp  *bothttp.PostUpdatesResponse
		errIn error
	)

	_, err := b.cb.Execute(func() (struct{}, error) {
		err := b.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = b.client.PostUpdatesWithResponse(ctx, req)
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, b.retryable)

		return struct{}{}, err
	})
	if err != nil {
		b.logger.Error("send update", slog.Any("err", err))
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%w: status=%d", ErrUnexpectedCode, resp.StatusCode())
	}

	return nil
}
