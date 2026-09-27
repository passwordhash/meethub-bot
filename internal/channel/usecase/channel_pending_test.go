package usecase

import (
	"context"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"go.uber.org/mock/gomock"
)

func TestRunCancelsWhileUpdatePending(t *testing.T) {
	controller := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	processor := NewMockupdateProcessor(controller)
	telegram := NewMockTelegramClient(controller)
	processor.EXPECT().GetOffset(gomock.Any()).Return(int64(8), nil)
	telegram.EXPECT().Updates(gomock.Any(), int64(8)).Return([]models.Update{{ID: 8}}, nil)
	processor.EXPECT().Process(gomock.Any(), models.Update{ID: 8}).DoAndReturn(func(context.Context, models.Update) error {
		cancel()
		return models.ErrUpdatePending
	})

	channel := NewChannelUseCase(1, nil, telegram, processor)
	if err := channel.Run(ctx); err != nil {
		t.Fatal(err)
	}
}
