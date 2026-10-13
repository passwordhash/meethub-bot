package domain

import (
	"testing"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

func TestScheduleOccurrences(t *testing.T) {
	moscow := channelmodels.Moscow
	at := func(day, hour, minute, second int) time.Time {
		return time.Date(2026, 9, day, hour, minute, second, 0, moscow)
	}
	grid := map[time.Weekday]time.Duration{
		time.Monday:    10 * time.Hour,
		time.Wednesday: 12*time.Hour + 30*time.Minute,
		time.Friday:    10 * time.Hour,
	}
	midnightGrid := map[time.Weekday]time.Duration{time.Tuesday: 30 * time.Minute}
	longAgo := at(28, 10, 0, 0).AddDate(0, 0, -60)
	justBefore := at(28, 8, 59, 50)
	tests := []struct {
		name      string
		schedule  channelmodels.Schedule
		watermark *time.Time
		now       time.Time
		want      []ScheduleOccurrence
	}{
		{name: "before post time nothing", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: &justBefore, now: at(28, 8, 59, 55)},
		{name: "post time reached", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: &justBefore, now: at(28, 9, 0, 5), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "late post still due before daily starts", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: &justBefore, now: at(28, 9, 59, 59), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "missed once daily started", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: &justBefore, now: at(28, 10, 0, 0), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Missed: true}}},
		{name: "zero offset keeps minute window", schedule: channelmodels.Schedule{DailyTimes: grid}, watermark: &justBefore, now: at(28, 10, 0, 59), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "zero offset missed after minute", schedule: channelmodels.Schedule{DailyTimes: grid}, watermark: &justBefore, now: at(28, 10, 1, 0), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Missed: true}}},
		{name: "one minute offset window ends at daily start", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 1}, watermark: &justBefore, now: at(28, 10, 0, 0), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Missed: true}}},
		{name: "post the evening before crosses midnight", schedule: channelmodels.Schedule{DailyTimes: midnightGrid, PostOffsetMinutes: 90}, watermark: ptr(at(28, 22, 59, 50)), now: at(28, 23, 0, 5), want: []ScheduleOccurrence{{StartsAt: at(29, 0, 30, 0), Due: true}}},
		{name: "maximum offset posts a day ahead", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 1439}, watermark: ptr(at(27, 10, 59, 50)), now: at(27, 11, 1, 0), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "maximum offset not yet reached", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 1439}, watermark: ptr(at(27, 10, 0, 0)), now: at(27, 10, 0, 30)},
		{name: "downtime reports passed and upcoming", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: ptr(at(23, 20, 0, 0)), now: at(28, 9, 30, 0), want: []ScheduleOccurrence{{StartsAt: at(25, 10, 0, 0), Missed: true}, {StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "ancient watermark is bounded to a week", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: &longAgo, now: at(28, 9, 30, 0), want: []ScheduleOccurrence{{StartsAt: at(21, 10, 0, 0), Missed: true}, {StartsAt: at(23, 12, 30, 0), Missed: true}, {StartsAt: at(25, 10, 0, 0), Missed: true}, {StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "no watermark starts from today's midnight", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: at(28, 9, 30, 0), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "watermark before start keeps the occurrence after an edit at start", schedule: channelmodels.Schedule{DailyTimes: grid}, watermark: ptr(at(28, 9, 59, 55)), now: at(28, 10, 0, 10), want: []ScheduleOccurrence{{StartsAt: at(28, 10, 0, 0), Due: true}}},
		{name: "daily set after its start gets no retroactive missed", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: ptr(at(28, 10, 29, 50)), now: at(28, 10, 30, 5)},
		{name: "empty grid", schedule: channelmodels.Schedule{PostOffsetMinutes: 60}, now: at(28, 9, 30, 0)},
		{name: "occurrence before watermark skipped", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, watermark: ptr(at(28, 10, 0, 30)), now: at(28, 10, 0, 45)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScheduleOccurrences(tt.schedule, tt.watermark, tt.now)
			if len(got) != len(tt.want) {
				t.Fatalf("got %+v; want %+v", got, tt.want)
			}
			for index := range got {
				if !got[index].StartsAt.Equal(tt.want[index].StartsAt) || got[index].Due != tt.want[index].Due || got[index].Missed != tt.want[index].Missed {
					t.Fatalf("occurrence %d = %+v; want %+v", index, got[index], tt.want[index])
				}
			}
		})
	}
}

func ptr(value time.Time) *time.Time { return &value }
