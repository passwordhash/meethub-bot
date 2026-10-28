package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

type ChannelUseCase struct {
	ownerID    int64
	repository ChannelRepository
	slots      slotRepository
	telegram   TelegramClient
	processor  updateProcessor
	pending    map[int64]pendingInput
	now        func() time.Time
	mrBoard    bool
}

func NewChannelUseCase(ownerID int64, repository ChannelRepository, slots slotRepository, telegram TelegramClient, processor updateProcessor) *ChannelUseCase {
	return &ChannelUseCase{ownerID: ownerID, repository: repository, slots: slots, telegram: telegram, processor: processor, pending: make(map[int64]pendingInput), now: time.Now}
}

// EnableMRBoard shows the MR board participants in the channel menu; without GitLab the board is off.
func (u *ChannelUseCase) EnableMRBoard() {
	u.mrBoard = true
}

func (u *ChannelUseCase) Run(ctx context.Context) error {
	offset, err := u.processor.GetOffset(ctx)
	if err != nil {
		return fmt.Errorf("load Telegram update offset: %w", err)
	}
	if err := u.notifyScheduleResets(ctx); err != nil {
		return err
	}
	warned := false
	for ctx.Err() == nil {
		updates, err := u.telegram.Updates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !warned {
				slog.Warn("telegram polling failed; retrying")
				warned = true
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil
			case <-timer.C:
			}
			continue
		}
		warned = false
		sort.Slice(updates, func(left, right int) bool { return updates[left].ID < updates[right].ID })
		for _, update := range updates {
			if update.ID < offset {
				continue
			}
			if update.ChatType == "private" && update.FromID == u.ownerID {
				if err := u.handle(ctx, update); err != nil {
					return err
				}
			}
			for {
				err := u.processor.Process(ctx, update)
				if !errors.Is(err, models.ErrUpdatePending) {
					if err != nil {
						return err
					}
					break
				}
				timer := time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return nil
				case <-timer.C:
				}
			}
			offset = update.ID + 1
		}
	}
	return nil
}

func (u *ChannelUseCase) notifyScheduleResets(ctx context.Context) error {
	schedules, err := u.repository.GetSchedules(ctx, models.GetSchedulesFilters{})
	if err != nil {
		return fmt.Errorf("load schedules for reset notice: %w", err)
	}
	for _, schedule := range schedules {
		if !schedule.ResetPending {
			continue
		}
		text := fmt.Sprintf("Расписание канала %d сброшено: прежнее cron-расписание нельзя перенести в недельную сетку. Откройте /start и задайте дни и время дейлика заново.", schedule.ChannelID)
		if err := u.telegram.Send(ctx, u.ownerID, text); err != nil {
			slog.Warn("schedule reset notice failed; retrying on next start", "channel_id", schedule.ChannelID, "error", err)
			continue
		}
		if err := u.repository.ClearScheduleReset(ctx, schedule.ChannelID); err != nil {
			return fmt.Errorf("clear schedule reset notice: %w", err)
		}
	}
	return nil
}

func (u *ChannelUseCase) scheduleForChannel(ctx context.Context, channelID int64) (models.Schedule, error) {
	schedules, err := u.repository.GetSchedules(ctx, models.GetSchedulesFilters{ChannelID: &channelID})
	if err != nil {
		return models.Schedule{}, err
	}
	if len(schedules) == 0 {
		return models.Schedule{}, models.ErrInvalidChannelID
	}
	if len(schedules) != 1 {
		return models.Schedule{}, fmt.Errorf("expected one schedule for channel %d, got %d", channelID, len(schedules))
	}
	return schedules[0], nil
}

func (u *ChannelUseCase) handle(ctx context.Context, update models.Update) error {
	if update.Kind == "callback_query" {
		return u.handleCallback(ctx, update)
	}
	text := strings.TrimSpace(update.Text)
	if input, waiting := u.pending[update.ChatID]; waiting {
		if !strings.HasPrefix(text, "/") {
			return u.handlePendingInput(ctx, update.ChatID, input, text)
		}
		delete(u.pending, update.ChatID)
	}
	if text == "/start" {
		return u.sendChannelList(ctx, update.ChatID)
	}
	if text == "/connect" {
		return u.telegram.Send(ctx, update.ChatID, "Для подключения используйте /connect <ID канала>.")
	}
	if !strings.HasPrefix(text, "/connect ") {
		return nil
	}
	channelID, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(text, "/connect ")), 10, 64)
	if err != nil || models.ValidateChannelID(channelID) != nil {
		return u.telegram.Send(ctx, update.ChatID, "Укажите числовой ID канала, например /connect -1001234567890")
	}
	permissions, err := u.telegram.VerifyChannel(ctx, channelID, u.ownerID)
	if err != nil || permissions.Validate(channelID) != nil {
		return u.telegram.Send(ctx, update.ChatID, "Не удалось подтвердить права бота и связь канала с группой; привязка не сохранена.")
	}
	if _, err := u.repository.Create(ctx, models.Channel{ChannelID: channelID, GroupID: permissions.LinkedGroupID}); err != nil {
		if errors.Is(err, models.ErrAlreadyConnected) || errors.Is(err, models.ErrGroupAlreadyConnected) {
			return u.telegram.Send(ctx, update.ChatID, "Канал или группа уже подключены к другой привязке.")
		}
		return u.telegram.Send(ctx, update.ChatID, "Не удалось сохранить привязку; попробуйте позже.")
	}
	return u.telegram.Send(ctx, update.ChatID, "Канал и связанная группа подключены. Privacy mode автоматически проверить нельзя: отключите его через BotFather и заново добавьте бота в группу.")
}
