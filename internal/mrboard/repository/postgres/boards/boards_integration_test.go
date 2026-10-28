package boards

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

func TestBoardsPostgres(t *testing.T) {
	ctx := context.Background()
	db := isolatedDatabase(t)
	applyMigrations(t, ctx, db, 1, 12)
	if _, err := db.Exec(ctx, `INSERT INTO channels (channel_id, linked_chat_id) VALUES (-1, -11), (-2, -12)`); err != nil {
		t.Fatal(err)
	}
	repository := New(postgres.NewGetter(db))
	at := time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC)
	for _, board := range []models.Board{{ChannelID: -1, MessageID: 5, Body: "old", UpdatedAt: at}, {ChannelID: -1, MessageID: 6, Body: "new", UpdatedAt: at.Add(time.Minute)}, {ChannelID: -2, MessageID: 9, Body: "other", UpdatedAt: at}} {
		if err := repository.SaveBoard(ctx, board); err != nil {
			t.Fatalf("SaveBoard(%+v) = %v", board, err)
		}
	}
	channelID := int64(-1)
	boards, err := repository.GetBoards(ctx, models.GetBoardsFilters{ChannelID: &channelID})
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].MessageID != 6 || boards[0].Body != "new" || !boards[0].UpdatedAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("boards = %+v", boards)
	}
	if err := repository.SaveBoard(ctx, models.Board{ChannelID: -99, MessageID: 1, UpdatedAt: at}); err == nil {
		t.Fatal("board of an unknown channel was stored")
	}
}

func TestTracksPostgres(t *testing.T) {
	ctx := context.Background()
	db := isolatedDatabase(t)
	applyMigrations(t, ctx, db, 1, 12)
	repository := New(postgres.NewGetter(db))
	at := time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC)
	first := []models.ReviewerTrack{
		{ProjectID: 1, IID: 7, Username: "a", State: models.StateReviewed, ReviewedOnce: true, ChangedAt: at},
		{ProjectID: 1, IID: 8, Username: "b", State: models.StateUnreviewed, ChangedAt: at},
	}
	firstSections := []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: at}, {ProjectID: 1, IID: 8, Section: models.SectionAwaitingReview, Since: at}}
	if err := repository.SaveTracks(ctx, first, firstSections); err != nil {
		t.Fatal(err)
	}
	second := []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateUnreviewed, ReviewedOnce: true, ChangedAt: at.Add(time.Hour)}}
	secondSections := []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: at}}
	if err := repository.SaveTracks(ctx, second, secondSections); err != nil {
		t.Fatal(err)
	}
	reviewers, sections, err := repository.GetTracks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for index := range reviewers {
		reviewers[index].ChangedAt = reviewers[index].ChangedAt.UTC()
	}
	for index := range sections {
		sections[index].Since = sections[index].Since.UTC()
	}
	if !reflect.DeepEqual(reviewers, second) || !reflect.DeepEqual(sections, secondSections) {
		t.Fatalf("tracks = %+v, %+v; want %+v, %+v", reviewers, sections, second, secondSections)
	}
	if err := repository.SaveTracks(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}
	if reviewers, sections, err = repository.GetTracks(ctx); err != nil || len(reviewers) != 0 || len(sections) != 0 {
		t.Fatalf("tracks after merge = %+v, %+v, %v", reviewers, sections, err)
	}
}

func isolatedDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connectionString := os.Getenv("REPORT_TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("REPORT_TEST_DATABASE_URL is unset; disposable PostgreSQL integration test not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := "boards_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(isolated.Close)
	return isolated
}

func applyMigrations(t *testing.T, ctx context.Context, db *pgxpool.Pool, from, to int) {
	t.Helper()
	files, err := filepath.Glob("../../../../../migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations not found: %v", err)
	}
	for _, file := range files {
		var version int
		if _, err := fmt.Sscanf(filepath.Base(file), "%d_", &version); err != nil || version < from || version > to {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(content), "-- +goose Down")
		if _, err := db.Exec(ctx, up); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
}
