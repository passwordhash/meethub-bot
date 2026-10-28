package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	publicationmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

func TestIngestionHandlesAutoForward(t *testing.T) {
	published := time.Unix(1700000000, 0)
	slotID := uuid.UUID{1}
	otherSlotID := uuid.UUID{2}
	messageID := int64(21)
	latestMessageID := int64(22)
	channel := channelmodels.Channel{ChannelID: -1001, GroupID: -2002}
	update := channelmodels.Update{Kind: "message", IsAutomaticForward: true, ForwardOriginType: "channel", ForwardChatID: channel.ChannelID, ForwardMessageID: messageID, ChatID: channel.GroupID, ChatType: "supergroup", MessageID: 31}
	publishedSlot := publicationmodels.Slot{ID: slotID, ChannelID: channel.ChannelID, State: publicationmodels.StatePublished, ChannelMessageID: &messageID, PublishedAt: &published}
	latestSlot := publicationmodels.Slot{ID: otherSlotID, ChannelID: channel.ChannelID, State: publicationmodels.StatePublished, ChannelMessageID: &latestMessageID, PublishedAt: &published}
	tests := []struct {
		name        string
		update      channelmodels.Update
		channels    []channelmodels.Channel
		published   []publicationmodels.Slot
		attempting  []publicationmodels.Slot
		wantErr     error
		wantBind    bool
		wantLookups int
	}{
		{name: "pending publication", update: update, channels: []channelmodels.Channel{channel}, attempting: []publicationmodels.Slot{{ChannelID: channel.ChannelID, State: publicationmodels.StateAttempting}}, wantErr: channelmodels.ErrUpdatePending, wantLookups: 1},
		{name: "old published forward matches exact message", update: update, channels: []channelmodels.Channel{channel}, published: []publicationmodels.Slot{publishedSlot, latestSlot}, wantBind: true, wantLookups: 2},
		{name: "publication completes between attempting and published lookups", update: update, channels: []channelmodels.Channel{channel}, published: []publicationmodels.Slot{publishedSlot}, wantBind: true, wantLookups: 2},
		{name: "unknown publication passes", update: update, channels: []channelmodels.Channel{channel}, wantLookups: 2},
		{name: "unrelated group passes", update: withChat(update, -2999), channels: []channelmodels.Channel{channel}, attempting: []publicationmodels.Slot{{ChannelID: channel.ChannelID, State: publicationmodels.StateAttempting}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			channels := NewMockchannelReader(controller)
			slots := NewMockslotReader(controller)
			reports := NewMockreportWriter(controller)
			channels.EXPECT().GetChannels(gomock.Any(), gomock.Any()).Return(tt.channels, nil)
			lookup := 0
			slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Times(tt.wantLookups).DoAndReturn(func(_ context.Context, filters publicationmodels.GetSlotsFilters) ([]publicationmodels.Slot, error) {
				lookup++
				if lookup == 1 && (len(filters.State) != 1 || filters.State[0] != publicationmodels.StateAttempting) {
					t.Fatalf("first slot lookup state = %v, want attempting", filters.State)
				}
				if lookup == 2 && (len(filters.State) != 1 || filters.State[0] != publicationmodels.StatePublished) {
					t.Fatalf("second slot lookup state = %v, want published", filters.State)
				}
				if len(filters.State) == 1 && filters.State[0] == publicationmodels.StateAttempting {
					return tt.attempting, nil
				}
				return tt.published, nil
			}).AnyTimes()
			if tt.wantBind {
				slots.EXPECT().BindLink(gomock.Any(), slotID, channel.ChannelID, channel.GroupID, update.ForwardMessageID, update.MessageID).Return(publicationmodels.Slot{}, nil)
			}
			ingestion := NewIngestion(channels, slots, reports)
			err := ingestion.Handle(context.Background(), tt.update)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Handle() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIngestionSkipsIgnoredAuthors(t *testing.T) {
	publishedAt := time.Unix(1700000010, 0)
	linkedMessageID := int64(301)
	channel := channelmodels.Channel{ChannelID: -1001, GroupID: -2002}
	slot := publicationmodels.Slot{ID: uuid.UUID{1}, ChannelID: channel.ChannelID, State: publicationmodels.StatePublished, LinkedMessageID: &linkedMessageID, PublishedAt: &publishedAt}
	update := channelmodels.Update{Kind: "message", ChatID: channel.GroupID, ChatType: "supergroup", FromID: 42, FromName: "Grace", MessageID: 400, ReplyToMessageID: linkedMessageID, Date: 1700000000, Text: "report"}
	tests := []struct {
		name     string
		ignored  []channelmodels.IgnoredUser
		wantSave bool
	}{
		{name: "regular author is saved", wantSave: true},
		{name: "ignored author is dropped", ignored: []channelmodels.IgnoredUser{{ChannelID: channel.ChannelID, UserID: 42}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			channels := NewMockchannelReader(controller)
			slots := NewMockslotReader(controller)
			reports := NewMockreportWriter(controller)
			channels.EXPECT().GetChannels(gomock.Any(), channelmodels.GetChannelsFilters{GroupID: &channel.GroupID}).Return([]channelmodels.Channel{channel}, nil)
			slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return([]publicationmodels.Slot{slot}, nil)
			channels.EXPECT().GetIgnoredUsers(gomock.Any(), channelmodels.GetIgnoredUsersFilters{ChannelID: &channel.ChannelID, UserID: &update.FromID}).Return(tt.ignored, nil)
			if tt.wantSave {
				reports.EXPECT().SaveFirst(gomock.Any(), gomock.Any()).Return(nil)
			}
			if err := NewIngestion(channels, slots, reports).Handle(context.Background(), update); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportFromUpdate(t *testing.T) {
	publishedAt := time.Unix(1700000010, 0)
	linkedMessageID := int64(301)
	channel := channelmodels.Channel{ChannelID: -1001, GroupID: -2002}
	slot := publicationmodels.Slot{ChannelID: channel.ChannelID, State: publicationmodels.StatePublished, LinkedMessageID: &linkedMessageID, PublishedAt: &publishedAt}
	base := channelmodels.Update{Kind: "message", ChatID: channel.GroupID, ChatType: "supergroup", FromID: 42, FromName: "Grace", MessageID: 400, ReplyToMessageID: linkedMessageID, Date: 1700000000, Text: "report"}
	tests := []struct {
		name       string
		update     channelmodels.Update
		channel    channelmodels.Channel
		slot       publicationmodels.Slot
		wantReport bool
		wantEdit   int64
	}{
		{name: "connected group direct reply", update: base, channel: channel, slot: slot, wantReport: true},
		{name: "unconnected group", update: withChat(base, -2999), channel: channel, slot: slot},
		{name: "old daily reply", update: base, channel: channel, slot: withLink(slot, 300)},
		{name: "nested reply", update: withReply(base, 302), channel: channel, slot: slot},
		{name: "bot sender", update: withBot(base), channel: channel, slot: slot},
		{name: "anonymous sender", update: withSenderChat(base), channel: channel, slot: slot},
		{name: "edited direct reply", update: withEdit(base), channel: channel, slot: slot, wantReport: true, wantEdit: 1700000005},
		{name: "empty text and caption", update: withText(base, ""), channel: channel, slot: slot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, ok := reportFromUpdate(tt.update, tt.channel, tt.slot)
			if ok != tt.wantReport {
				t.Fatalf("eligible = %v, want %v", ok, tt.wantReport)
			}
			if !ok {
				return
			}
			if report.AuthorID != 42 || report.MessageID != 400 || report.Text != tt.update.Text {
				t.Fatalf("report = %+v", report)
			}
			if tt.wantEdit == 0 && report.EditAt != nil || tt.wantEdit != 0 && (report.EditAt == nil || report.EditAt.Unix() != tt.wantEdit) {
				t.Fatalf("edit time = %v, want %d", report.EditAt, tt.wantEdit)
			}
		})
	}
}

func TestLinkedForward(t *testing.T) {
	valid := channelmodels.Update{Kind: "message", IsAutomaticForward: true, ForwardOriginType: "channel", ForwardChatID: -1001, ForwardMessageID: 21, ChatID: -2002, ChatType: "supergroup", MessageID: 31}
	tests := []struct {
		name   string
		update channelmodels.Update
		want   bool
	}{
		{name: "automatic channel post forward", update: valid, want: true},
		{name: "not automatic forward", update: withAutomatic(valid, false)},
		{name: "forwarded channel post but not automatic", update: withAutomatic(valid, false)},
		{name: "missing source channel", update: withForwardChat(valid, 0)},
		{name: "missing source post id", update: withForwardMessage(valid, 0)},
		{name: "non-group destination", update: withChatType(valid, "channel")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linkedForward(tt.update); got != tt.want {
				t.Fatalf("linkedForward = %v, want %v", got, tt.want)
			}
		})
	}
}

func withChat(update channelmodels.Update, id int64) channelmodels.Update {
	update.ChatID = id
	return update
}
func withReply(update channelmodels.Update, id int64) channelmodels.Update {
	update.ReplyToMessageID = id
	return update
}
func withBot(update channelmodels.Update) channelmodels.Update {
	update.SenderIsBot = true
	return update
}
func withSenderChat(update channelmodels.Update) channelmodels.Update {
	update.SenderChatID = -2002
	return update
}
func withEdit(update channelmodels.Update) channelmodels.Update {
	update.Kind = "edited_message"
	update.EditDate = 1700000005
	return update
}
func withText(update channelmodels.Update, text string) channelmodels.Update {
	update.Text = text
	return update
}
func withLink(slot publicationmodels.Slot, id int64) publicationmodels.Slot {
	slot.LinkedMessageID = &id
	return slot
}
func withAutomatic(update channelmodels.Update, automatic bool) channelmodels.Update {
	update.IsAutomaticForward = automatic
	return update
}
func withForwardChat(update channelmodels.Update, id int64) channelmodels.Update {
	update.ForwardChatID = id
	return update
}
func withForwardMessage(update channelmodels.Update, id int64) channelmodels.Update {
	update.ForwardMessageID = id
	return update
}
func withChatType(update channelmodels.Update, chatType string) channelmodels.Update {
	update.ChatType = chatType
	return update
}
