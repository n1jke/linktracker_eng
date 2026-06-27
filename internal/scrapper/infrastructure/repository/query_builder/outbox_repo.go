package qb

import (
	"context"
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5/pgxpool"

	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository"
)

var ErrInvalidLimit = errors.New("invalid limit count")

type OutboxRepoGoqu struct {
	db      *pgxpool.Pool
	metrics MetricsRecorder
}

func NewOutboxRepoGoqu(db *pgxpool.Pool, m MetricsRecorder) *OutboxRepoGoqu {
	return &OutboxRepoGoqu{
		db:      db,
		metrics: m,
	}
}

func (o *OutboxRepoGoqu) AddBatch(ctx context.Context, shots []*application.ResourceShot) error {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)
	batch := make([]goqu.Record, 0, len(shots))

	for i := range shots {
		batch = append(batch, goqu.Record{
			"shot_id":     shots[i].ID,
			"url":         shots[i].URL,
			"event_type":  shots[i].EventType,
			"description": shots[i].Description,
			"author":      shots[i].Author,
			"chat_ids":    shots[i].ChatIDs,
			"updated_at":  shots[i].UpdatedAt,
		})
	}

	query, args, err := goquDB.Insert(outboxTable).Rows(batch).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	_, err = querier.Exec(ctx, query, args...)

	return err
}

func (o *OutboxRepoGoqu) FetchPending(ctx context.Context, limit int) ([]*repository.OutboxRecord, error) {
	if limit < 0 {
		return nil, ErrInvalidLimit
	}

	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)

	query, args, err := goquDB.From(outboxTable).
		Select(columnID, columnRetryCount, columnShotID, columnURL, columnEventType, columnDescription,
			columnAuthor, columnChatIDs, columnUpdatedAt).
		Where(goqu.Ex{columnProcessedAt: nil}).
		Order(goqu.I(columnCreatedAt).Asc()).
		Limit(uint(limit)).
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

func (o *OutboxRepoGoqu) UpdateStatus(ctx context.Context, id int64, errIn error) error {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)

	var (
		query string
		args  []any
		err   error
	)

	if errIn != nil {
		query, args, err = goquDB.Update(outboxTable).Set(goqu.Record{
			columnRetryCount: goqu.L(columnRetryCount + " + 1"),
			columnError:      errIn.Error(),
		}).Where(goqu.Ex{"id": id}).Prepared(true).ToSQL()
		if err != nil {
			return err
		}
	} else {
		query, args, err = goquDB.Update(outboxTable).Set(goqu.Record{
			columnProcessedAt: time.Now(),
			columnError:       nil,
		}).Where(goqu.Ex{"id": id}).Prepared(true).ToSQL()
		if err != nil {
			return err
		}
	}

	_, err = querier.Exec(ctx, query, args)

	return err
}

func (o *OutboxRepoGoqu) Cleanup(ctx context.Context, gap time.Duration) (int64, error) {
	defer o.metrics.Observe(scope, outboxTable, time.Now())

	querier := sharedrepo.GetQuerier(ctx, o.db)
	cut := time.Now().Add(-gap)

	query, args, err := goquDB.Delete(outboxTable).Where(
		goqu.C(columnProcessedAt).IsNotNull(),
		goqu.C(columnProcessedAt).Lt(cut),
	).Prepared(true).ToSQL()
	if err != nil {
		return 0, err
	}

	tag, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}
