package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/clients/telegram"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/repository/postgres/channels"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/usecase"
	"github.com/passwordhash/cascade-hub-bot/internal/config"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	publicationSlots "github.com/passwordhash/cascade-hub-bot/internal/publication/repository/postgres/slots"
	publicationSummary "github.com/passwordhash/cascade-hub-bot/internal/publication/repository/postgres/summary"
	publicationUseCase "github.com/passwordhash/cascade-hub-bot/internal/publication/usecase"
	"github.com/passwordhash/cascade-hub-bot/internal/report/repository/postgres/reports"
	reportUseCase "github.com/passwordhash/cascade-hub-bot/internal/report/usecase"
	updateOffset "github.com/passwordhash/cascade-hub-bot/internal/telegramupdate/repository/postgres/offset"
	telegramUpdateUseCase "github.com/passwordhash/cascade-hub-bot/internal/telegramupdate/usecase"
)

type App struct {
	channelUseCase *usecase.ChannelUseCase
	lifecycle      *publicationUseCase.LifecycleUseCase
	pool           *pgxpool.Pool
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	pool, err := initPostgresDB(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	getter := postgres.NewGetter(pool)
	channelRepository := channels.New(getter)
	slotRepository := publicationSlots.New(getter)
	reportRepository := reports.New(getter)
	summaryRepository := publicationSummary.New(getter)
	offsetRepository := updateOffset.New(getter)
	telegramClient := telegram.New(cfg.Telegram.Token)
	notifier := ownerNotifier{telegram: telegramClient, ownerID: cfg.Telegram.OwnerID}
	ingestion := reportUseCase.NewIngestion(channelRepository, slotRepository, reportRepository)
	processor := telegramUpdateUseCase.NewProcessor(pool, offsetRepository, ingestion)
	channelUseCase := usecase.NewChannelUseCase(cfg.Telegram.OwnerID, channelRepository, slotRepository, telegramClient, processor)
	prepare := publicationUseCase.NewPrepareUseCase(slotRepository, reportRepository, summaryRepository)
	attempt, err := publicationUseCase.NewAttemptUseCase(slotRepository, channelRepository, telegramClient, notifier, cfg.Telegram.OwnerID, cfg.MeetingURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create publication attempt: %w", err)
	}
	scheduler, err := publicationUseCase.NewSchedulerUseCase(channelRepository, slotRepository, prepare, attempt, notifier)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create publication scheduler: %w", err)
	}
	delivery, err := publicationUseCase.NewSummaryDeliveryUseCase(slotRepository, summaryRepository, channelRepository, telegramClient, notifier, cfg.Telegram.OwnerID)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create summary delivery: %w", err)
	}
	dispatcher, err := publicationUseCase.NewSummaryDispatcherUseCase(slotRepository, summaryRepository, delivery)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create summary dispatcher: %w", err)
	}
	lifecycle, err := publicationUseCase.NewLifecycleUseCase(scheduler, dispatcher, retention{reports: reportRepository, summaries: summaryRepository})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create publication lifecycle: %w", err)
	}
	return &App{channelUseCase: channelUseCase, lifecycle: lifecycle, pool: pool}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.pool.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); errors <- a.channelUseCase.Run(ctx) }()
	go func() { defer workers.Done(); errors <- a.lifecycle.Run(ctx) }()
	firstError := <-errors
	cancel()
	workers.Wait()
	if ctx.Err() != nil && firstError == nil {
		return nil
	}
	return firstError
}

func Run(ctx context.Context) error {
	a, err := New(ctx)
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
