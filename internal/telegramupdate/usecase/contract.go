package usecase

import (
	"context"

	"github.com/jackc/pgx/v5"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

type updateOffset interface {
	Get(context.Context) (int64, error)
	Set(context.Context, int64) (int64, error)
}

type reportIngestion interface {
	Handle(context.Context, channelmodels.Update) error
}

type transactionStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}
