package rawsql

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository"
)

const (
	addOutboxBatch = `
		INSERT INTO outbox(shot_id, url, event_type, description, author, chat_ids, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	getPendingOutbox = `
		SELECT id, retry_count, shot_id, url, event_type, description, author, chat_ids, updated_at FROM outbox 
		WHERE processed_at IS NULL 
		ORDER BY created_at ASC
		LIMIT $1
	`

	updErrorOutbox = `
		UPDATE outbox
		SET retry_count = retry_count + 1, error = $2 
		WHERE id = $1
	`

	updSuccessOutbox = `
		UPDATE outbox 
		SET processed_at = NOW(), error = NULL 
		WHERE id = $1
	`

	deleteOldOutbox = `
		DELETE FROM outbox
		WHERE processed_at IS NOT NULL AND processed_at  < $1
	`
)

type OutboxRepoSQL struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewOutboxRepoSQL(db *pgxpool.Pool, m MetricsRecorder) *OutboxRepoSQL {
	return &OutboxRepoSQL{
		db:      db,
		metrics: m,
	}
}

func (o *OutboxRepoSQL) AddBatch(ctx context.Context, shots []*application.ResourceShot) error {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)
	batch := &pgx.Batch{}

	for i := range shots {
		batch.Queue(addOutboxBatch, shots[i].ID, shots[i].URL, shots[i].EventType, shots[i].Description,
			shots[i].Author, shots[i].ChatIDs, shots[i].UpdatedAt)
	}

	batchResults := querier.SendBatch(ctx, batch)

	return batchResults.Close()
}

func (o *OutboxRepoSQL) FetchPending(ctx context.Context, limit int) ([]*repository.OutboxRecord, error) {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)

	rows, err := querier.Query(ctx, getPendingOutbox, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*repository.OutboxRecord

	for rows.Next() {
		var rec repository.OutboxRecord

		rec.Shot = &application.ResourceShot{}

		err := rows.Scan(&rec.ID, &rec.RetryCount, &rec.Shot.ID, &rec.Shot.URL, &rec.Shot.EventType,
			&rec.Shot.Description, &rec.Shot.Author, &rec.Shot.ChatIDs, &rec.Shot.UpdatedAt)
		if err != nil {
			continue
		}

		records = append(records, &rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (o *OutboxRepoSQL) UpdateStatus(ctx context.Context, id int64, errIn error) error {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)

	if errIn != nil {
		_, err := querier.Exec(ctx, updErrorOutbox, id, errIn.Error())
		return err
	}

	_, err := querier.Exec(ctx, updSuccessOutbox, id)

	return err
}

func (o *OutboxRepoSQL) Cleanup(ctx context.Context, gap time.Duration) (int64, error) {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)
	cut := time.Now().Add(-gap)

	tag, err := querier.Exec(ctx, deleteOldOutbox, cut)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}
