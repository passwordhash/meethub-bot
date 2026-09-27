package usecase

import (
	"context"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"go.uber.org/mock/gomock"
)

func TestUISetScheduleFromButtonAndMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockChannelRepository(ctrl)
	telegram := NewMockTelegramClient(ctrl)
	channelID := int64(-100123)
	schedule := models.Schedule{ChannelID: channelID, LeadEnabled: true}
	repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return([]models.Schedule{schedule}, nil)
	repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: &channelID}).Return([]models.Schedule{schedule}, nil).Times(2)
	repository.EXPECT().SetSchedule(gomock.Any(), models.Schedule{ChannelID: channelID, Cron: "*/5 * * * *", LeadEnabled: true}, false).Return(nil)
	telegram.EXPECT().AnswerCallback(gomock.Any(), "query-1", "").Return(nil)
	telegram.EXPECT().AnswerCallback(gomock.Any(), "query-2", "").Return(nil)
	telegram.EXPECT().SendKeyboard(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Any()).Return(nil)

	uc := NewChannelUseCase(1, repository, telegram, nil)
	if err := uc.handle(context.Background(), models.Update{Kind: "callback_query", CallbackQueryID: "query-1", CallbackData: "ui:schedule", ChatID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := uc.handle(context.Background(), models.Update{Kind: "callback_query", CallbackQueryID: "query-2", CallbackData: "ui:schedule:-100123", ChatID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := uc.handle(context.Background(), models.Update{Kind: "message", ChatID: 1, Text: "*/5 * * * *"}); err != nil {
		t.Fatal(err)
	}
}

func TestUICommandCancelsPendingCronInput(t *testing.T) {
	uc := NewChannelUseCase(1, nil, nil, nil)
	uc.pendingCron[1] = -100123
	if err := uc.handle(context.Background(), models.Update{Kind: "message", ChatID: 1, Text: "/unknown"}); err != nil {
		t.Fatal(err)
	}
	if _, pending := uc.pendingCron[1]; pending {
		t.Fatal("pending cron input was not cancelled by a command")
	}
	if err := uc.handle(context.Background(), models.Update{Kind: "message", ChatID: 1, Text: "*/5 * * * *"}); err != nil {
		t.Fatal(err)
	}
}

func TestUILeadToggle(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		current bool
		want    bool
	}{
		{name: "turn on", data: "ui:lead:-100123:on", current: false, want: true},
		{name: "turn off", data: "ui:lead:-100123:off", current: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMockChannelRepository(ctrl)
			telegram := NewMockTelegramClient(ctrl)
			channelID := int64(-100123)
			repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: &channelID}).Return([]models.Schedule{{ChannelID: channelID, LeadEnabled: tt.current}}, nil)
			repository.EXPECT().SetLeadEnabled(gomock.Any(), channelID, tt.want).Return(nil)
			repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return([]models.Schedule{{ChannelID: channelID, LeadEnabled: tt.want}}, nil)
			telegram.EXPECT().AnswerCallback(gomock.Any(), "query-1", "").Return(nil)
			telegram.EXPECT().SendKeyboard(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).Return(nil)
			uc := NewChannelUseCase(1, repository, telegram, nil)
			if err := uc.handle(context.Background(), models.Update{Kind: "callback_query", CallbackQueryID: "query-1", CallbackData: tt.data, ChatID: 1}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
