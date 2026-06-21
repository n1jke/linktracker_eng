package rawsql

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

const (
	addSubscription = `
		INSERT INTO subscriptions(user_id, res_id, link, tags, last_update) 
		VALUES ($1, $2, $3, $4, $5)
	`

	deleteSubscription = `
		DELETE FROM subscriptions 
		WHERE user_id = $1 AND res_id = $2 AND link = $3
	`

	deleteUserSubscriptions = `
		DELETE FROM subscriptions
		WHERE user_id = $1
	`

	addSubscriptionTags = `
		UPDATE subscriptions
		SET tags = (
			SELECT COALESCE(array_agg(DISTINCT temp), ARRAY[]::text[])
			FROM unnest(COALESCE(subscriptions.tags, ARRAY[]::text[]) || $1::text[]) AS temp
		)
		WHERE user_id = $2 AND res_id = $3
	`

	deleteSubscriptionTags = `
		UPDATE subscriptions 
		SET tags = ARRAY[]::text[] 
		WHERE user_id = $1 AND res_id = $2
	`

	getSubscriptionByResourceID = `
		SELECT user_id, res_id,link, tags, last_update FROM subscriptions 
		WHERE res_id = $1
	`

	getSubscriptionByUserID = `
		SELECT user_id, res_id,link, tags, last_update FROM subscriptions 
		WHERE user_id = $1
	`

	updateSubscriptionLastUpdate = `
		UPDATE subscriptions 
		SET last_update = $1 
		WHERE user_id = $2 AND res_id = $3
	`
)

type SubscriptionsRepoSQL struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewSubscriptionsRepoSQL(db *pgxpool.Pool, m MetricsRecorder) *SubscriptionsRepoSQL {
	return &SubscriptionsRepoSQL{
		db:      db,
		metrics: m,
	}
}

func (s *SubscriptionsRepoSQL) AddSubscription(ctx context.Context, subscription *domain.LinkSubscription) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	_, err := querier.Exec(ctx, addSubscription, subscription.UserID(), subscription.ResourceID(),
		subscription.Link(), subscription.Tags(), subscription.LastUpdate())
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			if pgErr.Code == uniqueViolation {
				return application.ErrAlreadyExists
			}
		}

		return err
	}

	return nil
}

func (s *SubscriptionsRepoSQL) RemoveSubscription(ctx context.Context, subscription *domain.LinkSubscription) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, deleteSubscription, subscription.UserID(), subscription.ResourceID(), subscription.Link())
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoSQL) RemoveSubscriptionsByUserID(ctx context.Context, userID int64) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, deleteUserSubscriptions, userID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoSQL) AddTags(ctx context.Context, userID int64, resourceID uuid.UUID, tags []string) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, addSubscriptionTags, tags, userID, resourceID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoSQL) ClearTags(ctx context.Context, userID int64, resourceID uuid.UUID) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, deleteSubscriptionTags, userID, resourceID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoSQL) GetSubscriptionsByResourceID(ctx context.Context, resourceID uuid.UUID) ([]*domain.LinkSubscription, error) {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	rows, err := querier.Query(ctx, getSubscriptionByResourceID, resourceID)
	if err != nil {
		return nil, err
	}

	subs := make([]*domain.LinkSubscription, 0)

	var (
		userID     int64
		resID      uuid.UUID
		link       string
		tags       []string
		lastUpdate time.Time
	)

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&userID, &resID, &link, &tags, &lastUpdate); err != nil {
			continue
		}

		sub := domain.NewLinkSubscription(userID, resID, link, tags...)
		sub.SetLastUpdate(lastUpdate)
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

func (s *SubscriptionsRepoSQL) GetSubscriptionsByUserID(ctx context.Context, userID int64) ([]*domain.LinkSubscription, error) {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	rows, err := querier.Query(ctx, getSubscriptionByUserID, userID)
	if err != nil {
		return nil, err
	}

	subs := make([]*domain.LinkSubscription, 0)

	var (
		user       int64
		resID      uuid.UUID
		link       string
		tags       []string
		lastUpdate time.Time
	)

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&user, &resID, &link, &tags, &lastUpdate); err != nil {
			continue
		}

		sub := domain.NewLinkSubscription(user, resID, link, tags...)
		sub.SetLastUpdate(lastUpdate)
		subs = append(subs, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

func (s *SubscriptionsRepoSQL) UpdateSubscriptionLastUpdate(ctx context.Context, userID int64, resourceID uuid.UUID,
	updatedAt time.Time,
) error {
	defer s.metrics.Observe(scope, subsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, updateSubscriptionLastUpdate, updatedAt, userID, resourceID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}
