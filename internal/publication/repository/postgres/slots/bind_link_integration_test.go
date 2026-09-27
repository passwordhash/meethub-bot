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
)

func TestBindLinkPostgres(t *testing.T) {
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
	schema := "bind_link_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	_, err = isolated.Exec(ctx, `CREATE TABLE channels (channel_id BIGINT PRIMARY KEY, linked_chat_id BIGINT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = isolated.Exec(ctx, `CREATE TABLE publication_slots (id UUID PRIMARY KEY, channel_id BIGINT NOT NULL, scheduled_at TIMESTAMPTZ NOT NULL, state TEXT NOT NULL, channel_message_id BIGINT, linked_message_id BIGINT, lead_user_id BIGINT, published_at TIMESTAMPTZ)`)
	if err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	slotID := uuid.New()
	if _, err := isolated.Exec(ctx, `INSERT INTO channels VALUES (-1001, -2002), (-1003, -2004)`); err != nil {
		t.Fatal(err)
	}
	if _, err := isolated.Exec(ctx, `INSERT INTO publication_slots (id, channel_id, scheduled_at, state, channel_message_id, published_at) VALUES ($1, -1001, $2, 'published', 31, $3)`, slotID, publishedAt, publishedAt); err != nil {
		t.Fatal(err)
	}
	repository := New(postgres.NewGetter(isolated))
	tests := []struct {
		name                                       string
		channelID, groupID, postID, groupMessageID int64
		wantSuccess                                bool
	}{
		{name: "valid binding", channelID: -1001, groupID: -2002, postID: 31, groupMessageID: 41, wantSuccess: true},
		{name: "same binding is idempotent", channelID: -1001, groupID: -2002, postID: 31, groupMessageID: 41, wantSuccess: true},
		{name: "cross channel", channelID: -1003, groupID: -2004, postID: 31, groupMessageID: 41},
		{name: "cross group", channelID: -1001, groupID: -2004, postID: 31, groupMessageID: 41},
		{name: "cross post", channelID: -1001, groupID: -2002, postID: 32, groupMessageID: 41},
		{name: "different linked forward", channelID: -1001, groupID: -2002, postID: 31, groupMessageID: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.BindLink(ctx, slotID, tt.channelID, tt.groupID, tt.postID, tt.groupMessageID)
			if tt.wantSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if got.PublishedAt == nil || !got.PublishedAt.Equal(publishedAt) {
					t.Fatalf("published_at = %v, want %v", got.PublishedAt, publishedAt)
				}
				return
			}
			if err == nil {
				t.Fatal("expected binding mismatch")
			}
		})
	}
}
