package domain

import (
	"sort"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

const maxScheduleOccurrences = 100

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
	cronSchedule, err := channelmodels.ParseCron(schedule.Cron)
	if err != nil {
		return nil
	}
	localNow := now.In(channelmodels.Moscow)
	start := localNow
	if watermark != nil && watermark.Before(now) {
		start = watermark.In(channelmodels.Moscow)
	} else if watermark == nil {
		start = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, channelmodels.Moscow).Add(-time.Minute)
	}
	cutoff := localNow.Add(-maxScheduleOccurrences * time.Minute)
	if start.Before(cutoff) {
		start = cutoff
	}
	regular := make([]time.Time, 0, maxScheduleOccurrences)
	for candidate := cronSchedule.Next(start); !candidate.IsZero() && !candidate.After(localNow); candidate = cronSchedule.Next(candidate) {
		regular = append(regular, candidate)
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
