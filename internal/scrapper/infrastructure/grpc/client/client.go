package client

import (
	"context"
	"log/slog"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	transportgrpc "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/pkg/retry"
)

type BotGRPCClient struct {
	client    transportgrpc.BotServiceClient
	logger    *slog.Logger
	timeout   time.Duration
	retrier   *retry.Retrier
	retryable func(err error) bool
	cb        *gobreaker.CircuitBreaker[struct{}]
}

func NewBotGRPCClient(conn *grpc.ClientConn, logger *slog.Logger, timeout time.Duration, retryCfg *retry.Config,
	codes []uint32, cbCfg *gobreaker.Settings,
) *BotGRPCClient {
	logger = logger.With("module", "bot-grpc-client")

	return &BotGRPCClient{
		client:    transportgrpc.NewBotServiceClient(conn),
		logger:    logger,
		timeout:   timeout,
		retrier:   retry.NewRetrier(retryCfg),
		retryable: retry.NewGRPCRetryableFunc(codes),
		cb:        gobreaker.NewCircuitBreaker[struct{}](*cbCfg),
	}
}

func (b *BotGRPCClient) SendUpdate(ctx context.Context, update *application.ResourceShot) error {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	req := &transportgrpc.LinkUpdate{
		Id:          update.ID,
		Url:         update.URL,
		Description: update.Description,
		ChatIds:     update.ChatIDs,
		UpdatedAt:   timestamppb.New(update.UpdatedAt),
	}

	_, err := b.cb.Execute(func() (struct{}, error) {
		err := b.retrier.Do(ctx, func() error {
			_, err := b.client.PostUpdate(ctx, req)
			return err
		}, b.retryable)

		return struct{}{}, err
	})
	if err != nil {
		b.logger.Error("send update", slog.Any("err", err))
		return err
	}

	return nil
}
