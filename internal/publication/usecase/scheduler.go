package usecase

import (
	"context"
	"fmt"
	"log/slog"
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
		if err := u.processSchedule(ctx, schedule, now); err != nil {
			return err
		}
	}
	return nil
}

func (u *SchedulerUseCase) processSchedule(ctx context.Context, schedule channelmodels.Schedule, now time.Time) error {
	occurrences := pubdomain.ScheduleOccurrences(schedule, schedule.CheckedAt, now)
	for _, occurrence := range occurrences {
		claimed, err := u.dateClaimed(ctx, schedule.ChannelID, occurrence.StartsAt)
		if err != nil {
			return err
		}
		if claimed {
			continue
		}
		if occurrence.Missed {
			if err := u.markMissed(ctx, schedule.ChannelID, occurrence.StartsAt); err != nil {
				return err
			}
			continue
		}
		if occurrence.Due {
			if err := u.publishIfUnclaimed(ctx, schedule.ChannelID, occurrence.StartsAt, schedule.LeadEnabled); err != nil {
				return err
			}
		}
	}
	if err := u.channels.SetScheduleCheckedAt(ctx, schedule.ChannelID, now); err != nil {
		return fmt.Errorf("save publication schedule watermark: %w", err)
	}
	return nil
}

func (u *SchedulerUseCase) dateClaimed(ctx context.Context, channelID int64, startsAt time.Time) (bool, error) {
	dayStart, dayEnd := channelmodels.MoscowDay(startsAt)
	slots, err := u.slots.GetSlots(ctx, pubmodels.GetSlotsFilters{ChannelID: &channelID, State: pubmodels.ClaimedStates, ScheduledFrom: &dayStart, ScheduledTo: &dayEnd})
	if err != nil {
		return false, fmt.Errorf("get publication slots of the day: %w", err)
	}
	return len(slots) > 0, nil
}

func (u *SchedulerUseCase) publishIfUnclaimed(ctx context.Context, channelID int64, startsAt time.Time, leadEnabled bool) error {
	slot, err := u.slots.Create(ctx, channelID, startsAt)
	if err != nil {
		return fmt.Errorf("create due publication slot: %w", err)
	}
	if slot.State != pubmodels.StatePlanned {
		return nil
	}
	remaining, publishErr := u.publish(ctx, slot, leadEnabled)
	if publishErr == nil {
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("publish scheduled slot: %w", publishErr)
	}
	slog.Error("scheduled publication failed", "channel_id", channelID, "scheduled_at", startsAt, "error", publishErr)
	if remaining.State != pubmodels.StatePlanned {
		return nil
	}
	return u.alertPublishProblem(ctx, slot)
}

func (u *SchedulerUseCase) publish(ctx context.Context, slot pubmodels.Slot, leadEnabled bool) (pubmodels.Slot, error) {
	prepared, err := u.prepare.Prepare(ctx, slot.ChannelID, slot.ScheduledAt, leadEnabled)
	if err != nil {
		return slot, fmt.Errorf("prepare scheduled publication: %w", err)
	}
	attempted, err := u.attempt.Attempt(ctx, prepared)
	if err != nil {
		return attempted, fmt.Errorf("attempt scheduled publication: %w", err)
	}
	return attempted, nil
}

func (u *SchedulerUseCase) alertPublishProblem(ctx context.Context, slot pubmodels.Slot) error {
	if slot.PublishAlertedAt != nil {
		return nil
	}
	if err := u.notifier.NotifyPublicationProblem(ctx, slot.ChannelID); err != nil {
		slog.Error("owner notification about failed publication failed", "channel_id", slot.ChannelID, "scheduled_at", slot.ScheduledAt, "error", err)
		return nil
	}
	if err := u.slots.MarkPublishAlerted(ctx, slot.ID); err != nil {
		return fmt.Errorf("mark publication failure alert: %w", err)
	}
	return nil
}

func (u *SchedulerUseCase) markMissed(ctx context.Context, channelID int64, startsAt time.Time) error {
	slot, err := u.slots.Create(ctx, channelID, startsAt)
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
