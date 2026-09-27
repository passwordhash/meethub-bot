package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

func TestRecoverAttempting(t *testing.T) {
	tests := []struct {
		name            string
		channelIDs      []int64
		notificationErr bool
		recoveryErr     bool
		wantErr         bool
	}{
		{name: "empty"},
		{name: "one channel", channelIDs: []int64{-1001}},
		{name: "two channels", channelIDs: []int64{-1001, -1002}},
		{name: "notification failure prevents recovery", channelIDs: []int64{-1001}, notificationErr: true, wantErr: true},
		{name: "database failure after notification", channelIDs: []int64{-1001}, recoveryErr: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			parts := NewMocksummaryDeliveryRepository(ctrl)
			notifier := NewMocksummaryNotifier(ctrl)
			parts.EXPECT().ListAttemptingChannels(gomock.Any()).Return(tt.channelIDs, nil)
			for _, channelID := range tt.channelIDs {
				err := error(nil)
				if tt.notificationErr {
					err = errors.New("notification")
				}
				notifier.EXPECT().NotifySummaryProblem(gomock.Any(), channelID).Return(err)
				if tt.notificationErr {
					break
				}
			}
			if !tt.notificationErr {
				err := error(nil)
				if tt.recoveryErr {
					err = errors.New("database")
				}
				parts.EXPECT().RecoverAttempting(gomock.Any()).Return(err)
			}
			usecase, err := NewSummaryDeliveryUseCase(NewMocksummarySlotRepository(ctrl), parts, NewMockattemptChannelRepository(ctrl), NewMocksummaryTelegramClient(ctrl), notifier, 7)
			if err != nil {
				t.Fatal(err)
			}
			if err := usecase.RecoverAttempting(context.Background()); (err != nil) != tt.wantErr {
				t.Fatalf("RecoverAttempting() error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeliverSummary(t *testing.T) {
	tests := []struct {
		name        string
		sendError   error
		wantUnknown bool
	}{
		{name: "sent"},
		{name: "uncertain", sendError: errors.New("transport"), wantUnknown: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slots := NewMocksummarySlotRepository(ctrl)
			parts := NewMocksummaryDeliveryRepository(ctrl)
			channels := NewMockattemptChannelRepository(ctrl)
			telegram := NewMocksummaryTelegramClient(ctrl)
			notifier := NewMocksummaryNotifier(ctrl)
			slotID := uuid.New()
			channelMessageID, linkedMessageID := int64(31), int64(41)
			groupID := int64(-2002)
			slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return([]pubmodels.Slot{{ID: slotID, ChannelID: -1001, State: pubmodels.StatePublished, ChannelMessageID: &channelMessageID, LinkedMessageID: &linkedMessageID}}, nil)
			parts.EXPECT().GetSummaryParts(gomock.Any(), gomock.Any()).Return([]pubmodels.SummaryPart{{EditionSlotID: slotID, PartIndex: 0, Text: "summary", Status: "pending"}}, nil)
			channels.EXPECT().GetChannels(gomock.Any(), gomock.Any()).Return([]channelmodels.Channel{{ChannelID: -1001, GroupID: groupID}}, nil)
			permissions := channelmodels.Permissions{ChannelType: "channel", LinkedGroupID: groupID, OwnerIsAdmin: true, BotIsAdmin: true, BotCanPost: true, GroupType: "supergroup", BotInGroup: true, BotCanSend: true}
			telegram.EXPECT().VerifyChannel(gomock.Any(), int64(-1001), int64(7)).Return(permissions, nil)
			parts.EXPECT().Begin(gomock.Any(), slotID, 0).Return(nil)
			telegram.EXPECT().SendReply(gomock.Any(), groupID, linkedMessageID, "summary").Return(int64(51), tt.sendError)
			if tt.wantUnknown {
				parts.EXPECT().Unknown(gomock.Any(), slotID, 0).Return(nil)
				notifier.EXPECT().NotifySummaryProblem(gomock.Any(), int64(-1001)).Return(nil)
			} else {
				parts.EXPECT().Sent(gomock.Any(), slotID, 0, int64(51)).Return(nil)
			}
			usecase, err := NewSummaryDeliveryUseCase(slots, parts, channels, telegram, notifier, 7)
			if err != nil {
				t.Fatal(err)
			}
			err = usecase.DeliverSummary(context.Background(), slotID)
			if (err != nil) != tt.wantUnknown {
				t.Fatalf("DeliverSummary() error = %v, wantUnknown %v", err, tt.wantUnknown)
			}
		})
	}
}
