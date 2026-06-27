package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

//go:generate mockgen -source=interfaces.go -destination=mocks/mocks.go -package=mocks
type Transactor interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type UserRepository interface {
	AddClient(ctx context.Context, client *domain.Client) error
	RemoveClient(ctx context.Context, clientID int64) error
	GetClientByID(ctx context.Context, clientID int64) (*domain.Client, error)
}

type LinksRepository interface {
	AddLink(ctx context.Context, link *domain.TrackedLink) error
	RemoveLink(ctx context.Context, linkID uuid.UUID) error
	GetLinkByID(ctx context.Context, linkID uuid.UUID) (*domain.TrackedLink, error)
	GetLinkByName(ctx context.Context, name string) (*domain.TrackedLink, error)
	GetLinksBatch(ctx context.Context, limit int, pointer *time.Time) ([]*domain.TrackedLink, *time.Time, error)
}

type LinksCache interface {
	Get(ctx context.Context, key string) ([]*domain.LinkSubscription, bool, error)
	Set(ctx context.Context, key string, links []*domain.LinkSubscription) error
	Delete(ctx context.Context, key string) error
}

type SubscriptionsRepository interface {
	AddSubscription(ctx context.Context, subscription *domain.LinkSubscription) error
	AddTags(ctx context.Context, userID int64, resourceID uuid.UUID, tags []string) error
	ClearTags(ctx context.Context, userID int64, resourceID uuid.UUID) error
	RemoveSubscription(ctx context.Context, subscription *domain.LinkSubscription) error
	RemoveSubscriptionsByUserID(ctx context.Context, userID int64) error
	GetSubscriptionsByResourceID(ctx context.Context, resourceID uuid.UUID) ([]*domain.LinkSubscription, error)
	GetSubscriptionsByUserID(ctx context.Context, userID int64) ([]*domain.LinkSubscription, error)
	UpdateSubscriptionLastUpdate(ctx context.Context, userID int64, resourceID uuid.UUID, updatedAt time.Time) error
}

type SourceCrawler interface {
	SearchResource(ctx context.Context, link domain.TrackedLink) ([]*ResourceShot, error)
}

type OutboxRepository interface {
	AddBatch(ctx context.Context, shot []*ResourceShot) error
}
