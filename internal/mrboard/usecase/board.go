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
	pins := time.NewTicker(90 * time.Second)
	defer pins.Stop()
	if err := u.Tick(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("MR board update failed; retrying", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := u.Tick(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("MR board update failed; retrying", "error", err)
			}
		case <-pins.C:
			if err := u.maintainPins(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("MR board pin maintenance failed; retrying", "error", err)
			}
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
	// Manual participants remain stored, but profiles are the active source for all channels.
	labels := make(map[string]string)
	for _, mr := range mrs {
		for username, label := range mr.TelegramLabels {
			labels[username] = label
		}
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
		channelLabels := labels
		board, found := existing[channel.ChannelID]
		// Clear an existing board after the last participant is removed, but do not create one.
		if len(channelLabels) == 0 && !found {
			continue
		}
		body := domain.RenderBody(domain.Items(mrs, reviewers, sections, channelLabels, now), channelLabels, channel.BoardView, now)
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
	if board.Format != models.BoardFormatRich {
		// A plain-text pin cannot be edited into a rich message: drop it and post a rich board in its place (ADR 0002).
		if err := u.telegram.UnpinBoard(ctx, channel.GroupID, board.MessageID); err != nil {
			slog.Warn("unpin plain MR board failed; pinning the new board replaces it", "channel_id", channel.ChannelID, "error", err)
		}
		return u.create(ctx, channel, body, now)
	}
	if body != board.Body {
		err := u.telegram.EditBoard(ctx, channel.GroupID, board.MessageID, domain.Render(body, now))
		if errors.Is(err, channelmodels.ErrBoardMessageMissing) {
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
	return u.ensurePin(ctx, channel, board, now)
}

// maintainPins does not depend on GitLab being available or on the board body changing.
func (u *BoardUseCase) maintainPins(ctx context.Context) error {
	channels, err := u.channels.GetChannels(ctx, channelmodels.GetChannelsFilters{})
	if err != nil {
		return err
	}
	boards, err := u.boards.GetBoards(ctx, models.GetBoardsFilters{})
	if err != nil {
		return err
	}
	byChannel := make(map[int64]models.Board, len(boards))
	for _, board := range boards {
		byChannel[board.ChannelID] = board
	}
	var failed error
	for _, channel := range channels {
		if board, ok := byChannel[channel.ChannelID]; ok && channel.BoardPinLast && board.Format == models.BoardFormatRich {
			failed = errors.Join(failed, u.ensurePin(ctx, channel, board, u.now()))
		}
	}
	return failed
}

func (u *BoardUseCase) ensurePin(ctx context.Context, channel channelmodels.Channel, board models.Board, now time.Time) error {
	pinned, err := u.telegram.PinnedMessageID(ctx, channel.GroupID)
	if err != nil {
		return fmt.Errorf("get pinned message: %w", err)
	}
	if pinned == board.MessageID || (!channel.BoardPinLast && pinned != 0) {
		return nil
	}
	// Telegram can leave an already pinned message in its old position when pinning it again.
	// Unpin only the board, never the other messages, before putting it back.
	if channel.BoardPinLast && pinned != 0 {
		if err := u.telegram.UnpinBoard(ctx, channel.GroupID, board.MessageID); err != nil {
			return fmt.Errorf("unpin MR board: %w", err)
		}
	}
	err = u.telegram.PinBoard(ctx, channel.GroupID, board.MessageID)
	if errors.Is(err, channelmodels.ErrBoardMessageMissing) {
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
	if err := u.boards.SaveBoard(ctx, models.Board{ChannelID: channel.ChannelID, MessageID: messageID, Body: body, Format: models.BoardFormatRich, UpdatedAt: now}); err != nil {
		return err
	}
	if err := u.telegram.PinBoard(ctx, channel.GroupID, messageID); err != nil {
		return fmt.Errorf("pin MR board: %w", err)
	}
	return nil
}
