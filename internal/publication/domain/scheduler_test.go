package domain

import (
	"testing"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

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
