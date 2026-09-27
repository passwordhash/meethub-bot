package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

var ErrSummaryNotReady = errors.New("published post is not linked yet")

type SummaryDeliveryUseCase struct {
	slots    summarySlotRepository
	parts    summaryDeliveryRepository
	channels attemptChannelRepository
	telegram summaryTelegramClient
	notifier summaryNotifier
	ownerID  int64
}

func NewSummaryDeliveryUseCase(slots summarySlotRepository, parts summaryDeliveryRepository, channels attemptChannelRepository, telegram summaryTelegramClient, notifier summaryNotifier, ownerID int64) (*SummaryDeliveryUseCase, error) {
	if slots == nil || parts == nil || channels == nil || telegram == nil || notifier == nil {
		return nil, errors.New("summary delivery dependencies are required")
	}
	return &SummaryDeliveryUseCase{slots: slots, parts: parts, channels: channels, telegram: telegram, notifier: notifier, ownerID: ownerID}, nil
}

func (u *SummaryDeliveryUseCase) DeliverSummary(ctx context.Context, editionSlotID uuid.UUID) error {
	id := editionSlotID
	slots, err := u.slots.GetSlots(ctx, pubmodels.GetSlotsFilters{ID: &id})
	if err != nil {
		return fmt.Errorf("get summary edition slot: %w", err)
	}
	if len(slots) != 1 || slots[0].State != pubmodels.StatePublished {
		return errors.New("summary edition slot is not published")
	}
	slot := slots[0]
	if slot.LinkedMessageID == nil {
		return errors.Join(ErrSummaryNotReady, u.notifyProblem(ctx, slot.ChannelID))
	}
	parts, err := u.parts.GetSummaryParts(ctx, pubmodels.GetSummaryPartsFilters{EditionSlotID: &id})
	if err != nil {
		return fmt.Errorf("get summary parts: %w", err)
	}
	if len(parts) == 0 {
		return nil
	}
	channelID := slot.ChannelID
	channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{ChannelID: &channelID})
	if err != nil {
		return fmt.Errorf("get configured summary channel: %w", err)
	}
	if len(channels) != 1 {
		return errors.New("configured summary channel not found")
	}
	permissions, err := u.telegram.VerifyChannel(ctx, channelID, u.ownerID)
	if err != nil {
		return errors.Join(fmt.Errorf("verify summary channel and group: %w", err), u.notifyProblem(ctx, channelID))
	}
	if err := permissions.Validate(channelID); err != nil || permissions.LinkedGroupID != channels[0].GroupID {
		return errors.Join(errors.New("summary channel permissions or linked group do not match publication"), u.notifyProblem(ctx, channelID))
	}
	if slot.ChannelMessageID == nil || *slot.ChannelMessageID <= 0 {
		return errors.Join(errors.New("published channel message ID is missing"), u.notifyProblem(ctx, channelID))
	}
	for _, part := range parts {
		switch part.Status {
		case "sent":
			continue
		case "unknown":
			return errors.Join(errors.New("summary delivery outcome needs owner attention"), u.notifyProblem(ctx, channelID))
		case "pending":
		default:
			return fmt.Errorf("summary part has unsupported status %q", part.Status)
		}
		if part.Text == "" {
			return errors.Join(errors.New("summary part expired"), u.notifyProblem(ctx, channelID))
		}
		if err := u.parts.Begin(ctx, id, part.PartIndex); err != nil {
			return fmt.Errorf("claim summary part: %w", err)
		}
		messageID, sendErr := u.telegram.SendReply(ctx, channels[0].GroupID, *slot.LinkedMessageID, part.Text)
		if sendErr != nil {
			if errors.Is(sendErr, channelmodels.ErrTelegramRejected) {
				if err := u.parts.Rejected(ctx, id, part.PartIndex); err != nil {
					return errors.Join(fmt.Errorf("record rejected summary part: %w", err), u.notifyProblem(ctx, channelID))
				}
				return errors.Join(errors.New("telegram rejected summary part"), u.notifyProblem(ctx, channelID))
			}
			if err := u.parts.Unknown(ctx, id, part.PartIndex); err != nil {
				return errors.Join(fmt.Errorf("record uncertain summary part: %w", err), u.notifyProblem(ctx, channelID))
			}
			return errors.Join(errors.New("summary part delivery outcome is uncertain"), u.notifyProblem(ctx, channelID))
		}
		if messageID <= 0 {
			if err := u.parts.Unknown(ctx, id, part.PartIndex); err != nil {
				return errors.Join(fmt.Errorf("record summary part without message ID: %w", err), u.notifyProblem(ctx, channelID))
			}
			return errors.Join(errors.New("summary part returned no message ID"), u.notifyProblem(ctx, channelID))
		}
		if err := u.parts.Sent(ctx, id, part.PartIndex, messageID); err != nil {
			return errors.Join(fmt.Errorf("summary sent but durable state update failed: %w", err), u.notifyProblem(ctx, channelID))
		}
	}
	return nil
}

func (u *SummaryDeliveryUseCase) NotifyLinkMissing(ctx context.Context, channelID int64) error {
	if err := u.notifier.NotifySummaryProblem(ctx, channelID); err != nil {
		return errors.New("summary owner notification failed")
	}
	return nil
}

func (u *SummaryDeliveryUseCase) RecoverAttempting(ctx context.Context) error {
	channelIDs, err := u.parts.ListAttemptingChannels(ctx)
	if err != nil {
		return fmt.Errorf("list channels with attempting summary parts: %w", err)
	}
	for _, channelID := range channelIDs {
		if err := u.notifier.NotifySummaryProblem(ctx, channelID); err != nil {
			return errors.New("summary owner notification failed")
		}
	}
	if err := u.parts.RecoverAttempting(ctx); err != nil {
		return fmt.Errorf("recover attempting summary parts: %w", err)
	}
	return nil
}

func (u *SummaryDeliveryUseCase) notifyProblem(ctx context.Context, channelID int64) error {
	if err := u.notifier.NotifySummaryProblem(ctx, channelID); err != nil {
		return errors.New("summary owner notification failed")
	}
	return nil
}
