package usecase

import (
	"context"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks_test.go -package=$GOPACKAGE

type ChannelRepository interface {
	Create(context.Context, models.Channel) (models.Channel, error)
	GetSchedules(context.Context, models.GetSchedulesFilters) ([]models.Schedule, error)
	SetDailyTime(context.Context, int64, time.Weekday, time.Duration) error
	DeleteDailyTime(context.Context, int64, time.Weekday) error
	SetPostOffset(context.Context, int64, int) error
	SetLeadEnabled(context.Context, int64, bool) error
	ClearScheduleReset(context.Context, int64) error
	GetIgnoredUsers(context.Context, models.GetIgnoredUsersFilters) ([]models.IgnoredUser, error)
	AddIgnoredUser(context.Context, int64, int64) error
	DeleteIgnoredUser(context.Context, int64, int64) error
	GetParticipants(context.Context, models.GetParticipantsFilters) ([]models.Participant, error)
	SetParticipant(context.Context, models.Participant) error
	DeleteParticipant(context.Context, int64, string) error
}
type slotRepository interface {
	GetSlots(context.Context, pubmodels.GetSlotsFilters) ([]pubmodels.Slot, error)
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
