package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

type Processor struct {
	transactions transactionStarter
	offset       updateOffset
	ingestion    reportIngestion
}

func NewProcessor(transactions transactionStarter, offset updateOffset, ingestion reportIngestion) *Processor {
	return &Processor{transactions: transactions, offset: offset, ingestion: ingestion}
}

func (p *Processor) GetOffset(ctx context.Context) (int64, error) {
	return p.offset.Get(ctx)
}

func (p *Processor) Process(ctx context.Context, update channelmodels.Update) (resultErr error) {
	if update.ID <= 0 {
		return errors.New("process Telegram update: invalid update ID")
	}
	tx, err := p.transactions.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Telegram update transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback Telegram update transaction: %w", err))
		}
	}()

	txCtx := postgres.WithTx(ctx, tx)
	nextUpdateID, err := p.offset.Get(txCtx)
	if err != nil {
		return fmt.Errorf("read Telegram update offset: %w", err)
	}
	if update.ID < nextUpdateID {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit duplicate Telegram update: %w", err)
		}
		return nil
	}
	if reportUpdate(update) {
		if err := p.ingestion.Handle(txCtx, update); err != nil {
			if errors.Is(err, channelmodels.ErrUpdatePending) {
				return channelmodels.ErrUpdatePending
			}
			return fmt.Errorf("process report update: %w", err)
		}
	}
	if _, err := p.offset.Set(txCtx, update.ID+1); err != nil {
		return fmt.Errorf("persist Telegram update offset: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Telegram update: %w", err)
	}
	return nil
}

func reportUpdate(update channelmodels.Update) bool {
	return (update.Kind == "message" || update.Kind == "edited_message") &&
		(update.ChatType == "group" || update.ChatType == "supergroup")
}
