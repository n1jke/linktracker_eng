package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type ScrapperService struct {
	logger     *slog.Logger
	tx         Transactor
	subsRepo   SubscriptionsRepository
	linksRepo  LinksRepository
	userRepo   UserRepository
	linksCache LinksCache
}

func NewScrapperService(logger *slog.Logger, subsRepo SubscriptionsRepository,
	linksRepo LinksRepository, userRepo UserRepository, tx Transactor,
) *ScrapperService {
	logger = logger.With("module", "scrapper")

	return &ScrapperService{
		logger:    logger,
		tx:        tx,
		subsRepo:  subsRepo,
		linksRepo: linksRepo,
		userRepo:  userRepo,
	}
}

func (s *ScrapperService) Subscribe(ctx context.Context, link string, clientID int64, tags ...string) error {
	s.logger.Info("subscribe start", slog.String("link", link), slog.Int64("client_id", clientID))

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.subscribeTx(ctx, link, clientID, tags...)
	})
	if err != nil {
		s.logger.Error("subscribe transaction", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
		return err
	}

	s.logger.Info("subscribe end", slog.String("link", link), slog.Int64("client_id", clientID))

	return nil
}

func (s *ScrapperService) GetLinks(ctx context.Context, clientID int64) ([]*domain.LinkSubscription, error) {
	s.logger.Info("get links start", slog.Int64("client_id", clientID))

	cacheKey := fmtKey(clientID)

	if s.linksCache != nil {
		links, ok, err := s.linksCache.Get(ctx, cacheKey)
		if err != nil {
			s.logger.Warn("cache get links", slog.Int64("client_id", clientID), slog.Any("err", err))
		} else if ok {
			s.logger.Info("cache hit", slog.Int64("client_id", clientID), slog.Int("count", len(links)))
			return links, nil
		}
	}

	links := make([]*domain.LinkSubscription, 0)

	var returnErr error

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		links, returnErr = s.getLinksTx(ctx, clientID)
		return returnErr
	})
	if err != nil {
		s.logger.Error("getLinks transaction", slog.Int64("client_id", clientID), slog.Any("err", err))
		return nil, err
	}

	s.logger.Info("get links end", slog.Int64("client_id", clientID), slog.Int("count", len(links)))

	return links, nil
}

func (s *ScrapperService) UnSubscribe(ctx context.Context, clientID int64, link string) error {
	s.logger.Info("unsubscribe start", slog.Int64("client_id", clientID), slog.String("link", link))

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.unsubscribeTx(ctx, link, clientID)
	})
	if err != nil {
		s.logger.Error("unsubscribe transaction", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
		return err
	}

	s.logger.Info("unsubscribe end", slog.Int64("client_id", clientID), slog.String("link", link))

	return nil
}

func (s *ScrapperService) AddClient(ctx context.Context, clientID int64) error {
	s.logger.Info("add client start", slog.Int64("client_id", clientID))

	client := domain.NewClient(clientID)

	err := s.userRepo.AddClient(ctx, client)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return ErrAlreadyExists
		}

		s.logger.Error("add client", slog.Int64("client_id", clientID), slog.Any("err", err))

		return fmt.Errorf("add client error: %w", err)
	}

	s.logger.Info("add client end", slog.Int64("client_id", clientID))

	return nil
}

func (s *ScrapperService) RemoveClient(ctx context.Context, clientID int64) error {
	s.logger.Info("remove client start", slog.Int64("client_id", clientID))

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.removeClientTx(ctx, clientID)
	})
	if err != nil {
		s.logger.Error("removeClient transaction", slog.Int64("client_id", clientID), slog.Any("err", err))
		return err
	}

	s.logger.Info("remove client end", slog.Int64("client_id", clientID))

	return nil
}

func (s *ScrapperService) AddTags(ctx context.Context, clientID int64, link string, tags []string) error {
	s.logger.Info("add tags start", slog.Int64("client_id", clientID), slog.String("link", link))

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.addTagsTx(ctx, link, clientID, tags)
	})
	if err != nil {
		s.logger.Error("add tags transaction", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
		return err
	}

	s.logger.Info("add tags end", slog.Int64("client_id", clientID), slog.String("link", link))

	return nil
}

func (s *ScrapperService) ClearTags(ctx context.Context, clientID int64, link string) error {
	s.logger.Info("clear tags start", slog.Int64("client_id", clientID), slog.String("link", link))

	err := s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		return s.clearTagsTx(ctx, link, clientID)
	})
	if err != nil {
		s.logger.Error("clear tags transaction", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
		return err
	}

	s.logger.Info("clear tags end", slog.Int64("client_id", clientID), slog.String("link", link))

	return nil
}

func (s *ScrapperService) ensureClientExists(ctx context.Context, clientID int64) error {
	_, err := s.userRepo.GetClientByID(ctx, clientID)
	if err == nil {
		return nil
	}

	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}

	s.logger.ErrorContext(ctx, "get client", "client_id", clientID, "error", err)

	return fmt.Errorf("get client: %w", err)
}
