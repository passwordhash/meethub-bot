package models

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidSchedule = errors.New("invalid schedule")
var ErrInvalidMove = errors.New("invalid move")

var Moscow = time.FixedZone("UTC+3", 3*60*60)

func ParseWeekdays(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	seen := make(map[int]bool, len(parts))
	weekdays := make([]int, 0, len(parts))
	for _, part := range parts {
		day, err := strconv.Atoi(part)
		if err != nil || day < 1 || day > 7 || seen[day] {
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

func NextOccurrence(now time.Time, weekdays []int, hour, minute int, override *time.Time) *time.Time {
	first, second := RegularOccurrences(now, weekdays, hour, minute)
	if first == nil {
		return nil
	}
	if override != nil && override.After(now) && second != nil && override.Before(*second) {
		target := override.In(Moscow)
		return &target
	}
	return first
}

func RegularOccurrences(now time.Time, weekdays []int, hour, minute int) (*time.Time, *time.Time) {
	if len(weekdays) == 0 {
		return nil, nil
	}
	now = now.In(Moscow)
	var occurrences []time.Time
	for day := 0; day < 15; day++ {
		candidateDate := now.AddDate(0, 0, day)
		weekday := int(candidateDate.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		for _, scheduled := range weekdays {
			if weekday != scheduled {
				continue
			}
			candidate := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), hour, minute, 0, 0, Moscow)
			if candidate.After(now) {
				occurrences = append(occurrences, candidate)
			}
		}
	}
	sort.Slice(occurrences, func(i, j int) bool { return occurrences[i].Before(occurrences[j]) })
	if len(occurrences) == 0 {
		return nil, nil
	}
	first := occurrences[0]
	if len(occurrences) == 1 {
		return &first, nil
	}
	second := occurrences[1]
	return &first, &second
}

func KeepOverrideAfterScheduleEdit(now time.Time, schedule Schedule, override *time.Time) bool {
	return override != nil && ValidateMove(now, schedule, *override) == nil
}

func ValidateMove(now time.Time, schedule Schedule, target time.Time) error {
	if len(schedule.Weekdays) == 0 || !target.After(now) {
		return ErrInvalidMove
	}
	_, second := RegularOccurrences(now, schedule.Weekdays, schedule.Hour, schedule.Minute)
	if second == nil || !target.Before(*second) {
		return ErrInvalidMove
	}
	return nil
}
