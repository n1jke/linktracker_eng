package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	consumer "github.com/n1jke/linktracker/internal/bot/infrastructure/kafka"
	"github.com/n1jke/linktracker/internal/infrastructure/repository"
)

const (
	scope           = "database"
	inboxTable      = "bot_inbox"
	uniqueViolation = "23505"

	addRecordInbox = `
		INSERT INTO bot_inbox(idempotency_key, shot_id, url, description, chat_ids)
		VALUES ($1, $2, $3, $4, $5);
	`

	fetchPendingInbox = `
		SELECT shot_id, url, description, chat_ids
		FROM bot_inbox
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	markProcessedInbox = `
		UPDATE bot_inbox
		SET status = 'processed'
		WHERE shot_id = $1;
	`

	deleteOldInbox = `
		DELETE FROM bot_inbox
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

func (i *InboxRepo) AddRecord(ctx context.Context, rec *consumer.InboxRecord) error {
	querier := repository.GetQuerier(ctx, i.db)

	_, err := querier.Exec(ctx, addRecordInbox, rec.IdempotencyKey, rec.Record.ID, rec.Record.URL,
		rec.Record.Description, rec.Record.ChatIDs)
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

func (i *InboxRepo) FetchPending(ctx context.Context, batchSize int) ([]*consumer.Update, error) {
	querier := repository.GetQuerier(ctx, i.db)

	rows, err := querier.Query(ctx, fetchPendingInbox, batchSize)
	if err != nil {
		i.logger.Error("fetchPendingInbox query", slog.Any("err", err))
		return nil, err
	}
	defer rows.Close()

	records := make([]*consumer.Update, 0, batchSize)

	for rows.Next() {
		var raw consumer.Update

		err := rows.Scan(&raw.ID, &raw.URL, &raw.Description, &raw.ChatIDs)
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

func (i *InboxRepo) MarkProcessed(ctx context.Context, updates []*consumer.Update) error {
	querier := repository.GetQuerier(ctx, i.db)
	batch := &pgx.Batch{}

	for i := range updates {
		batch.Queue(markProcessedInbox, updates[i].ID)
	}

	batchResult := querier.SendBatch(ctx, batch)

	return batchResult.Close()
}

func (i *InboxRepo) Cleanup(ctx context.Context, gap time.Duration) error {
	querier := repository.GetQuerier(ctx, i.db)
	cut := time.Now().Add(-gap)

	_, err := querier.Exec(ctx, deleteOldInbox, cut)
	if err != nil {
		i.logger.Error("cleanup inbox", slog.Any("err", err))
		return err
	}

	return nil
}
