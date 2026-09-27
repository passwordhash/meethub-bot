package app

import (
	"context"

	"github.com/passwordhash/cascade-hub-bot/internal/publication/repository/postgres/summary"
	"github.com/passwordhash/cascade-hub-bot/internal/report/repository/postgres/reports"
)

type retention struct {
	reports   *reports.Repository
	summaries *summary.Repository
}

func (r retention) PurgeExpiredText(ctx context.Context) error {
	return r.reports.PurgeExpiredText(ctx)
}

func (r retention) PurgeExpired(ctx context.Context) error {
	return r.summaries.PurgeExpired(ctx)
}
