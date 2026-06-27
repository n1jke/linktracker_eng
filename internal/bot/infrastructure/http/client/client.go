package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"

	"github.com/n1jke/linktracker_eng/internal/bot/application"
	"github.com/n1jke/linktracker_eng/internal/infrastructure/transport"
	"github.com/n1jke/linktracker_eng/internal/infrastructure/transport/http/middleware"
	scrapperhttp "github.com/n1jke/linktracker_eng/internal/infrastructure/transport/http/scrapper"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
	"github.com/n1jke/linktracker_eng/pkg/retry"
)

type ScrapperClient struct {
	client    *scrapperhttp.ClientWithResponses
	logger    *slog.Logger
	timeout   time.Duration
	retrier   *retry.Retrier
	retryable func(int, error) bool
	cb        *gobreaker.CircuitBreaker[struct{}]
}

func NewScrapperClient(scrapperURL string, logger *slog.Logger, timeout time.Duration, retryCfg *retry.Config,
	retrCodes []int, cbCfg *gobreaker.Settings, m transport.DurationMetrics,
) (*ScrapperClient, error) {
	logger = logger.With("module", "scrapper-client")

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &middleware.ClientDurationTransport{
			Next:    http.DefaultTransport,
			Metrics: m,
		},
	}

	client, err := scrapperhttp.NewClientWithResponses(scrapperURL, scrapperhttp.WithHTTPClient(httpClient))
	if err != nil {
		logger.Error("create scrapper client", slog.String("err", err.Error()))
		return nil, err
	}

	return &ScrapperClient{
		client:    client,
		logger:    logger,
		timeout:   timeout,
		retrier:   retry.NewRetrier(retryCfg),
		retryable: retry.NewHTTPRetryableFunc(retrCodes),
		cb:        gobreaker.NewCircuitBreaker[struct{}](*cbCfg),
	}, nil
}

func (s *ScrapperClient) RegisterChat(ctx context.Context, chatID int64) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.PostTgChatIdResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.PostTgChatIdWithResponse(ctx, chatID)
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("unexpected resp", slog.Int64("chat_id", chatID), slog.Any("err", err))
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "register")

	return err
}

func (s *ScrapperClient) TrackLink(ctx context.Context, chatID int64, link string, tags []string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.PostLinksResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.PostLinksWithResponse(ctx, &scrapperhttp.PostLinksParams{TgChatId: chatID}, scrapperhttp.PostLinksJSONRequestBody{
				Link: &link,
				Tags: &tags,
			})
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("request", slog.Int64("chat_id", chatID), slog.String("url", link), slog.Any("err", err))
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "track")

	return err
}

func (s *ScrapperClient) UntrackLink(ctx context.Context, chatID int64, link string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.DeleteLinksResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.DeleteLinksWithResponse(ctx, &scrapperhttp.DeleteLinksParams{TgChatId: chatID},
				scrapperhttp.DeleteLinksJSONRequestBody{
					Link: &link,
				})
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("request", slog.Int64("chat_id", chatID), slog.String("url", link), slog.Any("err", err))
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "untrack")

	return err
}

func (s *ScrapperClient) AddTags(ctx context.Context, chatID int64, link string, tags []string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.PostTagsResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.PostTagsWithResponse(ctx, &scrapperhttp.PostTagsParams{TgChatId: chatID}, scrapperhttp.PostTagsJSONRequestBody{
				Link: &link, Tags: &tags,
			})
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("request", slog.Int64("chat_id", chatID), slog.String("url", link), slog.Any("err", err))
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "add_tags")

	return err
}

func (s *ScrapperClient) ClearTags(ctx context.Context, chatID int64, link string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.DeleteTagsResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.DeleteTagsWithResponse(ctx, &scrapperhttp.DeleteTagsParams{TgChatId: chatID},
				scrapperhttp.DeleteTagsJSONRequestBody{
					Link: &link,
				})
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("request", slog.Int64("chat_id", chatID), slog.String("url", link), slog.Any("err", err))
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "clear_tags")

	return err
}

func (s *ScrapperClient) ListLinks(ctx context.Context, chatID int64) ([]*domain.LinkSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	var (
		resp  *scrapperhttp.GetLinksResponse
		errIn error
	)

	_, err := s.cb.Execute(func() (struct{}, error) {
		err := s.retrier.DoWithCode(ctx, func() (int, error) {
			resp, errIn = s.client.GetLinksWithResponse(ctx, &scrapperhttp.GetLinksParams{TgChatId: chatID})
			if errIn != nil {
				return 0, errIn
			}

			return resp.StatusCode(), nil
		}, s.retryable)

		return struct{}{}, err
	})
	if err != nil {
		s.logger.Error("request", slog.Int64("chat_id", chatID), slog.Any("err", err))
		return nil, fmt.Errorf("%w: %v", application.ErrUnavailable, err)
	}

	err = mapStatusCodes(resp.StatusCode(), "list")
	if err != nil {
		return nil, err
	}

	result := make([]*domain.LinkSubscription, 0, len(*resp.JSON200.Links))
	for _, link := range *resp.JSON200.Links {
		var tags []string
		if link.Tags != nil {
			tags = append(tags, *link.Tags...)
		}

		result = append(result, domain.NewLinkSubscription(chatID, *link.Id, *link.Url, tags...))
	}

	return result, nil
}

func mapStatusCodes(status int, op string) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusConflict:
		if op == "register" {
			return application.ErrAlreadyRegistered
		}

		if op == "track" {
			return application.ErrAlreadyTracked
		}

		return application.ErrUnavailable
	case http.StatusNotFound:
		if op == "register" || op == "list" {
			return application.ErrChatNotFound
		}

		if op == "track" || op == "untrack" || op == "add_tags" || op == "clear_tags" {
			return application.ErrLinkNotFound
		}

		return application.ErrUnavailable
	case http.StatusBadRequest:
		return application.ErrBadRequest
	default:
		return application.ErrUnavailable
	}
}
