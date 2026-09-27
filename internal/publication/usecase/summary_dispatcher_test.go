package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

func TestSummaryDispatcherTick(t *testing.T) {
	linked := int64(12)
	id := uuid.New()
	tests := []struct {
		name     string
		linkedID *int64
		parts    []pubmodels.SummaryPart
		wantCall bool
	}{
		{name: "no link", parts: []pubmodels.SummaryPart{{Status: "pending"}}},
		{name: "linked pending", linkedID: &linked, parts: []pubmodels.SummaryPart{{Status: "pending"}}, wantCall: true},
		{name: "sent repeat", linkedID: &linked, parts: []pubmodels.SummaryPart{{Status: "sent"}}},
		{name: "unknown no retry", linkedID: &linked, parts: []pubmodels.SummaryPart{{Status: "unknown"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slots := NewMocksummarySlotRepository(ctrl)
			parts := NewMocksummaryDeliveryRepository(ctrl)
			delivery := NewMocksummaryDelivery(ctrl)
			slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return([]pubmodels.Slot{{ID: id, State: pubmodels.StatePublished, LinkedMessageID: tt.linkedID}}, nil)
			if tt.linkedID != nil {
				parts.EXPECT().GetSummaryParts(gomock.Any(), gomock.Any()).Return(tt.parts, nil)
			}
			if tt.wantCall {
				delivery.EXPECT().DeliverSummary(gomock.Any(), id).Return(nil)
			}
			dispatcher, err := NewSummaryDispatcherUseCase(slots, parts, delivery)
			if err != nil {
				t.Fatal(err)
			}
			if err := dispatcher.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSummaryDispatcherLinkAlert(t *testing.T) {
	id := uuid.New()
	channelID := int64(-1001)
	linked := int64(12)
	old := time.Now().Add(-6 * time.Minute)
	recent := time.Now().Add(-4 * time.Minute)
	already := time.Now().Add(-time.Minute)
	tests := []struct {
		name        string
		slot        pubmodels.Slot
		notifyErr   error
		wantNotify  bool
		wantMark    bool
		wantRetry   bool
		wantDeliver bool
	}{
		{name: "before grace", slot: pubmodels.Slot{ID: id, ChannelID: channelID, State: pubmodels.StatePublished, PublishedAt: &recent}},
		{name: "after grace alert once", slot: pubmodels.Slot{ID: id, ChannelID: channelID, State: pubmodels.StatePublished, PublishedAt: &old}, wantNotify: true, wantMark: true},
		{name: "notify fails and retries", slot: pubmodels.Slot{ID: id, ChannelID: channelID, State: pubmodels.StatePublished, PublishedAt: &old}, notifyErr: errors.New("notify failed"), wantNotify: true, wantRetry: true},
		{name: "persisted alert", slot: pubmodels.Slot{ID: id, ChannelID: channelID, State: pubmodels.StatePublished, PublishedAt: &old, LinkAlertedAt: &already}},
		{name: "link arrives and pending summary delivered", slot: pubmodels.Slot{ID: id, ChannelID: channelID, State: pubmodels.StatePublished, PublishedAt: &old, LinkedMessageID: &linked}, wantDeliver: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			slots := NewMocksummarySlotRepository(ctrl)
			parts := NewMocksummaryDeliveryRepository(ctrl)
			delivery := NewMocksummaryDelivery(ctrl)
			slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return([]pubmodels.Slot{tt.slot}, nil)
			if tt.wantNotify {
				delivery.EXPECT().NotifyLinkMissing(gomock.Any(), channelID).Return(tt.notifyErr)
			}
			if tt.wantMark {
				slots.EXPECT().MarkLinkAlerted(gomock.Any(), id).Return(nil)
			}
			if tt.wantDeliver {
				parts.EXPECT().GetSummaryParts(gomock.Any(), gomock.Any()).Return([]pubmodels.SummaryPart{{Status: "pending"}}, nil)
				delivery.EXPECT().DeliverSummary(gomock.Any(), id).Return(nil)
			}
			dispatcher, err := NewSummaryDispatcherUseCase(slots, parts, delivery)
			if err != nil {
				t.Fatal(err)
			}
			err = dispatcher.Tick(context.Background())
			if (err != nil) != tt.wantRetry {
				t.Fatalf("Tick() error = %v, wantRetry %t", err, tt.wantRetry)
			}
		})
	}
}

func TestSummaryDispatcherRecover(t *testing.T) {
	ctrl := gomock.NewController(t)
	delivery := NewMocksummaryDelivery(ctrl)
	delivery.EXPECT().RecoverAttempting(gomock.Any()).Return(nil)
	dispatcher, err := NewSummaryDispatcherUseCase(NewMocksummarySlotRepository(ctrl), NewMocksummaryDeliveryRepository(ctrl), delivery)
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
}
