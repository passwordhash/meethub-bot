package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/domain"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

type BoardUseCase struct {
	gitlab    gitLabClient
	channels  channelRepository
	boards    boardRepository
	telegram  boardClient
	notifier  boardNotifier
	interval  time.Duration
	now       func() time.Time
	cannotPin map[int64]bool
}

func NewBoardUseCase(gitlab gitLabClient, channels channelRepository, boards boardRepository, telegram boardClient, notifier boardNotifier, interval time.Duration) (*BoardUseCase, error) {
	if gitlab == nil || channels == nil || boards == nil || telegram == nil || notifier == nil {
		return nil, errors.New("MR board dependencies are required")
	}
	if interval <= 0 {
		return nil, errors.New("MR board poll interval must be positive")
	}
	return &BoardUseCase{gitlab: gitlab, channels: channels, boards: boards, telegram: telegram, notifier: notifier, interval: interval, now: time.Now, cannotPin: make(map[int64]bool)}, nil
}

// Run polls GitLab until ctx is cancelled. A failed poll is retried on the next tick:
// the board is an aid, so it must not stop dailies and summaries.
func (u *BoardUseCase) Run(ctx context.Context) error {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()
	warned := false
	for {
		if err := u.Tick(ctx); err != nil && ctx.Err() == nil {
			if !warned {
				slog.Warn("MR board update failed; retrying", "error", err)
				warned = true
			}
		} else {
			warned = false
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (u *BoardUseCase) Tick(ctx context.Context) error {
	mrs, err := u.gitlab.OpenMergeRequests(ctx)
	if err != nil {
		return fmt.Errorf("list GitLab merge requests: %w", err)
	}
	now := u.now()
	reviewers, sections, err := u.boards.GetTracks(ctx)
	if err != nil {
		return err
	}
	reviewers, sections = domain.Track(mrs, reviewers, sections, now)
	if err := u.boards.SaveTracks(ctx, reviewers, sections); err != nil {
		return err
	}
	channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{})
	if err != nil {
		return fmt.Errorf("get channels for MR boards: %w", err)
	}
	participants, err := u.channels.GetParticipants(ctx, channelmodels.GetParticipantsFilters{})
	if err != nil {
		return err
	}
	labels := make(map[int64]map[string]string)
	for _, participant := range participants {
		if labels[participant.ChannelID] == nil {
			labels[participant.ChannelID] = make(map[string]string)
		}
		labels[participant.ChannelID][participant.GitLabUsername] = participant.TelegramLabel
	}
	boards, err := u.boards.GetBoards(ctx, models.GetBoardsFilters{})
	if err != nil {
		return err
	}
	existing := make(map[int64]models.Board, len(boards))
	for _, board := range boards {
		existing[board.ChannelID] = board
	}
	var failed error
	for _, channel := range channels {
		// A channel without participants has no team, so it gets no board.
		channelLabels := labels[channel.ChannelID]
		if len(channelLabels) == 0 {
			continue
		}
		body := domain.RenderBody(domain.Items(mrs, reviewers, sections, channelLabels, now), channelLabels, now)
		board, found := existing[channel.ChannelID]
		if err := u.sync(ctx, channel, board, found, body, now); err != nil {
			slog.Warn("MR board sync failed", "channel_id", channel.ChannelID, "error", err)
			failed = errors.Join(failed, err)
		}
	}
	return failed
}

func (u *BoardUseCase) sync(ctx context.Context, channel channelmodels.Channel, board models.Board, found bool, body string, now time.Time) error {
	if !found {
		return u.create(ctx, channel, body, now)
	}
	if body != board.Body {
		err := u.telegram.EditBoard(ctx, channel.GroupID, board.MessageID, domain.Render(body, now))
		if errors.Is(err, channelmodels.ErrTelegramRejected) {
			return u.create(ctx, channel, body, now)
		}
		if err != nil {
			return fmt.Errorf("edit MR board: %w", err)
		}
		board.Body, board.UpdatedAt = body, now
		if err := u.boards.SaveBoard(ctx, board); err != nil {
			return err
		}
	}
	pinned, err := u.telegram.PinnedMessageID(ctx, channel.GroupID)
	if err != nil {
		return fmt.Errorf("get pinned message: %w", err)
	}
	if pinned == board.MessageID {
		return nil
	}
	err = u.telegram.PinBoard(ctx, channel.GroupID, board.MessageID)
	if errors.Is(err, channelmodels.ErrTelegramRejected) {
		return u.create(ctx, channel, board.Body, now)
	}
	if err != nil {
		return fmt.Errorf("pin MR board: %w", err)
	}
	return nil
}

// create posts and pins a new board, but only when the bot may pin it: an unpinned board would get lost in the group.
func (u *BoardUseCase) create(ctx context.Context, channel channelmodels.Channel, body string, now time.Time) error {
	canPin, err := u.telegram.CanPin(ctx, channel.GroupID)
	if err != nil {
		return fmt.Errorf("check pin permission: %w", err)
	}
	if !canPin {
		if u.cannotPin[channel.ChannelID] {
			return nil
		}
		if err := u.notifier.NotifyBoardProblem(ctx, channel.ChannelID); err != nil {
			return fmt.Errorf("notify about MR board permissions: %w", err)
		}
		u.cannotPin[channel.ChannelID] = true
		return nil
	}
	delete(u.cannotPin, channel.ChannelID)
	messageID, err := u.telegram.SendBoard(ctx, channel.GroupID, domain.Render(body, now))
	if err != nil {
		return fmt.Errorf("send MR board: %w", err)
	}
	if err := u.boards.SaveBoard(ctx, models.Board{ChannelID: channel.ChannelID, MessageID: messageID, Body: body, UpdatedAt: now}); err != nil {
		return err
	}
	if err := u.telegram.PinBoard(ctx, channel.GroupID, messageID); err != nil {
		return fmt.Errorf("pin MR board: %w", err)
	}
	return nil
}
