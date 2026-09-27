package usecase

import (
	"context"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks_test.go -package=$GOPACKAGE

type ChannelRepository interface {
	Create(context.Context, models.Channel) (models.Channel, error)
	GetSchedules(context.Context, models.GetSchedulesFilters) ([]models.Schedule, error)
	SetSchedule(context.Context, models.Schedule, bool) error
	SetLeadEnabled(context.Context, int64, bool) error
	SetOverride(context.Context, int64, time.Time) error
}
type updateProcessor interface {
	GetOffset(context.Context) (int64, error)
	Process(context.Context, models.Update) error
}

type TelegramClient interface {
	VerifyChannel(context.Context, int64, int64) (models.Permissions, error)
	Updates(context.Context, int64) ([]models.Update, error)
	Send(context.Context, int64, string) error
	SendKeyboard(context.Context, int64, string, [][]models.InlineButton) error
	AnswerCallback(context.Context, string, string) error
}
