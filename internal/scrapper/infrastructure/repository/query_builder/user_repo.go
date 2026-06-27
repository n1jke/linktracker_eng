package qb

import (
	"context"
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres" // deialect for Goqu
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

var goquDB = goqu.Dialect("postgres")

type UserRepoGoqu struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewUserRepoGoqu(db *pgxpool.Pool, m MetricsRecorder) *UserRepoGoqu {
	return &UserRepoGoqu{
		db:      db,
		metrics: m,
	}
}

func (s *UserRepoGoqu) AddClient(ctx context.Context, client *domain.Client) error {
	defer s.metrics.Observe(scope, usersTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Insert(usersTable).Rows(goqu.Record{
		columnChatID: client.ID(),
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

func (s *UserRepoGoqu) RemoveClient(ctx context.Context, clientID int64) error {
	defer s.metrics.Observe(scope, usersTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.Delete(usersTable).Where(goqu.Ex{columnChatID: clientID}).Prepared(true).ToSQL()
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

func (s *UserRepoGoqu) GetClientByID(ctx context.Context, clientID int64) (*domain.Client, error) {
	defer s.metrics.Observe(scope, usersTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	query, args, err := goquDB.From(usersTable).Select(columnChatID).Where(goqu.Ex{columnChatID: clientID}).Prepared(true).ToSQL()
	if err != nil {
		return nil, err
	}

	var id int64

	row := querier.QueryRow(ctx, query, args...)
	if err = row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.NewClient(id), nil
}
