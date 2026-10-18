package slots

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

func TestDailyStartSlotsPostgres(t *testing.T) {
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
	schema := "daily_start_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
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
	if _, err := isolated.Exec(ctx, `CREATE TABLE publication_slots (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), channel_id BIGINT NOT NULL, scheduled_at TIMESTAMPTZ NOT NULL, state TEXT NOT NULL DEFAULT 'planned', channel_message_id BIGINT, linked_message_id BIGINT, lead_user_id BIGINT, published_at TIMESTAMPTZ, link_alerted_at TIMESTAMPTZ, publish_alerted_at TIMESTAMPTZ, UNIQUE (channel_id, scheduled_at))`); err != nil {
		t.Fatal(err)
	}
	moscow := time.FixedZone("UTC+3", 3*60*60)
	now := time.Now()
	insert := func(channelID int64, scheduledAt time.Time, state string) {
		t.Helper()
		if _, err := isolated.Exec(ctx, `INSERT INTO publication_slots (channel_id, scheduled_at, state) VALUES ($1, $2, $3)`, channelID, scheduledAt, state); err != nil {
			t.Fatal(err)
		}
	}
	dayStart := time.Date(2026, 10, 12, 0, 0, 0, 0, moscow)
	dayEnd := dayStart.AddDate(0, 0, 1)
	insert(-1, dayStart.Add(10*time.Hour), "published")
	insert(-1, dayStart.Add(-time.Minute), "published")
	insert(-1, dayEnd, "published")
	insert(-2, dayStart.Add(10*time.Hour), "published")
	insert(-1, dayStart.AddDate(0, 0, 2), "missed")
	insert(-3, now.Add(2*time.Hour), "planned")
	insert(-4, now.Add(-2*time.Hour), "planned")

	repository := New(postgres.NewGetter(isolated))
	channelID := int64(-1)
	got, err := repository.GetSlots(ctx, models.GetSlotsFilters{ChannelID: &channelID, State: models.ClaimedStates, ScheduledFrom: &dayStart, ScheduledTo: &dayEnd})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].ScheduledAt.Equal(dayStart.Add(10*time.Hour)) {
		t.Fatalf("slots of the day = %+v", got)
	}

	recovered, err := repository.Recover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].ChannelID != -4 {
		t.Fatalf("recovered = %+v; want only the planned slot whose daily start passed", recovered)
	}
	future, err := repository.GetSlots(ctx, models.GetSlotsFilters{State: []models.State{models.StatePlanned}})
	if err != nil || len(future) != 1 || future[0].ChannelID != -3 {
		t.Fatalf("planned slots after recovery = %+v, %v", future, err)
	}

	created, err := repository.Create(ctx, -5, dayStart.Add(10*time.Hour))
	if err != nil || created.PublishAlertedAt != nil {
		t.Fatalf("Create() = %+v, %v", created, err)
	}
	if err := repository.MarkPublishAlerted(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkPublishAlerted(ctx, created.ID); err == nil {
		t.Fatal("second failure alert mark succeeded")
	}
	replayed, err := repository.Create(ctx, -5, dayStart.Add(10*time.Hour))
	if err != nil || replayed.PublishAlertedAt == nil {
		t.Fatalf("replayed Create() = %+v, %v", replayed, err)
	}
}
