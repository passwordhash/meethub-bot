package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubdomain "github.com/passwordhash/cascade-hub-bot/internal/publication/domain"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

func TestSchedulerUseCaseRecover(t *testing.T) {
	tests := []struct {
		name  string
		slots []pubmodels.Slot
	}{
		{name: "restart recovery notifies changed slots", slots: []pubmodels.Slot{{ChannelID: -10}, {ChannelID: -20}}},
		{name: "no changed slots", slots: []pubmodels.Slot{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slots := NewMockscheduleSlotRepository(ctrl)
			notifier := NewMockpublicationNotifier(ctrl)
			slots.EXPECT().Recover(gomock.Any()).Return(tt.slots, nil)
			for _, slot := range tt.slots {
				notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), slot.ChannelID).Return(nil)
			}
			uc := &SchedulerUseCase{slots: slots, notifier: notifier}
			if err := uc.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSchedulerUseCaseTick(t *testing.T) {
	monday := time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow)
	watermark := monday.AddDate(0, 0, -28)
	moved := monday.Add(-time.Hour)
	tests := []struct {
		name     string
		schedule channelmodels.Schedule
		now      time.Time
		states   []pubmodels.State
	}{
		{name: "due occurrence already claimed after old watermark", schedule: channelmodels.Schedule{ChannelID: -1, Cron: "0 10 * * 1"}, now: monday.Add(20 * time.Second), states: []pubmodels.State{pubmodels.StatePublished}},
		{name: "override replaces regular occurrence", schedule: channelmodels.Schedule{ChannelID: -1, Cron: "0 10 * * 1", Override: &moved}, now: monday.Add(20 * time.Second), states: []pubmodels.State{pubmodels.StatePlanned}},
		{name: "unknown is not retried", schedule: channelmodels.Schedule{ChannelID: -1, Cron: "0 10 * * 1"}, now: monday.Add(20 * time.Second), states: []pubmodels.State{pubmodels.StateUnknown}},
		{name: "repeat same minute published", schedule: channelmodels.Schedule{ChannelID: -1, Cron: "0 10 * * 1"}, now: monday.Add(40 * time.Second), states: []pubmodels.State{pubmodels.StatePublished}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			channels := NewMockscheduleRepository(ctrl)
			slots := NewMockscheduleSlotRepository(ctrl)
			notifier := NewMockpublicationNotifier(ctrl)
			tt.schedule.CheckedAt = &watermark
			channels.EXPECT().GetSchedules(gomock.Any(), gomock.Any()).Return([]channelmodels.Schedule{tt.schedule}, nil)
			occurrences := makeMissedScheduleOccurrences(tt.schedule, &watermark, tt.now)
			if len(occurrences) != len(tt.states) {
				t.Fatalf("got %d occurrences, want %d", len(occurrences), len(tt.states))
			}
			for index, scheduledAt := range occurrences {
				state := tt.states[index]
				slot := pubmodels.Slot{ID: uuid.New(), ChannelID: tt.schedule.ChannelID, ScheduledAt: scheduledAt, State: state}
				slots.EXPECT().Create(gomock.Any(), tt.schedule.ChannelID, scheduledAt).Return(slot, nil)
				if state == pubmodels.StatePlanned {
					slots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StatePlanned, pubmodels.StateMissed, nil, nil, nil).Return(pubmodels.Slot{}, nil)
					notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), tt.schedule.ChannelID).Return(nil)
				}
			}
			channels.EXPECT().SetScheduleCheckedAt(gomock.Any(), tt.schedule.ChannelID, tt.now).Return(nil)
			uc := &SchedulerUseCase{channels: channels, slots: slots, notifier: notifier}
			if err := uc.Tick(context.Background(), tt.now); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func makeMissedScheduleOccurrences(schedule channelmodels.Schedule, watermark *time.Time, now time.Time) []time.Time {
	occurrences := make([]time.Time, 0)
	for _, occurrence := range pubdomain.ScheduleOccurrences(schedule, watermark, now) {
		occurrences = append(occurrences, occurrence.ScheduledAt)
	}
	return occurrences
}
