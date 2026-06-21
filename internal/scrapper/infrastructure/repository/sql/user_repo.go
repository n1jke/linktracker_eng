package rawsql

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

const (
	uniqueViolation string = "23505"

	addClient = `
		INSERT INTO users(chat_id)
		VALUES ($1)
	`

	deleteClient = `
		DELETE FROM users 
		WHERE chat_id = $1
	`

	getClientByID = `
		SELECT chat_id FROM users 
		WHERE chat_id = $1
	`
)

type UserRepoSQL struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewUserRepoSQL(db *pgxpool.Pool, m MetricsRecorder) *UserRepoSQL {
	return &UserRepoSQL{
		db:      db,
		metrics: m,
	}
}

func (s *UserRepoSQL) AddClient(ctx context.Context, client *domain.Client) error {
	defer s.metrics.Observe(scope, userTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	_, err := querier.Exec(ctx, addClient, client.ID())
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

func (s *UserRepoSQL) RemoveClient(ctx context.Context, clientID int64) error {
	defer s.metrics.Observe(scope, userTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	commandTag, err := querier.Exec(ctx, deleteClient, clientID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	return nil
}

func (s *UserRepoSQL) GetClientByID(ctx context.Context, clientID int64) (*domain.Client, error) {
	defer s.metrics.Observe(scope, userTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, s.db)

	var id int64

	row := querier.QueryRow(ctx, getClientByID, clientID)
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, application.ErrNotFound
		}

		return nil, err
	}

	return domain.NewClient(id), nil
}
