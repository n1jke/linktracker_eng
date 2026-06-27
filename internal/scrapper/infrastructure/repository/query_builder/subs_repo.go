package qb

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

type SubscriptionsRepoGoqu struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewSubscriptionsRepoGoqu(db *pgxpool.Pool, m MetricsRecorder) *SubscriptionsRepoGoqu {
	return &SubscriptionsRepoGoqu{
		db:      db,
		metrics: m,
	}
}

func (s *SubscriptionsRepoGoqu) AddSubscription(ctx context.Context, subscription *domain.LinkSubscription) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Insert(subscriptionsTable).Rows(goqu.Record{
		columnUserID:     subscription.UserID(),
		columnResID:      subscription.ResourceID(),
		columnLink:       subscription.Link(),
		columnTags:       goqu.L("?::text[]", toTextArrayLiteral(subscription.Tags())),
		columnLastUpdate: subscription.LastUpdate(),
	}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	_, err = querier.Exec(ctx, query, args...)
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

func toTextArrayLiteral(tags []string) string {
	if len(tags) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(tags))
	for i := range tags {
		item := strings.ReplaceAll(tags[i], `\`, `\\`)
		item = strings.ReplaceAll(item, `"`, `\"`)
		parts = append(parts, `"`+item+`"`)
	}

	return "{" + strings.Join(parts, ",") + "}"
}

func (s *SubscriptionsRepoGoqu) RemoveSubscription(ctx context.Context, subscription *domain.LinkSubscription) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Delete(subscriptionsTable).Where(goqu.Ex{
		columnUserID: subscription.UserID(),
		columnResID:  subscription.ResourceID(),
		columnLink:   subscription.Link(),
	}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	commandTag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoGoqu) RemoveSubscriptionsByUserID(ctx context.Context, userID int64) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Delete(subscriptionsTable).Where(goqu.Ex{columnUserID: userID}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	commandTag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoGoqu) AddTags(ctx context.Context, userID int64, resourceID uuid.UUID, tags []string) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Update(subscriptionsTable).Set(goqu.Record{
		"tags": goqu.L(`(
			SELECT COALESCE(array_agg(DISTINCT t), ARRAY[]::text[])
			FROM unnest(COALESCE("subscriptions"."tags", ARRAY[]::text[]) || ?::text[]) AS t
		)`, toTextArrayLiteral(tags)),
	}).Where(goqu.Ex{
		columnUserID: userID,
		columnResID:  resourceID,
	}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	commandTag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoGoqu) ClearTags(ctx context.Context, userID int64, resourceID uuid.UUID) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Update(subscriptionsTable).Set(goqu.Record{
		"tags": goqu.L(`ARRAY[]::text[]`),
	}).Where(goqu.Ex{
		columnUserID: userID,
		columnResID:  resourceID,
	}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	commandTag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *SubscriptionsRepoGoqu) GetSubscriptionsByResourceID(ctx context.Context, resourceID uuid.UUID,
) ([]*domain.LinkSubscription, error) {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.From(subscriptionsTable).
		Select(columnUserID, columnResID, columnLink, columnTags, columnLastUpdate).
		Where(goqu.Ex{columnResID: resourceID}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := make([]*domain.LinkSubscription, 0)

	var (
		userID     int64
		resID      uuid.UUID
		link       string
		tags       []string
		lastUpdate time.Time
	)

	for rows.Next() {
		if err = rows.Scan(&userID, &resID, &link, &tags, &lastUpdate); err != nil {
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

func (s *SubscriptionsRepoGoqu) GetSubscriptionsByUserID(ctx context.Context, userID int64) ([]*domain.LinkSubscription, error) {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.From(subscriptionsTable).
		Select(columnUserID, columnResID, columnLink, columnTags, columnLastUpdate).
		Where(goqu.Ex{columnUserID: userID}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := make([]*domain.LinkSubscription, 0)

	var (
		user       int64
		resID      uuid.UUID
		link       string
		tags       []string
		lastUpdate time.Time
	)

	for rows.Next() {
		if err = rows.Scan(&user, &resID, &link, &tags, &lastUpdate); err != nil {
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

func (s *SubscriptionsRepoGoqu) UpdateSubscriptionLastUpdate(ctx context.Context, userID int64, resourceID uuid.UUID,
	updatedAt time.Time,
) error {
	defer s.metrics.Observe(scope, subscriptionsTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Update(subscriptionsTable).Set(goqu.Record{
		columnLastUpdate: updatedAt,
	}).Where(goqu.Ex{
		columnUserID: userID,
		columnResID:  resourceID,
	}).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	commandTag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}
