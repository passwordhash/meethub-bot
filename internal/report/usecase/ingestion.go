package usecase

import (
	"context"
	"strings"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	reportmodels "github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

type Ingestion struct {
	channels channelReader
	slots    slotReader
	reports  reportWriter
}

func NewIngestion(channels channelReader, slots slotReader, reports reportWriter) *Ingestion {
	return &Ingestion{channels: channels, slots: slots, reports: reports}
}

func (u *Ingestion) Handle(ctx context.Context, update channelmodels.Update) error {
	if linkedForward(update) {
		channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{ChannelID: &update.ForwardChatID})
		if err != nil {
			return err
		}
		for _, channel := range channels {
			if channel.ChannelID != update.ForwardChatID || channel.GroupID != update.ChatID {
				continue
			}
			attemptingSlots, err := u.slots.GetSlots(ctx, models.GetSlotsFilters{ChannelID: &channel.ChannelID, State: []models.State{models.StateAttempting}})
			if err != nil {
				return err
			}
			if len(attemptingSlots) > 0 {
				return channelmodels.ErrUpdatePending
			}
			slots, err := u.slots.GetSlots(ctx, models.GetSlotsFilters{ChannelID: &channel.ChannelID, State: []models.State{models.StatePublished}})
			if err != nil {
				return err
			}
			for _, slot := range slots {
				if slot.ChannelMessageID != nil && *slot.ChannelMessageID == update.ForwardMessageID {
					_, err = u.slots.BindLink(ctx, slot.ID, channel.ChannelID, channel.GroupID, update.ForwardMessageID, update.MessageID)
					return err
				}
			}
		}
		return nil
	}
	if !eligibleReply(update) {
		return nil
	}
	channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{GroupID: &update.ChatID})
	if err != nil {
		return err
	}
	if len(channels) != 1 || channels[0].GroupID != update.ChatID {
		return nil
	}
	slots, err := u.slots.GetSlots(ctx, models.GetSlotsFilters{ChannelID: &channels[0].ChannelID, State: []models.State{models.StatePublished}})
	if err != nil {
		return err
	}
	slot, ok := latest(slots)
	report, eligible := reportFromUpdate(update, channels[0], slot)
	if !ok || !eligible {
		return nil
	}
	ignored, err := u.channels.GetIgnoredUsers(ctx, channelmodels.GetIgnoredUsersFilters{ChannelID: &channels[0].ChannelID, UserID: &report.AuthorID})
	if err != nil {
		return err
	}
	if len(ignored) > 0 {
		return nil
	}
	return u.reports.SaveFirst(ctx, report)
}

func linkedForward(update channelmodels.Update) bool {
	return update.IsAutomaticForward && update.Kind == "message" && update.ForwardOriginType == "channel" && update.ForwardChatID != 0 && update.ForwardMessageID > 0 && update.ChatID != 0 && update.MessageID > 0 && (update.ChatType == "group" || update.ChatType == "supergroup")
}

func eligibleReply(update channelmodels.Update) bool {
	return (update.Kind == "message" || update.Kind == "edited_message") && update.ChatID != 0 && (update.ChatType == "group" || update.ChatType == "supergroup") && update.FromID > 0 && !update.SenderIsBot && update.SenderChatID == 0 && update.ReplyToMessageID > 0 && update.MessageID > 0 && update.Date > 0 && (strings.TrimSpace(update.Text) != "" || strings.TrimSpace(update.Caption) != "")
}

func reportFromUpdate(update channelmodels.Update, channel channelmodels.Channel, slot models.Slot) (reportmodels.Report, bool) {
	if !eligibleReply(update) || channel.GroupID != update.ChatID || slot.ChannelID != channel.ChannelID || slot.State != models.StatePublished || slot.PublishedAt == nil || slot.LinkedMessageID == nil || *slot.LinkedMessageID != update.ReplyToMessageID {
		return reportmodels.Report{}, false
	}
	text := update.Text
	if strings.TrimSpace(text) == "" {
		text = update.Caption
	}
	report := reportmodels.Report{SlotID: slot.ID, AuthorID: update.FromID, Author: update.FromName, Text: text, MessageID: update.MessageID, MessageAt: time.Unix(update.Date, 0)}
	if update.EditDate > 0 {
		edited := time.Unix(update.EditDate, 0)
		report.EditAt = &edited
	}
	return report, true
}

func latest(slots []models.Slot) (models.Slot, bool) {
	var result models.Slot
	for _, slot := range slots {
		if slot.PublishedAt == nil {
			continue
		}
		if result.PublishedAt == nil || slot.PublishedAt.After(*result.PublishedAt) {
			result = slot
		}
	}
	return result, result.PublishedAt != nil
}
