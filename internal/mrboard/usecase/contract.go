package usecase

import (
	"context"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks_test.go -package=$GOPACKAGE

type gitLabClient interface {
	OpenMergeRequests(context.Context) ([]models.MergeRequest, error)
}

type channelRepository interface {
	GetChannels(context.Context, channelmodels.GetChannelsFilters) ([]channelmodels.Channel, error)
	GetParticipants(context.Context, channelmodels.GetParticipantsFilters) ([]channelmodels.Participant, error)
}

type boardRepository interface {
	GetBoards(context.Context, models.GetBoardsFilters) ([]models.Board, error)
	SaveBoard(context.Context, models.Board) error
	GetTracks(context.Context) ([]models.ReviewerTrack, []models.SectionTrack, error)
	SaveTracks(context.Context, []models.ReviewerTrack, []models.SectionTrack) error
}

type boardClient interface {
	CanPin(context.Context, int64) (bool, error)
	SendBoard(context.Context, int64, string) (int64, error)
	EditBoard(context.Context, int64, int64, string) error
	PinBoard(context.Context, int64, int64) error
	PinnedMessageID(context.Context, int64) (int64, error)
}

type boardNotifier interface {
	NotifyBoardProblem(context.Context, int64) error
}
