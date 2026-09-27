package usecase

import (
	"context"
	"fmt"
	"time"

	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type SummaryDispatcherUseCase struct {
	slots    summarySlotRepository
	parts    summaryDeliveryRepository
	delivery summaryDelivery
}

func NewSummaryDispatcherUseCase(slots summarySlotRepository, parts summaryDeliveryRepository, delivery summaryDelivery) (*SummaryDispatcherUseCase, error) {
	if slots == nil || parts == nil || delivery == nil {
		return nil, fmt.Errorf("summary dispatcher dependencies are required")
	}
	return &SummaryDispatcherUseCase{slots: slots, parts: parts, delivery: delivery}, nil
}

func (u *SummaryDispatcherUseCase) Recover(ctx context.Context) error {
	return u.delivery.RecoverAttempting(ctx)
}

func (u *SummaryDispatcherUseCase) Tick(ctx context.Context) error {
	slots, err := u.slots.GetSlots(ctx, pubmodels.GetSlotsFilters{State: []pubmodels.State{pubmodels.StatePublished}})
	if err != nil {
		return fmt.Errorf("get published slots for summary delivery: %w", err)
	}
	for _, slot := range slots {
		if slot.LinkedMessageID == nil || *slot.LinkedMessageID <= 0 {
			if slot.PublishedAt != nil && slot.LinkAlertedAt == nil && time.Since(*slot.PublishedAt) >= 5*time.Minute {
				if err := u.delivery.NotifyLinkMissing(ctx, slot.ChannelID); err != nil {
					return fmt.Errorf("notify missing summary link for slot %s: %w", slot.ID, err)
				}
				if err := u.slots.MarkLinkAlerted(ctx, slot.ID); err != nil {
					return fmt.Errorf("mark missing summary link alert for slot %s: %w", slot.ID, err)
				}
			}
			continue
		}
		id := slot.ID
		parts, err := u.parts.GetSummaryParts(ctx, pubmodels.GetSummaryPartsFilters{EditionSlotID: &id})
		if err != nil {
			return fmt.Errorf("get summary parts for slot %s: %w", id, err)
		}
		pending := false
		for _, part := range parts {
			if part.Status == "pending" {
				pending = true
				break
			}
		}
		if pending {
			if err := u.delivery.DeliverSummary(ctx, id); err != nil {
				return fmt.Errorf("deliver summary for slot %s: %w", id, err)
			}
		}
	}
	return nil
}
