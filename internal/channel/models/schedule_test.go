package models

import (
	"testing"
	"time"
)

func TestNextOccurrence(t *testing.T) {
	loc := Moscow
	tests := []struct {
		name     string
		now      time.Time
		cron     string
		override *time.Time
		want     time.Time
	}{
		{"week boundary", time.Date(2025, 1, 5, 12, 0, 0, 0, loc), "0 9 * * 1", nil, time.Date(2025, 1, 6, 9, 0, 0, 0, loc)},
		{"same-day past", time.Date(2025, 1, 6, 10, 0, 0, 0, loc), "0 9 * * 1", nil, time.Date(2025, 1, 13, 9, 0, 0, 0, loc)},
		{"UTC+3", time.Date(2025, 1, 5, 21, 30, 0, 0, time.UTC), "0 1 * * 1", nil, time.Date(2025, 1, 6, 1, 0, 0, 0, loc)},
		{"day and month fields", time.Date(2025, 1, 1, 0, 0, 0, 0, loc), "30 9 15 2 *", nil, time.Date(2025, 2, 15, 9, 30, 0, 0, loc)},
		{"override replaces nearest even if later", time.Date(2025, 1, 5, 12, 0, 0, 0, loc), "0 9 * * 1", ptr(time.Date(2025, 1, 7, 15, 0, 0, 0, loc)), time.Date(2025, 1, 7, 15, 0, 0, 0, loc)},
		{"invalid crossing override ignored", time.Date(2025, 1, 5, 12, 0, 0, 0, loc), "0 9 * * 1,3", ptr(time.Date(2025, 1, 8, 9, 0, 0, 0, loc)), time.Date(2025, 1, 6, 9, 0, 0, 0, loc)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextOccurrence(tt.now, tt.cron, tt.override)
			if got == nil || !got.Equal(tt.want) {
				t.Fatalf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestParseScheduleInputs(t *testing.T) {
	if schedule, err := ParseCron("30 9 * * 1,3,5"); err != nil || schedule == nil {
		t.Fatalf("ParseCron returned %v, %v", schedule, err)
	}
	for _, invalid := range []string{"", "0", "60 9 * * 1", "0 25 * * 1", "@daily"} {
		if _, err := ParseCron(invalid); err == nil {
			t.Errorf("ParseCron(%q) succeeded", invalid)
		}
	}
}

func TestValidateMove(t *testing.T) {
	loc := Moscow
	now := time.Date(2025, 1, 6, 8, 0, 0, 0, loc)
	schedule := Schedule{Cron: "0 9 * * 1,3"}
	tests := []struct {
		name     string
		now      time.Time
		schedule Schedule
		target   time.Time
		valid    bool
	}{
		{"earlier than first is allowed", now, schedule, time.Date(2025, 1, 6, 8, 30, 0, 0, loc), true},
		{"same day before now rejected", now, schedule, time.Date(2025, 1, 6, 7, 59, 0, 0, loc), false},
		{"same day future before first allowed", now, schedule, time.Date(2025, 1, 6, 8, 1, 0, 0, loc), true},
		{"second slot equality rejected", now, schedule, time.Date(2025, 1, 8, 9, 0, 0, 0, loc), false},
		{"after second rejected", now, schedule, time.Date(2025, 1, 9, 9, 0, 0, 0, loc), false},
		{"month rollover second equality rejected", time.Date(2025, 1, 31, 10, 0, 0, 0, loc), Schedule{Cron: "0 9 15 * *"}, time.Date(2025, 3, 15, 9, 0, 0, 0, loc), false},
		{"missing schedule rejected", now, Schedule{}, time.Date(2025, 1, 6, 8, 30, 0, 0, loc), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMove(tt.now, tt.schedule, tt.target)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateMove error = %v, want valid %v", err, tt.valid)
			}
		})
	}
}

func TestScheduleEditPreservesOnlyStillValidFutureOverride(t *testing.T) {
	now := time.Date(2025, 1, 6, 8, 0, 0, 0, Moscow)
	schedule := Schedule{Cron: "0 9 * * 1,3"}
	for _, tt := range []struct {
		name   string
		target time.Time
		keep   bool
	}{
		{"valid before second", time.Date(2025, 1, 7, 8, 0, 0, 0, Moscow), true},
		{"at second", time.Date(2025, 1, 8, 9, 0, 0, 0, Moscow), false},
		{"expired", time.Date(2025, 1, 6, 7, 0, 0, 0, Moscow), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := KeepOverrideAfterScheduleEdit(now, schedule, &tt.target); got != tt.keep {
				t.Fatalf("keep = %v, want %v", got, tt.keep)
			}
		})
	}
}

func TestNextOccurrenceWithoutSchedule(t *testing.T) {
	if got := NextOccurrence(time.Now(), "", nil); got != nil {
		t.Fatalf("unset schedule returned %v", got)
	}
}

func ptr(value time.Time) *time.Time { return &value }
