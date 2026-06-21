package qb

import (
	"context"
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type MetricsRecorder interface {
	Observe(scope, scopeType string, start time.Time)
}

const scope = "database"

type LinkRepoGoqu struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewLinkRepoGoqu(db *pgxpool.Pool, m MetricsRecorder) *LinkRepoGoqu {
	return &LinkRepoGoqu{
		db:      db,
		metrics: m,
	}
}

func (l *LinkRepoGoqu) AddLink(ctx context.Context, link *domain.TrackedLink) error {
	defer l.metrics.Observe(scope, linksTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	query, args, err := goquDB.Insert(linksTable).Rows(goqu.Record{
		columnLinkID:       link.ID(),
		columnPath:         link.Path(),
		columnResourceType: link.Type(),
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

func (l *LinkRepoGoqu) RemoveLink(ctx context.Context, linkID uuid.UUID) error {
	defer l.metrics.Observe(scope, linksTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	query, args, err := goquDB.Delete(linksTable).Where(goqu.Ex{columnLinkID: linkID}).Prepared(true).ToSQL()
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

func (l *LinkRepoGoqu) GetLinkByID(ctx context.Context, linkID uuid.UUID) (*domain.TrackedLink, error) {
	defer l.metrics.Observe(scope, linksTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	query, args, err := goquDB.From(linksTable).
		Select(columnLinkID, columnPath, columnResourceType).
		Where(goqu.Ex{columnLinkID: linkID}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, err
	}

	var (
		id    uuid.UUID
		path  string
		rType domain.LinkSource
	)

	row := querier.QueryRow(ctx, query, args...)
	if err = row.Scan(&id, &path, &rType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.CreateTrackedLink(id, path, rType)
}

func (l *LinkRepoGoqu) GetLinkByName(ctx context.Context, name string) (*domain.TrackedLink, error) {
	defer l.metrics.Observe(scope, linksTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)
	normalizedName := name

	if trackedLink, err := domain.NewTrackedLink(name); err == nil {
		normalizedName = trackedLink.Path()
	}

	query, args, err := goquDB.From(linksTable).
		Select(columnLinkID, columnPath, columnResourceType).
		Where(goqu.Ex{columnPath: normalizedName}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, err
	}

	var (
		id    uuid.UUID
		path  string
		rType domain.LinkSource
	)

	row := querier.QueryRow(ctx, query, args...)
	if err = row.Scan(&id, &path, &rType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.CreateTrackedLink(id, path, rType)
}

func (l *LinkRepoGoqu) GetLinksBatch(ctx context.Context, limit int, pointer *time.Time) ([]*domain.TrackedLink, *time.Time, error) {
	if limit < 0 {
		return nil, nil, ErrInvalidLimit
	}

	defer l.metrics.Observe(scope, linksTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	ds := goquDB.From(linksTable).
		Select(columnLinkID, columnPath, columnResourceType, columnCreatedAt).
		Order(goqu.I(columnCreatedAt).Asc()).
		Limit(uint(limit))

	if pointer != nil {
		ds = ds.Where(goqu.C(columnCreatedAt).Gt(*pointer))
	}

	query, args, err := ds.Prepared(true).ToSQL()
	if err != nil {
		return nil, nil, err
	}

	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	links := make([]*domain.TrackedLink, 0, limit)

	var (
		id          uuid.UUID
		path        string
		rType       domain.LinkSource
		createdAt   time.Time
		lastCreated *time.Time
	)

	for rows.Next() {
		if err = rows.Scan(&id, &path, &rType, &createdAt); err != nil {
			continue
		}

		l, createErr := domain.CreateTrackedLink(id, path, rType)
		if createErr != nil {
			continue
		}

		links = append(links, l)
		lastCreated = new(createdAt)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return links, lastCreated, nil
}
