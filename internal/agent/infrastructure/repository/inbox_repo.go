package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/n1jke/linktracker_eng/internal/agent/application"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/kafka/consumer"
	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
)

const (
	baseSize        = 64
	uniqueViolation = "23505"

	addRecordInbox = `
		INSERT INTO agent_inbox(update_id, url, event_type, description, author, chat_ids, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7);
	`

	fetchPendingInbox = `
		WITH window_start AS (
			SELECT created_at
			FROM agent_inbox
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		
		SELECT update_id, url, event_type, description, author, chat_ids, updated_at
		FROM agent_inbox
		WHERE status = 'pending'
		AND created_at <= (SELECT created_at FROM window_start) + $1::interval
		FOR UPDATE SKIP LOCKED;
	`

	markProcessedInbox = `
		UPDATE agent_inbox
		SET status = 'processed'
		WHERE update_id = $1;
	`

	deleteOldInbox = `
		DELETE FROM agent_inbox
		WHERE status = 'processed' AND created_at < $1
	`
)

type InboxRepo struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

func NewInboxRepo(logger *slog.Logger, db *pgxpool.Pool) *InboxRepo {
	return &InboxRepo{
		logger: logger.With("module", "inbox-repo"),
		db:     db,
	}
}

func (i *InboxRepo) AddRecord(ctx context.Context, raw *application.RawUpdate) error {
	querier := sharedrepo.GetQuerier(ctx, i.db)

	_, err := querier.Exec(ctx, addRecordInbox, raw.ID, raw.Link, raw.EventType, raw.Description, raw.Author, raw.ChatIDs, raw.UpdatedAt)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			if pgErr.Code == uniqueViolation {
				return consumer.ErrDuplicateMsg
			}

			i.logger.Error("addRecordInbox query", slog.Any("err", err))

			return err
		}
	}

	return nil
}

func (i *InboxRepo) FetchPending(ctx context.Context, window time.Duration) ([]*application.RawUpdate, error) {
	querier := sharedrepo.GetQuerier(ctx, i.db)

	rows, err := querier.Query(ctx, fetchPendingInbox, window)
	if err != nil {
		i.logger.Error("fetchPendingInbox query", slog.Any("err", err))
		return nil, err
	}
	defer rows.Close()

	records := make([]*application.RawUpdate, 0, baseSize)

	for rows.Next() {
		var raw application.RawUpdate

		err := rows.Scan(&raw.ID, &raw.Link, &raw.EventType, &raw.Description, &raw.Author, &raw.ChatIDs, &raw.UpdatedAt)
		if err != nil {
			i.logger.Warn("fail scan row", slog.Any("err", err))
			continue
		}

		records = append(records, &raw)
	}

	if err := rows.Err(); err != nil {
		i.logger.Error("processing fetchPendingInbox", slog.Any("err", err))
		return nil, err
	}

	return records, nil
}

func (i *InboxRepo) MarkProcessed(ctx context.Context, updates []*application.RawUpdate) error {
	querier := sharedrepo.GetQuerier(ctx, i.db)
	batch := &pgx.Batch{}

	for i := range updates {
		batch.Queue(markProcessedInbox, updates[i].ID)
	}

	batchResult := querier.SendBatch(ctx, batch)

	return batchResult.Close()
}

func (i *InboxRepo) Cleanup(ctx context.Context, gap time.Duration) (int64, error) {
	querier := sharedrepo.GetQuerier(ctx, i.db)
	cut := time.Now().Add(-gap)

	tag, err := querier.Exec(ctx, deleteOldInbox, cut)
	if err != nil {
		i.logger.Error("cleanup", slog.Any("err", err))
		return 0, err
	}

	return tag.RowsAffected(), nil
}
