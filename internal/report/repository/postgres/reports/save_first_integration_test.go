package reports

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func TestSaveFirstReportPostgres(t *testing.T) {
	connectionString := os.Getenv("REPORT_TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("REPORT_TEST_DATABASE_URL is unset; disposable PostgreSQL integration test not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "save_first_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	}()
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	_, err = isolated.Exec(ctx, `CREATE TABLE reports (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		slot_id UUID NOT NULL,
		user_id BIGINT NOT NULL,
		display_name TEXT NOT NULL,
		text TEXT NOT NULL,
		message_id BIGINT NOT NULL,
		message_at TIMESTAMPTZ NOT NULL,
		edit_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE (slot_id, user_id)
	)`)
	if err != nil {
		t.Fatal(err)
	}

	slotID := uuid.New()
	firstTime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	first := models.Report{SlotID: slotID, AuthorID: 42, Author: "Ada", Text: "first", MessageID: 100, MessageAt: firstTime}
	second := models.Report{SlotID: slotID, AuthorID: 42, Author: "Ada", Text: "second", MessageID: 101, MessageAt: firstTime.Add(time.Minute)}
	repository := New(postgres.NewGetter(isolated))
	for _, report := range []models.Report{first, second} {
		if err := repository.SaveFirst(ctx, report); err != nil {
			t.Fatal(err)
		}
	}
	editedAt := firstTime.Add(2 * time.Minute)
	first.Text = "edited first"
	first.EditAt = &editedAt
	if err := repository.SaveFirst(ctx, first); err != nil {
		t.Fatal(err)
	}

	got, err := repository.GetReports(ctx, models.GetReportsFilters{SlotID: &slotID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MessageID != first.MessageID || got[0].Text != first.Text || got[0].EditAt == nil || !got[0].EditAt.Equal(editedAt) {
		t.Fatalf("stored report = %+v, want edited first message %+v", got, first)
	}
}
