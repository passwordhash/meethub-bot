package usecase

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/repository/postgres/channels"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	publicationSlots "github.com/passwordhash/cascade-hub-bot/internal/publication/repository/postgres/slots"
	reportRepositories "github.com/passwordhash/cascade-hub-bot/internal/report/repository/postgres/reports"
	reportUseCase "github.com/passwordhash/cascade-hub-bot/internal/report/usecase"
	offsetRepository "github.com/passwordhash/cascade-hub-bot/internal/telegramupdate/repository/postgres/offset"
)

func TestProcessorPersistsOffsetAcrossInstancesAndRollsBackIngestionFailure(t *testing.T) {
	databaseURL := os.Getenv("REPORT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("REPORT_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	schema := fmt.Sprintf("processor_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := adminPool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	}()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
CREATE TABLE telegram_update_offset (id SMALLINT PRIMARY KEY, next_update_id BIGINT NOT NULL);
INSERT INTO telegram_update_offset VALUES (1, 5);
CREATE TABLE channels (channel_id BIGINT PRIMARY KEY, linked_chat_id BIGINT NOT NULL);
INSERT INTO channels VALUES (-100, -1);`); err != nil {
		t.Fatal(err)
	}
	getter := postgres.NewGetter(pool)
	offsets := offsetRepository.New(getter)
	ingestion := reportUseCase.NewIngestion(channels.New(getter), publicationSlots.New(getter), reportRepositories.New(getter))
	firstProcessor := NewProcessor(pool, offsets, ingestion)
	if offset, err := firstProcessor.GetOffset(ctx); err != nil || offset != 5 {
		t.Fatalf("initial offset = %d, error = %v; want 5, nil", offset, err)
	}
	update := channelmodels.Update{ID: 6, Kind: "channel_post"}
	if err := firstProcessor.Process(ctx, update); err != nil {
		t.Fatal(err)
	}
	secondProcessor := NewProcessor(pool, offsets, ingestion)
	if err := secondProcessor.Process(ctx, update); err != nil {
		t.Fatal(err)
	}
	if err := secondProcessor.Process(ctx, channelmodels.Update{ID: 7, Kind: "message", ChatType: "group", ChatID: -1, FromID: 42, MessageID: 8, ReplyToMessageID: 9, Date: 1, Text: "report"}); err == nil {
		t.Fatal("expected ingestion failure")
	}
	if offset, err := secondProcessor.GetOffset(ctx); err != nil || offset != 7 {
		t.Fatalf("persisted offset = %d, error = %v; want 7, nil", offset, err)
	}
}
