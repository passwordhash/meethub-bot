package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	reportmodels "github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks_test.go -package=$GOPACKAGE

type slotRepository interface {
	Create(context.Context, int64, time.Time) (pubmodels.Slot, error)
	GetSlots(context.Context, pubmodels.GetSlotsFilters) ([]pubmodels.Slot, error)
	SetLead(context.Context, uuid.UUID, *int64) (pubmodels.Slot, error)
}
type reportRepository interface {
	GetReports(context.Context, reportmodels.GetReportsFilters) ([]reportmodels.Report, error)
	CreateSnapshot(context.Context, uuid.UUID, uuid.UUID) error
	GetSnapshots(context.Context, reportmodels.GetSnapshotsFilters) ([]reportmodels.Snapshot, error)
}
type scheduleRepository interface {
	GetSchedules(context.Context, channelmodels.GetSchedulesFilters) ([]channelmodels.Schedule, error)
	SetScheduleCheckedAt(context.Context, int64, time.Time) error
}
type scheduleSlotRepository interface {
	Create(context.Context, int64, time.Time) (pubmodels.Slot, error)
	Transition(context.Context, uuid.UUID, pubmodels.State, pubmodels.State, *int64, *int64, *int64) (pubmodels.Slot, error)
	Recover(context.Context) ([]pubmodels.Slot, error)
}
type attemptSlotRepository interface {
	Transition(context.Context, uuid.UUID, pubmodels.State, pubmodels.State, *int64, *int64, *int64) (pubmodels.Slot, error)
}
type attemptChannelRepository interface {
	GetChannels(context.Context, channelmodels.GetChannelsFilters) ([]channelmodels.Channel, error)
}
type postingClient interface {
	VerifyChannel(context.Context, int64, int64) (channelmodels.Permissions, error)
	SendPost(context.Context, int64, string) (int64, error)
}
type publicationNotifier interface {
	NotifyPublicationProblem(context.Context, int64) error
}
type summaryPreparationRepository interface {
	CreateSummaryParts(context.Context, uuid.UUID, []string) error
}
type summaryDeliveryRepository interface {
	GetSummaryParts(context.Context, pubmodels.GetSummaryPartsFilters) ([]pubmodels.SummaryPart, error)
	Begin(context.Context, uuid.UUID, int) error
	Sent(context.Context, uuid.UUID, int, int64) error
	Rejected(context.Context, uuid.UUID, int) error
	Unknown(context.Context, uuid.UUID, int) error
	ListAttemptingChannels(context.Context) ([]int64, error)
	RecoverAttempting(context.Context) error
}
type summarySlotRepository interface {
	GetSlots(context.Context, pubmodels.GetSlotsFilters) ([]pubmodels.Slot, error)
	MarkLinkAlerted(context.Context, uuid.UUID) error
}
type summaryTelegramClient interface {
	VerifyChannel(context.Context, int64, int64) (channelmodels.Permissions, error)
	SendReply(context.Context, int64, int64, string) (int64, error)
}
type summaryNotifier interface {
	NotifySummaryProblem(context.Context, int64) error
}
type summaryDelivery interface {
	DeliverSummary(context.Context, uuid.UUID) error
	NotifyLinkMissing(context.Context, int64) error
	RecoverAttempting(context.Context) error
}
type lifecycleScheduler interface {
	Recover(context.Context) error
	Tick(context.Context, time.Time) error
}
type lifecycleDispatcher interface {
	Recover(context.Context) error
	Tick(context.Context) error
}
type lifecycleRetention interface {
	PurgeExpiredText(context.Context) error
	PurgeExpired(context.Context) error
}
