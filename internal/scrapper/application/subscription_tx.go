package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

func (s *ScrapperService) subscribeTx(ctx context.Context, link string, clientID int64, tags ...string) error {
	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return err
	}

	linkObj, err := s.linksRepo.GetLinkByName(ctx, link)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.logger.Error("get link", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
			return fmt.Errorf("get link: %w", err)
		}

		linkObj, err = domain.NewTrackedLink(link)
		if err != nil {
			s.logger.Error("invalid link", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
			return err
		}

		if err := s.linksRepo.AddLink(ctx, linkObj); err != nil {
			s.logger.Error("add link", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))
			return fmt.Errorf("add link: %w", err)
		}
	}

	sub := domain.NewLinkSubscription(clientID, linkObj.ID(), link, tags...)
	if err := s.subsRepo.AddSubscription(ctx, sub); err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			s.logger.Info("link already tracked", slog.String("link", link), slog.Int64("client_id", clientID))
			return ErrLinkAlreadyTracked
		}

		s.logger.Error("add subscription", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))

		return fmt.Errorf("add subscription: %w", err)
	}

	return nil
}

func (s *ScrapperService) getLinksTx(ctx context.Context, clientID int64) ([]*domain.LinkSubscription, error) {
	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return nil, err
	}

	links, err := s.subsRepo.GetSubscriptionsByUserID(ctx, clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.logger.Error("subscribes not found", slog.Int64("client_id", clientID), slog.Any("err", err))
			return nil, nil
		}

		s.logger.Error("get subscriptions", slog.Int64("client_id", clientID), slog.Any("err", err))

		return nil, fmt.Errorf("get subscriptions: %w", err)
	}

	return links, err
}

func (s *ScrapperService) unsubscribeTx(ctx context.Context, link string, clientID int64) error {
	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return err
	}

	linkObj, err := s.linksRepo.GetLinkByName(ctx, link)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.logger.Info("link not found", slog.String("link", link), slog.Int64("client_id", clientID))
			return ErrLinkNotFound
		}

		s.logger.Error("get link", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))

		return fmt.Errorf("get link: %w", err)
	}

	sub := domain.NewLinkSubscription(clientID, linkObj.ID(), link)
	if err := s.subsRepo.RemoveSubscription(ctx, sub); err != nil {
		if errors.Is(err, ErrNotFound) {
			s.logger.Info("subscription not found", slog.String("link", link), slog.Int64("client_id", clientID))
			return ErrLinkNotFound
		}

		s.logger.Error("remove subscription", slog.String("link", link), slog.Int64("client_id", clientID), slog.Any("err", err))

		return fmt.Errorf("remove subscription: %w", err)
	}

	return nil
}

func (s *ScrapperService) removeClientTx(ctx context.Context, clientID int64) error {
	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return err
	}

	err := s.subsRepo.RemoveSubscriptionsByUserID(ctx, clientID)
	if err != nil {
		s.logger.Error("remove subscriptions", slog.Int64("client_id", clientID), slog.Any("err", err))
		return fmt.Errorf("remove subscriptions: %w", err)
	}

	if err = s.userRepo.RemoveClient(ctx, clientID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrChatNotFound
		}

		s.logger.Error("remove client", slog.Int64("client_id", clientID), slog.Any("err", err))

		return fmt.Errorf("remove client: %w", err)
	}

	return nil
}

func (s *ScrapperService) addTagsTx(ctx context.Context, link string, clientID int64, tags []string) error {
	if len(tags) == 0 {
		return ErrBadRequest
	}

	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return err
	}

	linkObj, err := s.linksRepo.GetLinkByName(ctx, link)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrLinkNotFound
		}

		return fmt.Errorf("get link: %w", err)
	}

	if err = s.subsRepo.AddTags(ctx, clientID, linkObj.ID(), tags); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrLinkNotFound
		}

		return fmt.Errorf("add tags: %w", err)
	}

	return nil
}

func (s *ScrapperService) clearTagsTx(ctx context.Context, link string, clientID int64) error {
	if err := s.ensureClientExists(ctx, clientID); err != nil {
		return err
	}

	linkObj, err := s.linksRepo.GetLinkByName(ctx, link)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrLinkNotFound
		}

		return fmt.Errorf("get link: %w", err)
	}

	if err = s.subsRepo.ClearTags(ctx, clientID, linkObj.ID()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrLinkNotFound
		}

		return fmt.Errorf("clear tags: %w", err)
	}

	return nil
}
