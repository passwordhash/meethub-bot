package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
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
	dayStart := time.Date(2026, 9, 28, 0, 0, 0, 0, channelmodels.Moscow)
	dayEnd := dayStart.AddDate(0, 0, 1)
	watermark := monday.Add(-time.Hour - 30*time.Second)
	grid := map[time.Weekday]time.Duration{time.Monday: 10 * time.Hour}
	tests := []struct {
		name        string
		schedule    channelmodels.Schedule
		now         time.Time
		claimed     []pubmodels.Slot
		slotState   pubmodels.State
		wantLookup  bool
		wantMissed  bool
		wantPublish bool
	}{
		{name: "before post time nothing happens", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-61 * time.Minute)},
		{name: "post goes out at post time", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-time.Hour + 5*time.Second), slotState: pubmodels.StatePlanned, wantLookup: true, wantPublish: true},
		{name: "late post still goes out before daily starts", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-time.Second), slotState: pubmodels.StatePlanned, wantLookup: true, wantPublish: true},
		{name: "unpublished slot is missed once daily started", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(5 * time.Second), slotState: pubmodels.StatePlanned, wantLookup: true, wantMissed: true},
		{name: "published date is not posted again", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-30 * time.Minute), claimed: []pubmodels.Slot{{State: pubmodels.StatePublished}}, wantLookup: true},
		{name: "published date is not marked missed after daily time moved", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(5 * time.Second), claimed: []pubmodels.Slot{{State: pubmodels.StatePublished}}, wantLookup: true},
		{name: "post-time slot of the deploy day blocks the new daily start", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-30 * time.Minute), claimed: []pubmodels.Slot{{ScheduledAt: monday.Add(-time.Hour), State: pubmodels.StatePublished}}, wantLookup: true},
		{name: "post-time slot of the deploy day is not followed by missed", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(5 * time.Second), claimed: []pubmodels.Slot{{ScheduledAt: monday.Add(-time.Hour), State: pubmodels.StatePublished}}, wantLookup: true},
		{name: "unknown outcome is not retried", schedule: channelmodels.Schedule{DailyTimes: grid, PostOffsetMinutes: 60}, now: monday.Add(-30 * time.Minute), claimed: []pubmodels.Slot{{State: pubmodels.StateUnknown}}, wantLookup: true},
		{name: "empty grid is skipped", schedule: channelmodels.Schedule{PostOffsetMinutes: 60}, now: monday},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			channels := NewMockscheduleRepository(ctrl)
			slots := NewMockscheduleSlotRepository(ctrl)
			notifier := NewMockpublicationNotifier(ctrl)
			prepareSlots := NewMockslotRepository(ctrl)
			attemptSlots := NewMockattemptSlotRepository(ctrl)
			attemptChannels := NewMockattemptChannelRepository(ctrl)
			client := NewMockpostingClient(ctrl)
			tt.schedule.ChannelID = -1
			tt.schedule.CheckedAt = &watermark
			channels.EXPECT().GetSchedules(gomock.Any(), gomock.Any()).Return([]channelmodels.Schedule{tt.schedule}, nil)
			slot := pubmodels.Slot{ID: uuid.New(), ChannelID: -1, ScheduledAt: monday, State: tt.slotState}
			if tt.wantLookup {
				slots.EXPECT().GetSlots(gomock.Any(), pubmodels.GetSlotsFilters{ChannelID: ptr(int64(-1)), State: pubmodels.ClaimedStates, ScheduledFrom: &dayStart, ScheduledTo: &dayEnd}).Return(tt.claimed, nil)
			}
			if tt.wantMissed {
				slots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(slot, nil)
				slots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StatePlanned, pubmodels.StateMissed, nil, nil, nil).Return(pubmodels.Slot{}, nil)
				notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(-1)).Return(nil)
			}
			if tt.wantPublish {
				slots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(slot, nil)
				prepareSlots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(slot, nil)
				prepareSlots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(nil, nil)
				attemptChannels.EXPECT().GetChannels(gomock.Any(), gomock.Any()).Return([]channelmodels.Channel{{ChannelID: -1, GroupID: -2}}, nil)
				client.EXPECT().VerifyChannel(gomock.Any(), int64(-1), int64(99)).Return(validPermissions(-2), nil)
				attemptSlots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StatePlanned, pubmodels.StateAttempting, nil, nil, nil).Return(slot, nil)
				client.EXPECT().SendPost(gomock.Any(), int64(-1), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, text string) (int64, error) {
					if !strings.Contains(text, "пн, 28.09.2026") || !strings.Contains(text, "Созвон в 10:00 (UTC+3)") {
						t.Errorf("post does not carry the daily start: %s", text)
					}
					return 5, nil
				})
				attemptSlots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StateAttempting, pubmodels.StatePublished, ptr(int64(5)), nil, nil).Return(slot, nil)
			}
			channels.EXPECT().SetScheduleCheckedAt(gomock.Any(), int64(-1), tt.now).Return(nil)
			attempt, err := NewAttemptUseCase(attemptSlots, attemptChannels, client, notifier, 99, "https://meet.example/room")
			if err != nil {
				t.Fatal(err)
			}
			uc := &SchedulerUseCase{channels: channels, slots: slots, prepare: NewPrepareUseCase(prepareSlots, nil, nil), attempt: attempt, notifier: notifier}
			if err := uc.Tick(context.Background(), tt.now); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSchedulerUseCaseTickSurvivesPublishFailure(t *testing.T) {
	monday := time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow)
	grid := map[time.Weekday]time.Duration{time.Monday: 10 * time.Hour}
	broken := channelmodels.Schedule{ChannelID: -1, DailyTimes: grid, PostOffsetMinutes: 60}
	healthy := channelmodels.Schedule{ChannelID: -2, DailyTimes: grid, PostOffsetMinutes: 60}
	first := monday.Add(-30 * time.Minute)
	second := first.Add(15 * time.Second)
	afterStart := monday.Add(5 * time.Second)
	ctx := context.Background()
	f := newPublishFixture(t)

	brokenSlot := pubmodels.Slot{ID: uuid.New(), ChannelID: -1, ScheduledAt: monday, State: pubmodels.StatePlanned}
	healthySlot := pubmodels.Slot{ID: uuid.New(), ChannelID: -2, ScheduledAt: monday, State: pubmodels.StatePlanned}

	f.expectSchedules(broken, healthy)
	f.expectNothingClaimed(-1, monday)
	f.expectNothingClaimed(-2, monday)
	f.expectBrokenAttempt(brokenSlot, errors.New("verification failed"))
	f.expectSuccessfulAttempt(healthySlot)
	f.notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(-1)).Return(nil)
	f.slots.EXPECT().MarkPublishAlerted(gomock.Any(), brokenSlot.ID).Return(nil)
	f.expectWatermark(-1, first)
	f.expectWatermark(-2, first)
	if err := f.uc.Tick(ctx, first); err != nil {
		t.Fatalf("first tick must survive a publication failure: %v", err)
	}

	alerted := brokenSlot
	alerted.PublishAlertedAt = &first
	f.expectSchedules(broken)
	f.expectNothingClaimed(-1, monday)
	f.expectBrokenAttempt(alerted, errors.New("verification failed"))
	f.expectWatermark(-1, second)
	if err := f.uc.Tick(ctx, second); err != nil {
		t.Fatalf("second tick must retry silently: %v", err)
	}

	f.expectSchedules(broken)
	f.expectNothingClaimed(-1, monday)
	f.slots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(alerted, nil)
	f.slots.EXPECT().Transition(gomock.Any(), alerted.ID, pubmodels.StatePlanned, pubmodels.StateMissed, nil, nil, nil).Return(pubmodels.Slot{}, nil)
	f.notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(-1)).Return(nil)
	f.expectWatermark(-1, afterStart)
	if err := f.uc.Tick(ctx, afterStart); err != nil {
		t.Fatalf("tick after the daily started: %v", err)
	}
}

