package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/n1jke/linktracker/internal/agent/application"
	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
)

const (
	addOutboxBatch = `
		INSERT INTO agent_outbox(update_id, url, description, chat_ids, priority)
		VALUES ($1, $2, $3, $4, $5)
	`

	getPendingOutbox = `
		SELECT update_id, url, description, chat_ids, priority
		FROM agent_outbox
		WHERE processed_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	updErrorOutbox = `
		UPDATE agent_outbox
		SET retry_count = retry_count + 1, error = $2
		WHERE update_id = $1
	`

	updSuccessOutbox = `
		UPDATE agent_outbox
		SET processed_at = NOW(), error = NULL
		WHERE update_id = $1
	`

	deleteOldOutbox = `
		DELETE FROM agent_outbox
		WHERE processed_at IS NOT NULL AND processed_at < $1
	`
)

type OutboxRepo struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

func NewOutboxRepo(logger *slog.Logger, db *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{
		logger: logger.With("module", "outbox-repo"),
		db:     db,
	}
}

func (o *OutboxRepo) AddBatch(ctx context.Context, updates []*application.ProcessedUpdate) error {
	querier := sharedrepo.GetQuerier(ctx, o.db)
	batch := &pgx.Batch{}

	for i := range updates {
		batch.Queue(addOutboxBatch, updates[i].ID, updates[i].Link, updates[i].Description, updates[i].ChatIDs, string(updates[i].Priority))
	}

	batchResults := querier.SendBatch(ctx, batch)

	return batchResults.Close()
}

func (o *OutboxRepo) FetchPending(ctx context.Context, limit int) ([]*application.ProcessedUpdate, error) {
	querier := sharedrepo.GetQuerier(ctx, o.db)

	rows, err := querier.Query(ctx, getPendingOutbox, limit)
	if err != nil {
		o.logger.Error("getPendingOutbox query", slog.Any("err", err))
		return nil, err
	}
	defer rows.Close()

	var records []*application.ProcessedUpdate

	for rows.Next() {
		var rec application.ProcessedUpdate

		err := rows.Scan(&rec.ID, &rec.Link, &rec.Description, &rec.ChatIDs, &rec.Priority)
		if err != nil {
			o.logger.Warn("fail scan row", slog.Any("err", err))
			continue
		}

		records = append(records, &rec)
	}

	if err := rows.Err(); err != nil {
		o.logger.Error("processing getPendingOutbox", slog.Any("err", err))
		return nil, err
	}

	return records, nil
}

func (o *OutboxRepo) UpdateStatus(ctx context.Context, id int64, errIn error) error {
	querier := sharedrepo.GetQuerier(ctx, o.db)

	if errIn != nil {
		_, err := querier.Exec(ctx, updErrorOutbox, id, errIn.Error())
		if err != nil {
			o.logger.Error("updErrorOutbox query", slog.Any("err", err))
		}

		return err
	}

	_, err := querier.Exec(ctx, updSuccessOutbox, id)
	if err != nil {
		o.logger.Error("updSuccessOutbox query", slog.Any("err", err))
	}

	return err
}

func (o *OutboxRepo) Cleanup(ctx context.Context, gap time.Duration) (int64, error) {
	querier := sharedrepo.GetQuerier(ctx, o.db)
	cut := time.Now().Add(-gap)

	tag, err := querier.Exec(ctx, deleteOldOutbox, cut)
	if err != nil {
		o.logger.Error("deleteOldOutbox query", slog.Any("err", err))
		return 0, err
	}

	return tag.RowsAffected(), nil
}
