package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// Querier is the subset of pgx that both a pool and a transaction satisfy.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type TxManager struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewTxManager(pool *pgxpool.Pool, log *slog.Logger) *TxManager {
	return &TxManager{pool: pool, log: log}
}

// Do runs fn in one transaction, joining the ambient one when there is one.
func (m *TxManager) Do(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			m.rollbackEvenIfRequestCancelled(ctx, tx)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

func (m *TxManager) rollbackEvenIfRequestCancelled(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(context.WithoutCancel(ctx)); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		m.log.Warn("rollback failed", "err", err)
	}
}

type db struct {
	pool *pgxpool.Pool
}

func (d *db) querier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return d.pool
}

// ExecInTx runs a statement on the transaction carried by ctx, or on the pool.
func ExecInTx(ctx context.Context, pool *pgxpool.Pool, statement string, args ...any) (pgconn.CommandTag, error) {
	source := db{pool: pool}
	return source.querier(ctx).Exec(ctx, statement, args...)
}
