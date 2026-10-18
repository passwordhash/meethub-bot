package models

import (
	"time"
)

const MaxPostOffsetMinutes = 1439

var Moscow = time.FixedZone("UTC+3", 3*60*60)

var shortWeekdays = [...]string{"вс", "пн", "вт", "ср", "чт", "пт", "сб"}

func ShortWeekday(weekday time.Weekday) string {
	return shortWeekdays[weekday]
}

func ParseStartsAt(text string) (time.Duration, error) {
	if len(text) != len("15:04") {
		return 0, ErrInvalidDailyTime
	}
	parsed, err := time.Parse("15:04", text)
	if err != nil {
		return 0, ErrInvalidDailyTime
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute, nil
}

func FormatStartsAt(startsAt time.Duration) string {
	return time.Time{}.Add(startsAt).Format("15:04")
}

func ValidatePostOffset(minutes int) error {
	if minutes < 0 || minutes > MaxPostOffsetMinutes {
		return ErrInvalidPostOffset
	}
	return nil
}

func (s Schedule) PostOffset() time.Duration {
	return time.Duration(s.PostOffsetMinutes) * time.Minute
}

func MoscowDay(moment time.Time) (time.Time, time.Time) {
	local := moment.In(Moscow)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Moscow)
	return start, start.AddDate(0, 0, 1)
}

func NextDateOf(now time.Time, weekday time.Weekday) time.Time {
	today, _ := MoscowDay(now)
	return today.AddDate(0, 0, (int(weekday)-int(today.Weekday())+7)%7)
}
