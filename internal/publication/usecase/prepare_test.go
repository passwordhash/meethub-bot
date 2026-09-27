package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	reportmodels "github.com/passwordhash/cascade-hub-bot/internal/report/models"
	"go.uber.org/mock/gomock"
)

func TestPrepareLatestPublishedSnapshotAndReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	slots, reports, summaries := NewMockslotRepository(ctrl), NewMockreportRepository(ctrl), NewMocksummaryPreparationRepository(ctrl)
	edition, sourceOld, sourceNew := uuid.New(), uuid.New(), uuid.New()
	at := time.Now()
	older, newer := at.Add(-3*24*time.Hour), at.Add(-24*time.Hour)
	previous := int64(1)
	slot := pubmodels.Slot{ID: edition, ChannelID: 7, ScheduledAt: at, State: pubmodels.StatePlanned}
	history := []pubmodels.Slot{{ID: sourceOld, State: pubmodels.StatePublished, PublishedAt: &older}, {ID: sourceNew, State: pubmodels.StatePublished, PublishedAt: &newer, LeadUserID: &previous}}
	snapshots := []reportmodels.Snapshot{{EditionSlotID: edition, SourceSlotID: sourceNew, AuthorID: 1, Author: "One", Text: "Сегодня: old plan"}, {EditionSlotID: edition, SourceSlotID: sourceNew, AuthorID: 2, Author: "Two", Text: "Сегодня: plan"}}
	createCount := 0
	slots.EXPECT().Create(gomock.Any(), int64(7), at).DoAndReturn(func(context.Context, int64, time.Time) (pubmodels.Slot, error) {
		createCount++
		if createCount == 1 {
			return slot, nil
		}
		return storedSlot(slot, 2), nil
	}).Times(2)
	slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(history, nil).Times(2)
	reports.EXPECT().CreateSnapshot(gomock.Any(), edition, sourceNew).Return(nil).Times(2)
	reports.EXPECT().GetSnapshots(gomock.Any(), gomock.Any()).Return(snapshots, nil).Times(2)
	chosen := int64(2)
	stored := storedSlot(slot, chosen)
	slots.EXPECT().SetLead(gomock.Any(), edition, gomock.Any()).DoAndReturn(func(_ context.Context, _ uuid.UUID, id *int64) (pubmodels.Slot, error) {
		if id == nil || *id != 2 {
			t.Fatalf("selected lead %v, want 2", id)
		}
		return stored, nil
	})
	summaries.EXPECT().CreateSummaryParts(gomock.Any(), edition, gomock.Any()).Times(2)
	uc := NewPrepareUseCase(slots, reports, summaries)
	first, err := uc.Prepare(context.Background(), 7, at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Prepare(context.Background(), 7, at)
	if err != nil {
		t.Fatal(err)
	}
	if first.Lead == nil || second.Lead == nil || first.Lead.AuthorID != 2 || second.Lead.AuthorID != 2 {
		t.Fatal("lead not retained")
	}
	if len(first.SummaryParts) != len(second.SummaryParts) || first.SummaryParts[0] != second.SummaryParts[0] {
		t.Fatal("snapshot summary changed on replay")
	}
}

func TestPrepareFirstPublicationNoLeadSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	slots, reports, summaries := NewMockslotRepository(ctrl), NewMockreportRepository(ctrl), NewMocksummaryPreparationRepository(ctrl)
	slot := pubmodels.Slot{ID: uuid.New(), State: pubmodels.StatePlanned}
	slots.EXPECT().Create(gomock.Any(), int64(1), gomock.Any()).Return(slot, nil)
	slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(nil, nil)
	got, err := NewPrepareUseCase(slots, reports, summaries).Prepare(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.Lead != nil || len(got.SummaryParts) != 0 {
		t.Fatal("first publication should have no lead or summary")
	}
}

func TestPrepareRefusesUnknownSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	slots := NewMockslotRepository(ctrl)
	slot := pubmodels.Slot{ID: uuid.New(), State: pubmodels.StateUnknown}
	slots.EXPECT().Create(gomock.Any(), int64(1), gomock.Any()).Return(slot, nil)
	_, err := NewPrepareUseCase(slots, NewMockreportRepository(ctrl), NewMocksummaryPreparationRepository(ctrl)).Prepare(context.Background(), 1, time.Now())
	if err == nil {
		t.Fatal("unknown slot accepted")
	}
}

func storedSlot(slot pubmodels.Slot, leadID int64) pubmodels.Slot {
	slot.LeadUserID = &leadID
	return slot
}
