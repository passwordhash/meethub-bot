package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Getter interface {
	Get(context.Context) DB
}

type transactionContextKey struct{}

type poolGetter struct {
	pool *pgxpool.Pool
}

func NewGetter(pool *pgxpool.Pool) Getter {
	return poolGetter{pool: pool}
}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, tx)
}

func (g poolGetter) Get(ctx context.Context) DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return g.pool
}
