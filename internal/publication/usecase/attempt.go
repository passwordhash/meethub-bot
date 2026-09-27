package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubdomain "github.com/passwordhash/cascade-hub-bot/internal/publication/domain"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type AttemptUseCase struct {
	slots      attemptSlotRepository
	channels   attemptChannelRepository
	telegram   postingClient
	notifier   publicationNotifier
	ownerID    int64
	meetingURL string
}

func NewAttemptUseCase(slots attemptSlotRepository, channels attemptChannelRepository, client postingClient, notifier publicationNotifier, ownerID int64, meetingURL string) (*AttemptUseCase, error) {
	if slots == nil || channels == nil || client == nil || notifier == nil {
		return nil, errors.New("publication attempt dependencies are required")
	}
	return &AttemptUseCase{slots: slots, channels: channels, telegram: client, notifier: notifier, ownerID: ownerID, meetingURL: meetingURL}, nil
}

func (u *AttemptUseCase) Attempt(ctx context.Context, prepared Prepared) (pubmodels.Slot, error) {
	slot := prepared.Slot
	if slot.State != pubmodels.StatePlanned {
		return slot, fmt.Errorf("publication slot is %s", slot.State)
	}
	parsed, err := url.Parse(u.meetingURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return slot, errors.New("valid HTTPS meeting URL is required")
	}
	channelID := slot.ChannelID
	channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{ChannelID: &channelID})
	if err != nil {
		return slot, fmt.Errorf("get configured publication channel: %w", err)
	}
	if len(channels) != 1 {
		return slot, errors.New("configured publication channel not found")
	}
	permissions, err := u.telegram.VerifyChannel(ctx, channelID, u.ownerID)
	if err != nil {
		return slot, errors.Join(fmt.Errorf("verify publication channel: %w", err), u.notifyOwner(ctx, channelID))
	}
	if err := permissions.Validate(channelID); err != nil || permissions.LinkedGroupID != channels[0].GroupID {
		return slot, errors.Join(errors.New("publication channel permissions or linked group do not match configuration"), u.notifyOwner(ctx, channelID))
	}
	attempting, err := u.slots.Transition(ctx, slot.ID, pubmodels.StatePlanned, pubmodels.StateAttempting, nil, nil, slot.LeadUserID)
	if err != nil {
		return slot, fmt.Errorf("claim publication slot: %w", err)
	}
	messageID, sendErr := u.telegram.SendPost(ctx, channelID, pubdomain.RenderDaily(slot.ScheduledAt, prepared.Lead, true, u.meetingURL))
	if sendErr == nil && messageID > 0 {
		published, transitionErr := u.slots.Transition(ctx, slot.ID, pubmodels.StateAttempting, pubmodels.StatePublished, &messageID, nil, slot.LeadUserID)
		if transitionErr != nil {
			return attempting, errors.Join(fmt.Errorf("publication sent but durable state update failed: %w", transitionErr), u.notifyOwner(ctx, channelID))
		}
		return published, nil
	}
	unknown, transitionErr := u.slots.Transition(ctx, slot.ID, pubmodels.StateAttempting, pubmodels.StateUnknown, nil, nil, slot.LeadUserID)
	if transitionErr != nil {
		return attempting, fmt.Errorf("publication outcome unresolved and state update failed: %w", transitionErr)
	}
	return unknown, errors.Join(errors.New("publication outcome needs owner attention"), u.notifyOwner(ctx, channelID))
}

func (u *AttemptUseCase) notifyOwner(ctx context.Context, channelID int64) error {
	if err := u.notifier.NotifyPublicationProblem(ctx, channelID); err != nil {
		return errors.New("owner notification failed")
	}
	return nil
}
