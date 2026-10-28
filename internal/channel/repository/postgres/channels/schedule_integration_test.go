package channels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/postgres"
)

func TestScheduleGridPostgres(t *testing.T) {
	ctx := context.Background()
	db := isolatedDatabase(t)
	applyMigrations(t, ctx, db, 1, 9)
	legacyCrons := map[int64]string{
		-1: "30 9 * * 1,3,5",
		-2: "15 11 * * 1-5",
		-3: "0 8 * * *",
		-4: "0 9 * * 7",
		-5: "45 9 * * 1-3,5,0",
		-6: "15 10 1 * *",
		-7: "*/5 * * * *",
		-8: "0 9 * * mon",
		-9: "0 9 * * 5-1",
	}
	for channelID, cron := range legacyCrons {
		if _, err := db.Exec(ctx, `INSERT INTO channels (channel_id, linked_chat_id, schedule_cron) VALUES ($1, $2, $3)`, channelID, channelID-1000, cron); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO channels (channel_id, linked_chat_id) VALUES (-10, -1010)`); err != nil {
		t.Fatal(err)
	}
	applyMigrations(t, ctx, db, 10, 10)
	repository := New(postgres.NewGetter(db))

	ten := 10 * time.Hour
	tests := []struct {
		name         string
		channelID    int64
		wantTimes    map[time.Weekday]time.Duration
		wantOffset   int
		wantResetted bool
	}{
		{name: "list converts to ten o'clock with offset", channelID: -1, wantTimes: map[time.Weekday]time.Duration{time.Monday: ten, time.Wednesday: ten, time.Friday: ten}, wantOffset: 30},
		{name: "cron later than ten keeps its time with zero offset", channelID: -2, wantTimes: map[time.Weekday]time.Duration{time.Monday: 11*time.Hour + 15*time.Minute, time.Tuesday: 11*time.Hour + 15*time.Minute, time.Wednesday: 11*time.Hour + 15*time.Minute, time.Thursday: 11*time.Hour + 15*time.Minute, time.Friday: 11*time.Hour + 15*time.Minute}, wantOffset: 0},
		{name: "every day with two hour offset", channelID: -3, wantTimes: map[time.Weekday]time.Duration{time.Sunday: ten, time.Monday: ten, time.Tuesday: ten, time.Wednesday: ten, time.Thursday: ten, time.Friday: ten, time.Saturday: ten}, wantOffset: 120},
		{name: "seven means Sunday", channelID: -4, wantTimes: map[time.Weekday]time.Duration{time.Sunday: ten}, wantOffset: 60},
		{name: "range list and zero combined", channelID: -5, wantTimes: map[time.Weekday]time.Duration{time.Sunday: ten, time.Monday: ten, time.Tuesday: ten, time.Wednesday: ten, time.Friday: ten}, wantOffset: 15},
		{name: "day of month cron is reset", channelID: -6, wantTimes: map[time.Weekday]time.Duration{}, wantOffset: 60, wantResetted: true},
		{name: "step cron is reset", channelID: -7, wantTimes: map[time.Weekday]time.Duration{}, wantOffset: 60, wantResetted: true},
		{name: "named weekday is reset", channelID: -8, wantTimes: map[time.Weekday]time.Duration{}, wantOffset: 60, wantResetted: true},
		{name: "descending range is reset", channelID: -9, wantTimes: map[time.Weekday]time.Duration{}, wantOffset: 60, wantResetted: true},
		{name: "unconfigured channel stays empty without notice", channelID: -10, wantTimes: map[time.Weekday]time.Duration{}, wantOffset: 60},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedules, err := repository.GetSchedules(ctx, models.GetSchedulesFilters{ChannelID: &tt.channelID})
			if err != nil {
				t.Fatal(err)
			}
			if len(schedules) != 1 {
				t.Fatalf("got %d schedules", len(schedules))
			}
			got := schedules[0]
			if !reflect.DeepEqual(got.DailyTimes, tt.wantTimes) || got.PostOffsetMinutes != tt.wantOffset || got.ResetPending != tt.wantResetted {
				t.Fatalf("schedule = %+v; want times %v offset %d reset %t", got, tt.wantTimes, tt.wantOffset, tt.wantResetted)
			}
		})
	}
}

func TestScheduleEditsPostgres(t *testing.T) {
	ctx := context.Background()
	db := isolatedDatabase(t)
	applyMigrations(t, ctx, db, 1, 10)
	if _, err := db.Exec(ctx, `INSERT INTO channels (channel_id, linked_chat_id, schedule_reset_pending) VALUES (-1, -11, TRUE), (-2, -12, FALSE)`); err != nil {
		t.Fatal(err)
	}
	repository := New(postgres.NewGetter(db))
	channelID := int64(-1)
	load := func() models.Schedule {
		schedules, err := repository.GetSchedules(ctx, models.GetSchedulesFilters{ChannelID: &channelID})
		if err != nil || len(schedules) != 1 {
			t.Fatalf("GetSchedules() = %v, %v", schedules, err)
		}
		return schedules[0]
	}

	if err := repository.SetDailyTime(ctx, channelID, time.Wednesday, 12*time.Hour+30*time.Minute); err != nil {
		t.Fatal(err)
	}
	firstWatermark := load().CheckedAt
	if firstWatermark == nil {
		t.Fatal("first daily time edit did not initialise the schedule watermark")
	}
	tickAt := firstWatermark.Add(time.Hour).Truncate(time.Microsecond)
	if err := repository.SetScheduleCheckedAt(ctx, channelID, tickAt); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDailyTime(ctx, channelID, time.Sunday, 0); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDailyTime(ctx, channelID, time.Wednesday, 9*time.Hour+5*time.Minute); err != nil {
		t.Fatal(err)
	}
	schedule := load()
	wantTimes := map[time.Weekday]time.Duration{time.Wednesday: 9*time.Hour + 5*time.Minute, time.Sunday: 0}
	if !reflect.DeepEqual(schedule.DailyTimes, wantTimes) {
		t.Fatalf("daily times = %v; want %v", schedule.DailyTimes, wantTimes)
	}
	if schedule.CheckedAt == nil || !schedule.CheckedAt.Equal(tickAt) {
		t.Fatalf("daily time edit moved the schedule watermark to %v; want %v", schedule.CheckedAt, tickAt)
	}

	if err := repository.DeleteDailyTime(ctx, channelID, time.Wednesday); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteDailyTime(ctx, channelID, time.Thursday); err != nil {
		t.Fatalf("deleting an absent day: %v", err)
	}
	if got := load().DailyTimes; !reflect.DeepEqual(got, map[time.Weekday]time.Duration{time.Sunday: 0}) {
		t.Fatalf("daily times after delete = %v", got)
	}

	if err := repository.SetPostOffset(ctx, channelID, 1439); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetLeadEnabled(ctx, channelID, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.ClearScheduleReset(ctx, channelID); err != nil {
		t.Fatal(err)
	}
	if schedule = load(); schedule.PostOffsetMinutes != 1439 || schedule.LeadEnabled || schedule.ResetPending {
		t.Fatalf("schedule = %+v", schedule)
	}
	if schedule.CheckedAt == nil || !schedule.CheckedAt.Equal(tickAt) {
		t.Fatalf("delete, offset or lead edit moved the schedule watermark to %v; want %v", schedule.CheckedAt, tickAt)
	}
	if err := repository.SetPostOffset(ctx, channelID, 1440); err == nil {
		t.Fatal("offset beyond the day was stored")
	}

	missing := int64(-99)
	for name, err := range map[string]error{
		"daily time": repository.SetDailyTime(ctx, missing, time.Monday, time.Hour),
		"offset":     repository.SetPostOffset(ctx, missing, 30),
		"lead":       repository.SetLeadEnabled(ctx, missing, true),
		"reset":      repository.ClearScheduleReset(ctx, missing),
	} {
		if !errors.Is(err, models.ErrInvalidChannelID) {
			t.Errorf("%s for an unknown channel = %v", name, err)
		}
	}
}

func TestIgnoredUsersPostgres(t *testing.T) {
	ctx := context.Background()
	db := isolatedDatabase(t)
	applyMigrations(t, ctx, db, 1, 11)
	if _, err := db.Exec(ctx, `INSERT INTO channels (channel_id, linked_chat_id) VALUES (-1, -11), (-2, -12)`); err != nil {
		t.Fatal(err)
	}
	repository := New(postgres.NewGetter(db))
	channelID, otherID := int64(-1), int64(-2)
	for _, entry := range []struct{ channelID, userID int64 }{{channelID, 42}, {channelID, 7}, {channelID, 42}, {otherID, 42}} {
		if err := repository.AddIgnoredUser(ctx, entry.channelID, entry.userID); err != nil {
			t.Fatalf("AddIgnoredUser(%d, %d) = %v", entry.channelID, entry.userID, err)
		}
	}
	users, err := repository.GetIgnoredUsers(ctx, models.GetIgnoredUsersFilters{ChannelID: &channelID})
	if err != nil {
		t.Fatal(err)
	}
	if want := []models.IgnoredUser{{ChannelID: channelID, UserID: 42}, {ChannelID: channelID, UserID: 7}}; !reflect.DeepEqual(users, want) {
		t.Fatalf("ignored users = %+v; want %+v", users, want)
	}
	userID := int64(7)
	if err := repository.DeleteIgnoredUser(ctx, channelID, userID); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteIgnoredUser(ctx, channelID, userID); err != nil {
		t.Fatalf("deleting an absent user: %v", err)
	}
	users, err = repository.GetIgnoredUsers(ctx, models.GetIgnoredUsersFilters{ChannelID: &channelID, UserID: &userID})
	if err != nil || len(users) != 0 {
		t.Fatalf("deleted user lookup = %+v, %v", users, err)
	}
	ignored := int64(42)
	if users, err = repository.GetIgnoredUsers(ctx, models.GetIgnoredUsersFilters{UserID: &ignored}); err != nil || len(users) != 2 {
		t.Fatalf("other channel list was affected: %+v, %v", users, err)
	}
	if err := repository.AddIgnoredUser(ctx, -99, 42); !errors.Is(err, models.ErrInvalidChannelID) {
		t.Fatalf("ignoring a user of an unknown channel = %v", err)
	}
	if err := repository.AddIgnoredUser(ctx, channelID, 0); err == nil {
		t.Fatal("non-positive user id was stored")
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
	schema := "channels_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
