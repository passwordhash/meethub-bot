package usecase

import (
	"context"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"go.uber.org/mock/gomock"
)

func TestDailyScheduleCron(t *testing.T) {
	tests := []struct {
		name string
		args []string
		cron string
	}{
		{name: "five-field cron", args: []string{"-100123", "30", "9", "*", "*", "1,3,5"}, cron: "30 9 * * 1,3,5"},
		{name: "legacy weekdays and time", args: []string{"-100123", "1,3,5", "09:30"}, cron: "30 9 * * 1,3,5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMockChannelRepository(ctrl)
			telegram := NewMockTelegramClient(ctrl)
			channelID := int64(-100123)
			repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: &channelID}).Return([]models.Schedule{{ChannelID: channelID, LeadEnabled: true}}, nil)
			repository.EXPECT().SetSchedule(gomock.Any(), models.Schedule{ChannelID: channelID, Cron: tt.cron, LeadEnabled: true}, false).Return(nil)
			telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			uc := NewChannelUseCase(1, repository, telegram, nil)
			if err := uc.setDailySchedule(context.Background(), 1, tt.args); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDailyScheduleLeadToggle(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		want    bool
	}{
		{name: "enable random lead", setting: "on", want: true},
		{name: "disable random lead", setting: "off", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMockChannelRepository(ctrl)
			telegram := NewMockTelegramClient(ctrl)
			channelID := int64(-100123)
			repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: &channelID}).Return([]models.Schedule{{ChannelID: channelID}}, nil)
			repository.EXPECT().SetLeadEnabled(gomock.Any(), channelID, tt.want).Return(nil)
			telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			uc := NewChannelUseCase(1, repository, telegram, nil)
			if err := uc.setDailySchedule(context.Background(), 1, []string{"-100123", "lead", tt.setting}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
