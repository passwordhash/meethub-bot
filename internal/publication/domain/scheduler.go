package domain

import (
	"sort"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

type ScheduleOccurrence struct {
	ScheduledAt time.Time
	Due         bool
	Missed      bool
}

func EvaluateSchedule(schedule channelmodels.Schedule, watermark *time.Time, now time.Time) ScheduleOccurrence {
	occurrences := ScheduleOccurrences(schedule, watermark, now)
	for _, occurrence := range occurrences {
		if occurrence.Due {
			return occurrence
		}
	}
	if len(occurrences) > 0 {
		return occurrences[len(occurrences)-1]
	}
	return ScheduleOccurrence{}
}

func ScheduleOccurrences(schedule channelmodels.Schedule, watermark *time.Time, now time.Time) []ScheduleOccurrence {
	localNow := now.In(channelmodels.Moscow)
	start := localNow
	if watermark != nil && watermark.Before(now) {
		start = watermark.In(channelmodels.Moscow)
	}
	regular := make([]time.Time, 0)
	for day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, channelmodels.Moscow); !day.After(localNow); day = day.AddDate(0, 0, 1) {
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		for _, scheduledDay := range schedule.Weekdays {
			if weekday != scheduledDay {
				continue
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), schedule.Hour, schedule.Minute, 0, 0, channelmodels.Moscow)
			if (watermark == nil || candidate.After(watermark.In(channelmodels.Moscow))) && !candidate.After(localNow) {
				regular = append(regular, candidate)
			}
			break
		}
	}
	if schedule.Override != nil && (watermark == nil || schedule.Override.After(*watermark)) {
		if len(regular) > 0 {
			regular[0] = schedule.Override.In(channelmodels.Moscow)
		} else {
			regular = append(regular, schedule.Override.In(channelmodels.Moscow))
		}
	}
	sort.Slice(regular, func(i, j int) bool { return regular[i].Before(regular[j]) })
	occurrences := make([]ScheduleOccurrence, 0, len(regular))
	for _, candidate := range regular {
		if candidate.After(localNow) {
			continue
		}
		occurrence := ScheduleOccurrence{ScheduledAt: candidate, Missed: true}
		if localNow.Sub(candidate) < time.Minute {
			occurrence.Due = true
			occurrence.Missed = false
		}
		occurrences = append(occurrences, occurrence)
	}
	return occurrences
}
