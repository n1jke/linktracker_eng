package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type LinkService interface {
	GetLinks(ctx context.Context, clientID int64) ([]*domain.LinkSubscription, error)
	Subscribe(ctx context.Context, link string, clientID int64, tags ...string) error
	UnSubscribe(ctx context.Context, clientID int64, link string) error
	AddClient(ctx context.Context, clientID int64) error
	RemoveClient(ctx context.Context, clientID int64) error
	AddTags(ctx context.Context, clientID int64, link string, tags []string) error
	ClearTags(ctx context.Context, clientID int64, link string) error
}

var _ LinkService = (*CachedScrapperService)(nil)

type CachedScrapperService struct {
	logger     *slog.Logger
	service    LinkService
	linksCache LinksCache
}

func NewCachedScrapperService(logger *slog.Logger, service LinkService, linksCache LinksCache,
) *CachedScrapperService {
	logger = logger.With("module", "scrapper")

	return &CachedScrapperService{
		logger:     logger,
		service:    service,
		linksCache: linksCache,
	}
}

func (s *CachedScrapperService) Subscribe(ctx context.Context, link string, clientID int64, tags ...string) error {
	if err := s.service.Subscribe(ctx, link, clientID, tags...); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	s.invalidateCache(ctx, clientID)

	return nil
}

func (s *CachedScrapperService) GetLinks(ctx context.Context, clientID int64) ([]*domain.LinkSubscription, error) {
	cacheKey := fmtKey(clientID)

	links, ok, err := s.linksCache.Get(ctx, cacheKey)
	if err != nil {
		s.logger.Warn("cache get links", slog.Int64("client_id", clientID), slog.Any("err", err))
	} else if ok {
		s.logger.Info("cache hit", slog.Int64("client_id", clientID), slog.Int("count", len(links)))
		return links, nil
	}

	links, err = s.service.GetLinks(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}

	if err := s.linksCache.Set(ctx, cacheKey, links); err != nil {
		s.logger.Warn("cache set", slog.Int64("client_id", clientID), slog.Any("err", err))
	}

	return links, nil
}

func (s *CachedScrapperService) UnSubscribe(ctx context.Context, clientID int64, link string) error {
	if err := s.service.UnSubscribe(ctx, clientID, link); err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}

	s.invalidateCache(ctx, clientID)

	return nil
}

func (s *CachedScrapperService) AddClient(ctx context.Context, clientID int64) error {
	if err := s.service.AddClient(ctx, clientID); err != nil {
		return fmt.Errorf("add client: %w", err)
	}

	return nil
}

func (s *CachedScrapperService) RemoveClient(ctx context.Context, clientID int64) error {
	if err := s.service.RemoveClient(ctx, clientID); err != nil {
		return fmt.Errorf("remove client: %w", err)
	}

	s.invalidateCache(ctx, clientID)

	return nil
}

func (s *CachedScrapperService) AddTags(ctx context.Context, clientID int64, link string, tags []string) error {
	if err := s.service.AddTags(ctx, clientID, link, tags); err != nil {
		return fmt.Errorf("add tags: %w", err)
	}

	s.invalidateCache(ctx, clientID)

	return nil
}

func (s *CachedScrapperService) ClearTags(ctx context.Context, clientID int64, link string) error {
	if err := s.service.ClearTags(ctx, clientID, link); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}

	s.invalidateCache(ctx, clientID)

	return nil
}

func (s *CachedScrapperService) invalidateCache(ctx context.Context, clientID int64) {
	key := fmtKey(clientID)
	if err := s.linksCache.Delete(ctx, key); err != nil {
		s.logger.Warn("invalidate cache", slog.Int64("client_id", clientID), slog.Any("err", err))
	}
}

func fmtKey(clientID int64) string {
	return fmt.Sprintf("links:%d", clientID)
}
