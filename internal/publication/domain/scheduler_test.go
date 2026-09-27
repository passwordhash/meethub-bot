package domain

import (
	"testing"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

func TestScheduleOccurrences(t *testing.T) {
	midnight := time.Date(2026, 9, 28, 0, 0, 0, 0, channelmodels.Moscow)
	now := time.Date(2026, 9, 28, 10, 0, 20, 0, channelmodels.Moscow)
	oldWatermark := now.AddDate(0, 0, -30)
	tests := []struct {
		name        string
		schedule    channelmodels.Schedule
		watermark   *time.Time
		now         time.Time
		wantCount   int
		wantLast    time.Time
		wantLastDue bool
	}{
		{name: "include midnight occurrence without watermark", schedule: channelmodels.Schedule{Cron: "0 0 * * *"}, now: midnight.Add(20 * time.Second), wantCount: 1, wantLast: midnight, wantLastDue: true},
		{name: "bound dense cron backlog", schedule: channelmodels.Schedule{Cron: "* * * * *"}, watermark: &oldWatermark, now: now, wantCount: maxScheduleOccurrences, wantLast: time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow), wantLastDue: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScheduleOccurrences(tt.schedule, tt.watermark, tt.now)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d occurrences; want %d", len(got), tt.wantCount)
			}
			last := got[len(got)-1]
			if !last.ScheduledAt.Equal(tt.wantLast) || last.Due != tt.wantLastDue {
				t.Fatalf("last occurrence = %+v", last)
			}
		})
	}
}

func TestEvaluateSchedule(t *testing.T) {
	monday := time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow)
	threeWeeksAgo := monday.AddDate(0, 0, -21)
	yesterday := monday.Add(-24 * time.Hour)
	tests := []struct {
		name       string
		schedule   channelmodels.Schedule
		watermark  *time.Time
		now        time.Time
		wantDue    bool
		wantMissed bool
	}{
		{name: "due despite multiple missed occurrences", schedule: channelmodels.Schedule{Cron: "0 10 * * 1"}, watermark: &threeWeeksAgo, now: monday.Add(20 * time.Second), wantDue: true},
		{name: "current minute due after downtime", schedule: channelmodels.Schedule{Cron: "0 10 * * 1"}, watermark: &yesterday, now: monday.Add(20 * time.Second), wantDue: true},
		{name: "expired occurrence missed", schedule: channelmodels.Schedule{Cron: "0 10 * * 1"}, watermark: &yesterday, now: monday.Add(time.Minute), wantMissed: true},
		{name: "future occurrence ignored", schedule: channelmodels.Schedule{Cron: "0 10 * * 1"}, now: monday.Add(-time.Minute)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateSchedule(tt.schedule, tt.watermark, tt.now)
			if got.Due != tt.wantDue || got.Missed != tt.wantMissed {
				t.Fatalf("EvaluateSchedule() = %+v", got)
			}
		})
	}
}
