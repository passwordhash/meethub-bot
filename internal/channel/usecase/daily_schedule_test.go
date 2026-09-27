package usecase

import (
	"context"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"go.uber.org/mock/gomock"
)

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
