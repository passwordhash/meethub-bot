package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/domain"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
	"go.uber.org/mock/gomock"
)

const (
	channelID = int64(-100)
	groupID   = int64(-200)
)

var now = time.Date(2026, 10, 29, 11, 32, 0, 0, time.UTC)

type boardFixture struct {
	uc       *BoardUseCase
	gitlab   *MockgitLabClient
	channels *MockchannelRepository
	boards   *MockboardRepository
	telegram *MockboardClient
	notifier *MockboardNotifier
}

func newBoardFixture(t *testing.T) *boardFixture {
	ctrl := gomock.NewController(t)
	f := &boardFixture{gitlab: NewMockgitLabClient(ctrl), channels: NewMockchannelRepository(ctrl), boards: NewMockboardRepository(ctrl), telegram: NewMockboardClient(ctrl), notifier: NewMockboardNotifier(ctrl)}
	uc, err := NewBoardUseCase(f.gitlab, f.channels, f.boards, f.telegram, f.notifier, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	uc.now = func() time.Time { return now }
	f.uc = uc
	return f
}

var teamMR = models.MergeRequest{ProjectID: 1, IID: 406, Reference: "cascade!406", Title: "лимит", Author: "tema", UpdatedAt: now.Add(-time.Hour), Reviewers: []models.Reviewer{{Username: "yarik", State: models.StateUnreviewed}}}

func teamBody() string {
	_, sections := domain.Track([]models.MergeRequest{teamMR}, nil, nil, now)
	labels := map[string]string{"yarik": "@yarik"}
	return domain.RenderBody(domain.Items([]models.MergeRequest{teamMR}, nil, sections, labels, now), labels, now)
}

// expectPoll sets up a poll that finds teamMR and returns the board body it renders to.
func (f *boardFixture) expectPoll(boards []models.Board) string {
	f.gitlab.EXPECT().OpenMergeRequests(gomock.Any()).Return([]models.MergeRequest{teamMR}, nil)
	f.boards.EXPECT().GetTracks(gomock.Any()).Return(nil, nil, nil)
	f.boards.EXPECT().SaveTracks(gomock.Any(), gomock.Len(1), gomock.Len(1)).Return(nil)
	f.channels.EXPECT().GetChannels(gomock.Any(), channelmodels.GetChannelsFilters{}).Return([]channelmodels.Channel{{ChannelID: channelID, GroupID: groupID}, {ChannelID: -101, GroupID: -201}}, nil)
	f.channels.EXPECT().GetParticipants(gomock.Any(), channelmodels.GetParticipantsFilters{}).Return([]channelmodels.Participant{{ChannelID: channelID, GitLabUsername: "yarik", TelegramLabel: "@yarik"}}, nil)
	f.boards.EXPECT().GetBoards(gomock.Any(), models.GetBoardsFilters{}).Return(boards, nil)
	return teamBody()
}

func TestTickCreatesAndPinsBoard(t *testing.T) {
	f := newBoardFixture(t)
	body := f.expectPoll(nil)
	if !strings.Contains(body, "cascade!406 лимит — tema → @yarik · 1ч") {
		t.Fatalf("body = %q", body)
	}
	gomock.InOrder(
		f.telegram.EXPECT().CanPin(gomock.Any(), groupID).Return(true, nil),
		f.telegram.EXPECT().SendBoard(gomock.Any(), groupID, domain.Render(body, now)).Return(int64(77), nil),
		f.boards.EXPECT().SaveBoard(gomock.Any(), models.Board{ChannelID: channelID, MessageID: 77, Body: body, UpdatedAt: now}).Return(nil),
		f.telegram.EXPECT().PinBoard(gomock.Any(), groupID, int64(77)).Return(nil),
	)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTickWithoutPinRightNotifiesOnce(t *testing.T) {
	f := newBoardFixture(t)
	f.expectPoll(nil)
	f.telegram.EXPECT().CanPin(gomock.Any(), groupID).Return(false, nil).Times(2)
	f.notifier.EXPECT().NotifyBoardProblem(gomock.Any(), channelID).Return(nil)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.expectPoll(nil)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTickKeepsUnchangedPinnedBoard(t *testing.T) {
	f := newBoardFixture(t)
	f.expectPoll([]models.Board{{ChannelID: channelID, MessageID: 77, Body: teamBody(), UpdatedAt: now.Add(-time.Hour)}})
	f.telegram.EXPECT().PinnedMessageID(gomock.Any(), groupID).Return(int64(77), nil)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTickEditsChangedBoard(t *testing.T) {
	f := newBoardFixture(t)
	body := f.expectPoll([]models.Board{{ChannelID: channelID, MessageID: 77, Body: "old", UpdatedAt: now.Add(-time.Hour)}})
	gomock.InOrder(
		f.telegram.EXPECT().EditBoard(gomock.Any(), groupID, int64(77), domain.Render(body, now)).Return(nil),
		f.boards.EXPECT().SaveBoard(gomock.Any(), models.Board{ChannelID: channelID, MessageID: 77, Body: body, UpdatedAt: now}).Return(nil),
		f.telegram.EXPECT().PinnedMessageID(gomock.Any(), groupID).Return(int64(77), nil),
	)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTickRecreatesLostBoard(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		expect func(*boardFixture, string)
	}{
		{
			name:   "deleted board cannot be edited",
			stored: "old",
			expect: func(f *boardFixture, body string) {
				f.telegram.EXPECT().EditBoard(gomock.Any(), groupID, int64(77), domain.Render(body, now)).Return(errors.Join(errors.New("api"), channelmodels.ErrTelegramRejected))
			},
		},
		{
			name: "deleted board cannot be pinned again",
			expect: func(f *boardFixture, _ string) {
				f.telegram.EXPECT().PinnedMessageID(gomock.Any(), groupID).Return(int64(0), nil)
				f.telegram.EXPECT().PinBoard(gomock.Any(), groupID, int64(77)).Return(channelmodels.ErrTelegramRejected)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := teamBody()
			stored := tt.stored
			if stored == "" {
				stored = body
			}
			f := newBoardFixture(t)
			f.expectPoll([]models.Board{{ChannelID: channelID, MessageID: 77, Body: stored}})
			tt.expect(f, body)
			f.telegram.EXPECT().CanPin(gomock.Any(), groupID).Return(true, nil)
			f.telegram.EXPECT().SendBoard(gomock.Any(), groupID, domain.Render(body, now)).Return(int64(78), nil)
			f.boards.EXPECT().SaveBoard(gomock.Any(), models.Board{ChannelID: channelID, MessageID: 78, Body: body, UpdatedAt: now}).Return(nil)
			f.telegram.EXPECT().PinBoard(gomock.Any(), groupID, int64(78)).Return(nil)
			if err := f.uc.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTickRepinsUnpinnedBoard(t *testing.T) {
	f := newBoardFixture(t)
	f.gitlab.EXPECT().OpenMergeRequests(gomock.Any()).Return(nil, nil)
	f.boards.EXPECT().GetTracks(gomock.Any()).Return(nil, nil, nil)
	f.boards.EXPECT().SaveTracks(gomock.Any(), gomock.Len(0), gomock.Len(0)).Return(nil)
	f.channels.EXPECT().GetChannels(gomock.Any(), gomock.Any()).Return([]channelmodels.Channel{{ChannelID: channelID, GroupID: groupID}}, nil)
	f.channels.EXPECT().GetParticipants(gomock.Any(), gomock.Any()).Return([]channelmodels.Participant{{ChannelID: channelID, GitLabUsername: "yarik", TelegramLabel: "@yarik"}}, nil)
	f.boards.EXPECT().GetBoards(gomock.Any(), gomock.Any()).Return([]models.Board{{ChannelID: channelID, MessageID: 77, Body: "Открытых MR команды нет."}}, nil)
	f.telegram.EXPECT().PinnedMessageID(gomock.Any(), groupID).Return(int64(12), nil)
	f.telegram.EXPECT().PinBoard(gomock.Any(), groupID, int64(77)).Return(nil)
	if err := f.uc.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTickStopsOnGitLabFailure(t *testing.T) {
	f := newBoardFixture(t)
	f.gitlab.EXPECT().OpenMergeRequests(gomock.Any()).Return(nil, errors.New("boom"))
	if err := f.uc.Tick(context.Background()); err == nil {
		t.Fatal("GitLab failure was ignored")
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	f := newBoardFixture(t)
	f.gitlab.EXPECT().OpenMergeRequests(gomock.Any()).Return(nil, errors.New("boom"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- f.uc.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}
