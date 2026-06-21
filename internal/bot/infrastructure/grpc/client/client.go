package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/n1jke/linktracker/internal/bot/application"
	transportgrpc "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
	"github.com/n1jke/linktracker/pkg/retry"
)

type ScrapperGRPCClient struct {
	client    transportgrpc.ScrapperServiceClient
	logger    *slog.Logger
	timeout   time.Duration
	retrier   *retry.Retrier
	retryable func(err error) bool
	cb        *gobreaker.CircuitBreaker[struct{}]
}

func NewScrapperGRPCClient(conn *grpc.ClientConn, logger *slog.Logger, timeout time.Duration, retryCfg *retry.Config,
	retrCodes []uint32, cbCfg *gobreaker.Settings,
) *ScrapperGRPCClient {
	logger = logger.With("module", "scrapper-grpc-client")

	return &ScrapperGRPCClient{
		client:    transportgrpc.NewScrapperServiceClient(conn),
		logger:    logger,
		timeout:   timeout,
		retrier:   retry.NewRetrier(retryCfg),
		retryable: retry.NewGRPCRetryableFunc(retrCodes),
		cb:        gobreaker.NewCircuitBreaker[struct{}](*cbCfg),
	}
}

func (s *ScrapperGRPCClient) RegisterChat(ctx context.Context, chatID int64) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			_, err := s.client.RegisterChat(ctx, &transportgrpc.RegisterChatRequest{ChatId: chatID})
			return err
		}, s.retryable)

		return struct{}{}, err
	})

	return mapErrors(err, "register")
}

func (s *ScrapperGRPCClient) TrackLink(ctx context.Context, chatID int64, link string, tags []string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			_, err := s.client.TrackLink(ctx, &transportgrpc.TrackLinkRequest{ChatId: chatID, Url: link, Tags: tags})
			return err
		}, s.retryable)

		return struct{}{}, err
	})

	return mapErrors(err, "track")
}

func (s *ScrapperGRPCClient) UntrackLink(ctx context.Context, chatID int64, link string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			_, err := s.client.UntrackLink(ctx, &transportgrpc.UntrackLinkRequest{ChatId: chatID, Url: link})
			return err
		}, s.retryable)

		return struct{}{}, err
	})

	return mapErrors(err, "untrack")
}

func (s *ScrapperGRPCClient) AddTags(ctx context.Context, chatID int64, link string, tags []string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			_, err := s.client.AddTags(ctx, &transportgrpc.AddTagsRequest{
				ChatId: chatID,
				Url:    link,
				Tags:   tags,
			})

			return err
		}, s.retryable)

		return struct{}{}, err
	})

	return mapErrors(err, "add_tags")
}

func (s *ScrapperGRPCClient) ClearTags(ctx context.Context, chatID int64, link string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			_, err := s.client.ClearTags(ctx, &transportgrpc.ClearTagsRequest{
				ChatId: chatID,
				Url:    link,
			})

			return err
		}, s.retryable)

		return struct{}{}, err
	})

	return mapErrors(err, "clear_tags")
}

func (s *ScrapperGRPCClient) ListLinks(ctx context.Context, chatID int64) ([]*domain.LinkSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *transportgrpc.ListLinksResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.Do(ctx, func() error {
			resp, errIn = s.client.ListLinks(ctx, &transportgrpc.ListLinksRequest{ChatId: chatID})
			return errIn
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		return nil, mapErrors(err, "list")
	}

	result := make([]*domain.LinkSubscription, 0, len(resp.GetLinks()))
	for _, link := range resp.GetLinks() {
		id, parseErr := uuid.Parse(link.GetId())
		if parseErr != nil {
			return nil, fmt.Errorf("invalid uuid: %w", parseErr)
		}

		result = append(result, domain.NewLinkSubscription(chatID, id, link.GetUrl(), link.GetTags()...))
	}

	return result, nil
}

func mapErrors(err error, op string) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	switch st.Code() {
	case codes.AlreadyExists:
		if op == "register" {
			return application.ErrAlreadyRegistered
		}

		if op == "track" {
			return application.ErrAlreadyTracked
		}

		return application.ErrUnavailable
	case codes.NotFound:
		if op == "register" || op == "list" {
			return application.ErrChatNotFound
		}

		if op == "track" || op == "untrack" || op == "add_tags" || op == "clear_tags" {
			return application.ErrLinkNotFound
		}

		return application.ErrUnavailable
	case codes.InvalidArgument:
		return application.ErrBadRequest
	case codes.Unavailable:
		return application.ErrUnavailable
	case codes.OK, codes.Canceled, codes.Unknown, codes.DeadlineExceeded,
		codes.PermissionDenied, codes.ResourceExhausted, codes.FailedPrecondition,
		codes.Aborted, codes.OutOfRange, codes.Unimplemented, codes.Internal,
		codes.DataLoss, codes.Unauthenticated:
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	default:
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}
}
