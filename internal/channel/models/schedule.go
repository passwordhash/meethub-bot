package models

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var ErrInvalidSchedule = errors.New("invalid schedule")
var ErrInvalidMove = errors.New("invalid move")

var Moscow = time.FixedZone("UTC+3", 3*60*60)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func ParseCron(expression string) (*cron.SpecSchedule, error) {
	parsed, err := cronParser.Parse(strings.TrimSpace(expression))
	if err != nil {
		return nil, ErrInvalidSchedule
	}
	schedule, ok := parsed.(*cron.SpecSchedule)
	if !ok {
		return nil, ErrInvalidSchedule
	}
	schedule.Location = Moscow
	return schedule, nil
}

func ParseWeekdays(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	seen := make(map[int]bool, len(parts))
	weekdays := make([]int, 0, len(parts))
	for _, part := range parts {
		day, err := strconv.Atoi(part)
		if err != nil {
			return nil, ErrInvalidSchedule
		}
		if day < 1 || day > 7 || seen[day] {
			return nil, ErrInvalidSchedule
		}
		seen[day] = true
		weekdays = append(weekdays, day)
	}
	return weekdays, nil
}

func ParseClock(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, 0, ErrInvalidSchedule
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func CronFromLegacy(weekdays []int, hour, minute int) string {
	days := make([]string, 0, len(weekdays))
	for _, day := range weekdays {
		if day == 7 {
			day = 0
		}
		days = append(days, strconv.Itoa(day))
	}
	return strings.Join([]string{strconv.Itoa(minute), strconv.Itoa(hour), "*", "*", strings.Join(days, ",")}, " ")
}

func NextOccurrence(now time.Time, expression string, override *time.Time) *time.Time {
	first, second := RegularOccurrences(now, expression)
	if first == nil {
		return nil
	}
	if override != nil && override.After(now) && second != nil && override.Before(*second) {
		target := override.In(Moscow)
		return &target
	}
	return first
}

func RegularOccurrences(now time.Time, expression string) (*time.Time, *time.Time) {
	if strings.TrimSpace(expression) == "" {
		return nil, nil
	}
	schedule, err := ParseCron(expression)
	if err != nil {
		return nil, nil
	}
	first := schedule.Next(now.In(Moscow))
	if first.IsZero() {
		return nil, nil
	}
	second := schedule.Next(first)
	if second.IsZero() {
		return &first, nil
	}
	return &first, &second
}

func KeepOverrideAfterScheduleEdit(now time.Time, schedule Schedule, override *time.Time) bool {
	return override != nil && ValidateMove(now, schedule, *override) == nil
}

func ValidateMove(now time.Time, schedule Schedule, target time.Time) error {
	if strings.TrimSpace(schedule.Cron) == "" || !target.After(now) {
		return ErrInvalidMove
	}
	_, second := RegularOccurrences(now, schedule.Cron)
	if second == nil || !target.Before(*second) {
		return ErrInvalidMove
	}
	return nil
}
