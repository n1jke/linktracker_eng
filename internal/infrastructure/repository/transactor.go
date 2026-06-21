package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNilTransactors = errors.New("nil pgx pool")

type Querier interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

func injectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func GetQuerier(ctx context.Context, defaultQuerier Querier) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}

	return defaultQuerier
}

type TxChain struct {
	db *pgxpool.Pool
}

func NewTxChain(pool *pgxpool.Pool) *TxChain {
	return &TxChain{pool}
}

func (c *TxChain) WithTransaction(ctx context.Context, txFunc func(ctx context.Context) error) error {
	if c == nil || c.db == nil {
		return ErrNilTransactors
	}

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback(ctx))
		}
	}()

	err = txFunc(injectTx(ctx, tx))
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
