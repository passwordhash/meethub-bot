package usecase

import (
	"context"
	"fmt"
	"time"

	pubdomain "github.com/passwordhash/cascade-hub-bot/internal/publication/domain"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	reportdomain "github.com/passwordhash/cascade-hub-bot/internal/report/domain"
	reportmodels "github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

type PrepareUseCase struct {
	slots     slotRepository
	reports   reportRepository
	summaries summaryPreparationRepository
}

type Prepared struct {
	Slot         pubmodels.Slot
	Lead         *reportmodels.Report
	SummaryParts []string
}

func NewPrepareUseCase(slots slotRepository, reports reportRepository, summaries summaryPreparationRepository) *PrepareUseCase {
	return &PrepareUseCase{slots: slots, reports: reports, summaries: summaries}
}

func (u *PrepareUseCase) Prepare(ctx context.Context, channelID int64, scheduledAt time.Time) (Prepared, error) {
	slot, err := u.slots.Create(ctx, channelID, scheduledAt)
	if err != nil {
		return Prepared{}, err
	}
	if slot.State != pubmodels.StatePlanned {
		return Prepared{}, fmt.Errorf("publication slot is %s, preparation refused", slot.State)
	}
	history, err := u.slots.GetSlots(ctx, pubmodels.GetSlotsFilters{ChannelID: &channelID, State: []pubmodels.State{pubmodels.StatePublished}})
	if err != nil {
		return Prepared{}, err
	}
	var source *pubmodels.Slot
	for i := range history {
		if history[i].PublishedAt != nil && (source == nil || history[i].PublishedAt.After(*source.PublishedAt)) {
			source = &history[i]
		}
	}
	if source == nil {
		return Prepared{Slot: slot}, nil
	}
	if err = u.reports.CreateSnapshot(ctx, slot.ID, source.ID); err != nil {
		return Prepared{}, err
	}
	id := slot.ID
	snapshots, err := u.reports.GetSnapshots(ctx, reportmodels.GetSnapshotsFilters{EditionSlotID: &id})
	if err != nil {
		return Prepared{}, err
	}
	snapshotReports := make([]reportmodels.Report, 0, len(snapshots))
	for _, snapshot := range snapshots {
		snapshotReports = append(snapshotReports, reportmodels.Report{AuthorID: snapshot.AuthorID, Author: snapshot.Author, Text: snapshot.Text})
	}
	var lead *reportmodels.Report
	if slot.LeadUserID != nil {
		for i := range snapshotReports {
			if snapshotReports[i].AuthorID == *slot.LeadUserID {
				lead = &snapshotReports[i]
				break
			}
		}
	} else {
		lead, err = SelectLead(snapshotReports, previousLead(source))
		if err != nil {
			return Prepared{}, err
		}
		var leadID *int64
		if lead != nil {
			value := lead.AuthorID
			leadID = &value
		}
		slot, err = u.slots.SetLead(ctx, slot.ID, leadID)
		if err != nil {
			return Prepared{}, err
		}
		if slot.LeadUserID != nil {
			lead = nil
			for i := range snapshotReports {
				if snapshotReports[i].AuthorID == *slot.LeadUserID {
					lead = &snapshotReports[i]
					break
				}
			}
		}
	}
	parts := pubdomain.RenderSummary(reportdomain.ExtractPlans(snapshotReports))
	if len(parts) > 0 {
		if err = u.summaries.CreateSummaryParts(ctx, slot.ID, parts); err != nil {
			return Prepared{}, err
		}
	}
	return Prepared{Slot: slot, Lead: lead, SummaryParts: parts}, nil
}

func previousLead(source *pubmodels.Slot) int64 {
	if source.LeadUserID == nil {
		return 0
	}
	return *source.LeadUserID
}
