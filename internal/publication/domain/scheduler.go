package domain

import (
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

const maxScheduleLookback = 7 * 24 * time.Hour

type ScheduleOccurrence struct {
	StartsAt time.Time
	Due      bool
	Missed   bool
}

func ScheduleOccurrences(schedule channelmodels.Schedule, watermark *time.Time, now time.Time) []ScheduleOccurrence {
	localNow := now.In(channelmodels.Moscow)
	after := occurrencesAfter(watermark, localNow)
	last := localNow.Add(schedule.PostOffset())
	occurrences := make([]ScheduleOccurrence, 0, len(schedule.DailyTimes))
	firstDay, _ := channelmodels.MoscowDay(after)
	for day := firstDay; !day.After(last); day = day.AddDate(0, 0, 1) {
		dailyTime, scheduled := schedule.DailyTimes[day.Weekday()]
		if !scheduled {
			continue
		}
		startsAt := day.Add(dailyTime)
		if !startsAt.After(after) || startsAt.After(last) {
			continue
		}
		occurrences = append(occurrences, classify(startsAt, schedule.PostOffset(), localNow))
	}
	return occurrences
}

func occurrencesAfter(watermark *time.Time, localNow time.Time) time.Time {
	dayStart, _ := channelmodels.MoscowDay(localNow)
	after := dayStart.Add(-time.Minute)
	if watermark != nil {
		after = watermark.In(channelmodels.Moscow)
		if after.After(localNow) {
			after = localNow
		}
	}
	if cutoff := localNow.Add(-maxScheduleLookback); after.Before(cutoff) {
		after = cutoff
	}
	return after
}

func classify(startsAt time.Time, offset time.Duration, localNow time.Time) ScheduleOccurrence {
	deadline := startsAt
	if minuteWindow := startsAt.Add(-offset).Add(time.Minute); minuteWindow.After(deadline) {
		deadline = minuteWindow
	}
	missed := !localNow.Before(deadline)
	return ScheduleOccurrence{StartsAt: startsAt, Due: !missed, Missed: missed}
}
