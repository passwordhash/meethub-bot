package offset

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

type Repository struct{ getter postgres.Getter }

func New(getter postgres.Getter) *Repository { return &Repository{getter: getter} }

func (r *Repository) Get(ctx context.Context) (int64, error) {
	rows, err := r.getter.Get(ctx).Query(ctx, getQuery())
	if err != nil {
		return 0, fmt.Errorf("get Telegram update offset: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[offsetEntity])
	if err != nil {
		return 0, fmt.Errorf("get Telegram update offset: %w", err)
	}
	return entity.NextUpdateID, nil
}

func (r *Repository) Set(ctx context.Context, value int64) (int64, error) {
	query, args := setQuery(value)
	rows, err := r.getter.Get(ctx).Query(ctx, query, args)
	if err != nil {
		return 0, fmt.Errorf("set Telegram update offset: %w", err)
	}
	entity, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[offsetEntity])
	if err != nil {
		return 0, fmt.Errorf("set Telegram update offset: %w", err)
	}
	return entity.NextUpdateID, nil
}
