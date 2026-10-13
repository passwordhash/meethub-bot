package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

func TestAttempt(t *testing.T) {
	tests := []struct {
		name         string
		state        pubmodels.State
		url          string
		channels     []channelmodels.Channel
		permissions  channelmodels.Permissions
		channelErr   error
		verifyErr    error
		sendID       int64
		sendErr      error
		publishedErr error
		wantState    pubmodels.State
		wantError    bool
		wantVerify   bool
		wantSend     bool
		wantNotify   bool
	}{
		{name: "missing URL", state: pubmodels.StatePlanned, channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, permissions: validPermissions(8), wantState: pubmodels.StatePlanned, wantError: true},
		{name: "disconnected channel", state: pubmodels.StatePlanned, url: "https://meet.example/room", wantState: pubmodels.StatePlanned, wantError: true},
		{name: "wrong group", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 9}}, permissions: validPermissions(8), wantState: pubmodels.StatePlanned, wantError: true, wantVerify: true},
		{name: "verification failed", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, verifyErr: errors.New("verification failed"), wantState: pubmodels.StatePlanned, wantError: true, wantVerify: true},
		{name: "no posting rights", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, wantState: pubmodels.StatePlanned, wantError: true, wantVerify: true},
		{name: "success", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, permissions: validPermissions(8), sendID: 123, wantState: pubmodels.StatePublished, wantVerify: true, wantSend: true},
		{name: "send outcome unknown", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, permissions: validPermissions(8), sendErr: errors.New("transport failed"), wantState: pubmodels.StateUnknown, wantError: true, wantVerify: true, wantSend: true, wantNotify: true},
		{name: "nonpositive message ID", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, permissions: validPermissions(8), wantState: pubmodels.StateUnknown, wantError: true, wantVerify: true, wantSend: true, wantNotify: true},
		{name: "already published", state: pubmodels.StatePublished, url: "https://meet.example/room", wantState: pubmodels.StatePublished, wantError: true},
		{name: "already unknown", state: pubmodels.StateUnknown, url: "https://meet.example/room", wantState: pubmodels.StateUnknown, wantError: true},
		{name: "published transition failed", state: pubmodels.StatePlanned, url: "https://meet.example/room", channels: []channelmodels.Channel{{ChannelID: 7, GroupID: 8}}, permissions: validPermissions(8), sendID: 123, publishedErr: errors.New("db failed"), wantState: pubmodels.StateAttempting, wantError: true, wantVerify: true, wantSend: true, wantNotify: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slots := NewMockattemptSlotRepository(ctrl)
			channels := NewMockattemptChannelRepository(ctrl)
			client := NewMockpostingClient(ctrl)
			notifier := NewMockpublicationNotifier(ctrl)
			uc, err := NewAttemptUseCase(slots, channels, client, notifier, 99, tt.url)
			if err != nil {
				t.Fatal(err)
			}
			slot := pubmodels.Slot{ID: uuid.New(), ChannelID: 7, ScheduledAt: time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC), State: tt.state}
			if tt.state != pubmodels.StatePlanned {
				got, err := uc.Attempt(context.Background(), Prepared{Slot: slot})
				if (err != nil) != tt.wantError || got.State != tt.wantState {
					t.Fatalf("Attempt()=(%v,%v), want state %s and error=%v", got.State, err, tt.wantState, tt.wantError)
				}
				return
			}
			if tt.url == "" {
				_, err := uc.Attempt(context.Background(), Prepared{Slot: slot})
				if err == nil {
					t.Fatal("Attempt() error=nil, want missing URL error")
				}
				return
			}
			channels.EXPECT().GetChannels(gomock.Any(), channelmodels.GetChannelsFilters{ChannelID: ptr(int64(7))}).Return(tt.channels, tt.channelErr)
			if len(tt.channels) > 0 && tt.channelErr == nil {
				client.EXPECT().VerifyChannel(gomock.Any(), int64(7), int64(99)).Return(tt.permissions, tt.verifyErr).Times(boolCount(tt.wantVerify))
				if tt.wantVerify && tt.verifyErr == nil && tt.permissions.Validate(7) == nil && tt.permissions.LinkedGroupID == tt.channels[0].GroupID {
					slotTrying := slot
					slotTrying.State = pubmodels.StateAttempting
					slots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StatePlanned, pubmodels.StateAttempting, nil, nil, slot.LeadUserID).Return(slotTrying, nil)
					if tt.wantSend {
						client.EXPECT().SendPost(gomock.Any(), int64(7), gomock.Any()).Return(tt.sendID, tt.sendErr)
						if tt.sendErr == nil && tt.sendID > 0 {
							final := slot
							final.State = tt.wantState
							var stepErr error
							if tt.publishedErr != nil {
								stepErr = tt.publishedErr
							}
							slots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StateAttempting, pubmodels.StatePublished, gomock.Any(), nil, slot.LeadUserID).Return(final, stepErr)
							if stepErr != nil {
								notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(7)).Return(nil)
							}
						} else {
							final := slot
							final.State = pubmodels.StateUnknown
							slots.EXPECT().Transition(gomock.Any(), slot.ID, pubmodels.StateAttempting, pubmodels.StateUnknown, nil, nil, slot.LeadUserID).Return(final, nil)
							notifier.EXPECT().NotifyPublicationProblem(gomock.Any(), int64(7)).Return(nil)
						}
					}
				}
			}
			got, err := uc.Attempt(context.Background(), Prepared{Slot: slot})
			if (err != nil) != tt.wantError {
				t.Fatalf("Attempt() error=%v, wantError=%v", err, tt.wantError)
			}
			if got.State != tt.wantState {
				t.Fatalf("Attempt() state=%s, want %s", got.State, tt.wantState)
			}
		})
	}
}

func validPermissions(groupID int64) channelmodels.Permissions {
	return channelmodels.Permissions{ChannelType: "channel", LinkedGroupID: groupID, OwnerIsAdmin: true, BotIsAdmin: true, BotCanPost: true, GroupType: "supergroup", BotInGroup: true, BotCanSend: true}
}
func ptr(value int64) *int64 { return &value }
func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
