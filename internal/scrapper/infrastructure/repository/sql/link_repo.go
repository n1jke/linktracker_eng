package rawsql

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

type MetricsRecorder interface {
	Observe(scope, scopeType string, start time.Time)
}

const (
	scope       = "database"
	linkTable   = "links"
	userTable   = "users"
	subsTable   = "subscriptions"
	outboxTable = "outbox"
)

var ErrInvalidLimit = errors.New("invalid limit count")

const (
	addLink = `
		INSERT INTO links(link_id, path, resource_type) 
		VALUES ($1, $2, $3)
	`

	deleteLink = `
		DELETE FROM links 
		WHERE link_id = $1
	`

	getLinkByID = `
		SELECT link_id, path, resource_type FROM links
		WHERE link_id = $1
	`

	getLinkByName = `
		SELECT link_id, path, resource_type FROM links 
		WHERE path = $1
	`

	getFirstLinkBatch = `
		SELECT link_id, path, resource_type, created_at FROM links 
		ORDER BY created_at ASC, link_id ASC
		LIMIT $1
	`

	getLinksPtrBatch = `
		SELECT link_id, path, resource_type, created_at FROM links 
		WHERE created_at > $1 
		ORDER BY created_at ASC, link_id ASC 
		LIMIT $2
	`
)

type LinkRepoSQL struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewLinkRepoSQL(db *pgxpool.Pool, m MetricsRecorder) *LinkRepoSQL {
	return &LinkRepoSQL{
		db:      db,
		metrics: m,
	}
}

func (l *LinkRepoSQL) AddLink(ctx context.Context, link *domain.TrackedLink) error {
	defer l.metrics.Observe(scope, linkTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	_, err := querier.Exec(ctx, addLink, link.ID(), link.Path(), link.Type())
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

func (l *LinkRepoSQL) RemoveLink(ctx context.Context, linkID uuid.UUID) error {
	defer l.metrics.Observe(scope, linkTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	commandTag, err := querier.Exec(ctx, deleteLink, linkID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (l *LinkRepoSQL) GetLinkByID(ctx context.Context, linkID uuid.UUID) (*domain.TrackedLink, error) {
	defer l.metrics.Observe(scope, linkTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	var (
		id    uuid.UUID
		path  string
		rType domain.LinkSource
	)

	row := querier.QueryRow(ctx, getLinkByID, linkID)
	if err := row.Scan(&id, &path, &rType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.CreateTrackedLink(id, path, rType)
}

func (l *LinkRepoSQL) GetLinkByName(ctx context.Context, name string) (*domain.TrackedLink, error) {
	defer l.metrics.Observe(scope, linkTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)
	normalizedName := name

	if trackedLink, err := domain.NewTrackedLink(name); err == nil {
		normalizedName = trackedLink.Path()
	}

	var (
		id    uuid.UUID
		path  string
		rType domain.LinkSource
	)

	row := querier.QueryRow(ctx, getLinkByName, normalizedName)
	if err := row.Scan(&id, &path, &rType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.CreateTrackedLink(id, path, rType)
}

func (l *LinkRepoSQL) GetLinksBatch(ctx context.Context, limit int, pointer *time.Time) ([]*domain.TrackedLink, *time.Time, error) {
	if limit < 0 {
		return nil, nil, ErrInvalidLimit
	}

	defer l.metrics.Observe(scope, linkTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, l.db)

	var (
		rows pgx.Rows
		err  error
	)

	if pointer == nil {
		rows, err = querier.Query(ctx, getFirstLinkBatch, limit)
	} else {
		rows, err = querier.Query(ctx, getLinksPtrBatch, *pointer, limit)
	}

	if err != nil {
		return nil, nil, err
	}

	links := make([]*domain.TrackedLink, 0, limit)

	var (
		id          uuid.UUID
		path        string
		rType       domain.LinkSource
		createdAt   time.Time
		lastCreated *time.Time
	)

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&id, &path, &rType, &createdAt); err != nil {
			continue
		}

		l, err := domain.CreateTrackedLink(id, path, rType)
		if err != nil {
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
