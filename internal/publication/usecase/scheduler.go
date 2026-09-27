package usecase

import (
	"context"
	"fmt"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubdomain "github.com/passwordhash/cascade-hub-bot/internal/publication/domain"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type SchedulerUseCase struct {
	channels scheduleRepository
	slots    scheduleSlotRepository
	prepare  *PrepareUseCase
	attempt  *AttemptUseCase
	notifier publicationNotifier
}

func NewSchedulerUseCase(channels scheduleRepository, slots scheduleSlotRepository, prepare *PrepareUseCase, attempt *AttemptUseCase, notifier publicationNotifier) (*SchedulerUseCase, error) {
	if channels == nil || slots == nil || prepare == nil || attempt == nil || notifier == nil {
		return nil, fmt.Errorf("publication scheduler dependencies are required")
	}
	return &SchedulerUseCase{channels: channels, slots: slots, prepare: prepare, attempt: attempt, notifier: notifier}, nil
}

func (u *SchedulerUseCase) Recover(ctx context.Context) error {
	slots, err := u.slots.Recover(ctx)
	if err != nil {
		return fmt.Errorf("recover publication slots: %w", err)
	}
	for _, slot := range slots {
		if err := u.notifier.NotifyPublicationProblem(ctx, slot.ChannelID); err != nil {
			return fmt.Errorf("notify owner about recovered publication: %w", err)
		}
	}
	return nil
}

func (u *SchedulerUseCase) Tick(ctx context.Context, now time.Time) error {
	schedules, err := u.channels.GetSchedules(ctx, channelmodels.GetSchedulesFilters{})
	if err != nil {
		return fmt.Errorf("get publication schedules: %w", err)
	}
	for _, schedule := range schedules {
		if len(schedule.Weekdays) == 0 {
			continue
		}
		if err := u.processSchedule(ctx, schedule, now); err != nil {
			return err
		}
	}
	return nil
}

func (u *SchedulerUseCase) processSchedule(ctx context.Context, schedule channelmodels.Schedule, now time.Time) error {
	occurrences := pubdomain.ScheduleOccurrences(schedule, schedule.CheckedAt, now)
	for _, occurrence := range occurrences {
		if occurrence.Missed {
			if err := u.markMissed(ctx, schedule.ChannelID, occurrence.ScheduledAt); err != nil {
				return err
			}
			continue
		}
		if occurrence.Due {
			if err := u.publishIfUnclaimed(ctx, schedule.ChannelID, occurrence.ScheduledAt, schedule.LeadEnabled); err != nil {
				return err
			}
		}
	}
	if err := u.channels.SetScheduleCheckedAt(ctx, schedule.ChannelID, now); err != nil {
		return fmt.Errorf("save publication schedule watermark: %w", err)
	}
	return nil
}

func (u *SchedulerUseCase) publishIfUnclaimed(ctx context.Context, channelID int64, scheduledAt time.Time, leadEnabled bool) error {
	slot, err := u.slots.Create(ctx, channelID, scheduledAt)
	if err != nil {
		return fmt.Errorf("create due publication slot: %w", err)
	}
	if slot.State != pubmodels.StatePlanned {
		return nil
	}
	prepared, err := u.prepare.Prepare(ctx, channelID, scheduledAt, leadEnabled)
	if err != nil {
		return fmt.Errorf("prepare scheduled publication: %w", err)
	}
	if _, err := u.attempt.Attempt(ctx, prepared); err != nil {
		return fmt.Errorf("attempt scheduled publication: %w", err)
	}
	return nil
}

func (u *SchedulerUseCase) markMissed(ctx context.Context, channelID int64, scheduledAt time.Time) error {
	slot, err := u.slots.Create(ctx, channelID, scheduledAt)
	if err != nil {
		return fmt.Errorf("create missed publication slot: %w", err)
	}
	if slot.State != pubmodels.StatePlanned {
		return nil
	}
	if _, err := u.slots.Transition(ctx, slot.ID, pubmodels.StatePlanned, pubmodels.StateMissed, nil, nil, nil); err != nil {
		return fmt.Errorf("mark publication missed: %w", err)
	}
	if err := u.notifier.NotifyPublicationProblem(ctx, channelID); err != nil {
		return fmt.Errorf("notify owner about missed publication: %w", err)
	}
	return nil
}