func TestSchedulerUseCaseTickFailedAlertIsNotMarked(t *testing.T) {
	monday := time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow)
	now := monday.Add(-30 * time.Minute)
	schedule := channelmodels.Schedule{ChannelID: -1, DailyTimes: map[time.Weekday]time.Duration{time.Monday: 10 * time.Hour}, PostOffsetMinutes: 60}
	slot := pubmodels.Slot{ID: uuid.New(), ChannelID: -1, ScheduledAt: monday, State: pubmodels.StatePlanned}
	f := newPublishFixture(t)
	f.expectSchedules(schedule)
	f.expectNothingClaimed(-1, monday)
	f.expectBrokenAttempt(slot, errors.New("verification failed"))
	f.notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(-1)).Return(errors.New("telegram is down"))
	f.expectWatermark(-1, now)
	if err := f.uc.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerUseCaseTickCancellationIsNotSwallowed(t *testing.T) {
	monday := time.Date(2026, 9, 28, 10, 0, 0, 0, channelmodels.Moscow)
	now := monday.Add(-30 * time.Minute)
	schedule := channelmodels.Schedule{ChannelID: -1, DailyTimes: map[time.Weekday]time.Duration{time.Monday: 10 * time.Hour}, PostOffsetMinutes: 60}
	slot := pubmodels.Slot{ID: uuid.New(), ChannelID: -1, ScheduledAt: monday, State: pubmodels.StatePlanned}
	f := newPublishFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.expectSchedules(schedule)
	f.expectNothingClaimed(-1, monday)
	f.slots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(slot, nil)
	f.prepareSlots.EXPECT().Create(gomock.Any(), int64(-1), monday).Return(slot, nil)
	f.prepareSlots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.attemptChannels.EXPECT().GetChannels(gomock.Any(), gomock.Any()).Return([]channelmodels.Channel{{ChannelID: -1, GroupID: -2}}, nil)
	f.client.EXPECT().VerifyChannel(gomock.Any(), int64(-1), int64(99)).DoAndReturn(func(context.Context, int64, int64) (channelmodels.Permissions, error) {
		cancel()
		return channelmodels.Permissions{}, context.Canceled
	})
	if err := f.uc.Tick(ctx, now); !errors.Is(err, context.Canceled) {
		t.Fatalf("Tick() = %v; want context cancellation", err)
	}
}

