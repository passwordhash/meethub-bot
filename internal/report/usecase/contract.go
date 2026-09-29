package usecase

import (
	"context"

	"github.com/google/uuid"
	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	publicationmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	reportmodels "github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks_test.go -package=$GOPACKAGE

type channelReader interface {
	GetChannels(context.Context, channelmodels.GetChannelsFilters) ([]channelmodels.Channel, error)
}

type slotReader interface {
	GetSlots(context.Context, publicationmodels.GetSlotsFilters) ([]publicationmodels.Slot, error)
	BindLink(context.Context, uuid.UUID, int64, int64, int64, int64) (publicationmodels.Slot, error)
}

type reportWriter interface {
	SaveFirst(context.Context, reportmodels.Report) error
}
