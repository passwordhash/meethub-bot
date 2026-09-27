package summary

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

func TestSummaryPartsPostgres(t *testing.T) {
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
	schema := "summary_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(ctx, `CREATE TABLE publication_slots (id UUID PRIMARY KEY, channel_id BIGINT NOT NULL); CREATE TABLE publication_summary_parts (edition_slot_id UUID NOT NULL REFERENCES publication_slots(id), part_index INTEGER NOT NULL CHECK (part_index >= 0), text TEXT CHECK (text IS NULL OR length(text) > 0), status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','attempting','unknown','sent')), group_message_id BIGINT, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (edition_slot_id, part_index), CHECK ((status = 'sent') = (group_message_id IS NOT NULL)), CHECK (status <> 'sent' OR group_message_id > 0))`)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO publication_slots VALUES ($1, -1001)`, id); err != nil {
		t.Fatal(err)
	}
	repo := New(postgres.NewGetter(db))
	texts := []string{"chunk one", "chunk two"}
	if err := repo.CreateSummaryParts(ctx, id, texts); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSummaryParts(ctx, id, texts); err != nil {
		t.Fatalf("idempotent create: %v", err)
	}
	other := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO publication_slots VALUES ($1, -1002)`, other); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSummaryParts(ctx, other, []string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSummaryParts(ctx, other, []string{"first", "different"}); err == nil || strings.Contains(strings.ToLower(err.Error()), "constraint") || strings.Contains(err.Error(), "different") {
		t.Fatal("mismatched batch did not return a safe generic conflict")
	}
	if err := repo.CreateSummaryParts(ctx, other, []string{"first", "different", "third"}); err == nil || strings.Contains(strings.ToLower(err.Error()), "constraint") || strings.Contains(err.Error(), "different") {
		t.Fatal("mismatched batch with a new trailing part did not return a safe generic conflict")
	}
	var unchanged int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM publication_summary_parts WHERE edition_slot_id = $1`, other).Scan(&unchanged); err != nil {
		t.Fatal(err)
	}
	if unchanged != 2 {
		t.Fatalf("failed batch changed row count: %d", unchanged)
	}
	unchangedParts, err := repo.GetSummaryParts(ctx, models.GetSummaryPartsFilters{EditionSlotID: &other})
	if err != nil {
		t.Fatal(err)
	}
	if len(unchangedParts) != 2 || unchangedParts[0].Text != "first" || unchangedParts[1].Text != "second" || unchangedParts[0].Status != "pending" || unchangedParts[1].Status != "pending" {
		t.Fatal("failed batch modified existing content or state")
	}
	parts, err := repo.GetSummaryParts(ctx, models.GetSummaryPartsFilters{EditionSlotID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0].PartIndex != 0 || parts[1].Text != texts[1] {
		t.Fatalf("unexpected ordered parts: %+v", parts)
	}
	if err := repo.Begin(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	channelIDs, err := repo.ListAttemptingChannels(ctx)
	if err != nil || len(channelIDs) != 1 || channelIDs[0] != -1001 {
		t.Fatalf("unexpected attempting channel IDs: %v, %v", channelIDs, err)
	}
	if err := repo.RecoverAttempting(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.Sent(ctx, id, 0, 25); err == nil {
		t.Fatal("sent after recovery should conflict")
	}
	if err := repo.Begin(ctx, id, 0); err == nil {
		t.Fatal("unknown must not retry")
	}
	if err := repo.Begin(ctx, id, 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Sent(ctx, id, 1, 26); err != nil {
		t.Fatal(err)
	}
	if err := repo.Sent(ctx, id, 1, 27); err == nil {
		t.Fatal("sent content was rewritten")
	}
	if _, err := db.Exec(ctx, `UPDATE publication_summary_parts SET created_at = now() - interval '31 days' WHERE edition_slot_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var messageID int64
	if err := db.QueryRow(ctx, `SELECT status, group_message_id FROM publication_summary_parts WHERE edition_slot_id = $1 AND part_index = 1`, id).Scan(&status, &messageID); err != nil {
		t.Fatal(err)
	}
	if status != "sent" || messageID != 26 {
		t.Fatalf("technical IDs not retained: %s %d", status, messageID)
	}
	if _, err := repo.GetSummaryParts(ctx, models.GetSummaryPartsFilters{EditionSlotID: &id}); err == nil {
		t.Fatal("expired parts remained readable")
	}
	if err := repo.CreateSummaryParts(ctx, id, texts); err == nil {
		t.Fatal("purged content was re-created")
	}
}