type publishFixture struct {
	uc              *SchedulerUseCase
	channels        *MockscheduleRepository
	slots           *MockscheduleSlotRepository
	notifier        *MockpublicationNotifier
	prepareSlots    *MockslotRepository
	attemptSlots    *MockattemptSlotRepository
	attemptChannels *MockattemptChannelRepository
	client          *MockpostingClient
}

func newPublishFixture(t *testing.T) *publishFixture {
	ctrl := gomock.NewController(t)
	f := &publishFixture{
		channels:        NewMockscheduleRepository(ctrl),
		slots:           NewMockscheduleSlotRepository(ctrl),
		notifier:        NewMockpublicationNotifier(ctrl),
		prepareSlots:    NewMockslotRepository(ctrl),
		attemptSlots:    NewMockattemptSlotRepository(ctrl),
		attemptChannels: NewMockattemptChannelRepository(ctrl),
		client:          NewMockpostingClient(ctrl),
	}
	attempt, err := NewAttemptUseCase(f.attemptSlots, f.attemptChannels, f.client, f.notifier, 99, "https://meet.example/room")
	if err != nil {
		t.Fatal(err)
	}
	f.uc = &SchedulerUseCase{channels: f.channels, slots: f.slots, prepare: NewPrepareUseCase(f.prepareSlots, nil, nil), attempt: attempt, notifier: f.notifier}
	return f
}

func (f *publishFixture) expectSchedules(schedules ...channelmodels.Schedule) {
	watermark := time.Date(2026, 9, 28, 8, 0, 0, 0, channelmodels.Moscow)
	for index := range schedules {
		schedules[index].CheckedAt = &watermark
	}
	f.channels.EXPECT().GetSchedules(gomock.Any(), gomock.Any()).Return(schedules, nil)
}

func (f *publishFixture) expectWatermark(channelID int64, now time.Time) {
	f.channels.EXPECT().SetScheduleCheckedAt(gomock.Any(), channelID, now).Return(nil)
}

func (f *publishFixture) expectNothingClaimed(channelID int64, startsAt time.Time) {
	dayStart, dayEnd := channelmodels.MoscowDay(startsAt)
	f.slots.EXPECT().GetSlots(gomock.Any(), pubmodels.GetSlotsFilters{ChannelID: &channelID, State: pubmodels.ClaimedStates, ScheduledFrom: &dayStart, ScheduledTo: &dayEnd}).Return(nil, nil)
}

func (f *publishFixture) expectPreparation(slot pubmodels.Slot) {
	f.slots.EXPECT().Create(gomock.Any(), slot.ChannelID, slot.ScheduledAt).Return(slot, nil)
	f.prepareSlots.EXPECT().Create(gomock.Any(), slot.ChannelID, slot.ScheduledAt).Return(slot, nil)
	f.prepareSlots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.attemptChannels.EXPECT().GetChannels(gomock.Any(), channelmodels.GetChannelsFilters{ChannelID: &slot.ChannelID}).Return([]channelmodels.Channel{{ChannelID: slot.ChannelID, GroupID: -20}}, nil)
}

func (f *publishFixture) expectBrokenAttempt(slot pubmodels.Slot, verifyErr error) {
	f.expectPreparation(slot)
	f.client.EXPECT().VerifyChannel(gomock.Any(), slot.ChannelID, int64(99)).Return(channelmodels.Permissions{}, verifyErr)
}

func (f *publishFixture) expectSuccessfulAttempt(slot pubmodels.Slot) {
	f.expectPreparation(slot)
	f.client.EXPECT().VerifyChannel(gomock.Any(), slot.ChannelID, int64(99)).Return(validPermissions(-20), nil)
	f.attemptSlots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StatePlanned, pubmodels.StateAttempting, nil, nil, nil).Return(slot, nil)
	f.client.EXPECT().SendPost(gomock.Any(), slot.ChannelID, gomock.Any()).Return(int64(5), nil)
	f.attemptSlots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StateAttempting, pubmodels.StatePublished, ptr(int64(5)), nil, nil).Return(slot, nil)
}
