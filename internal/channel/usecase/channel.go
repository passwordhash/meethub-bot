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
	telegram   TelegramClient
	processor  updateProcessor
}

func NewChannelUseCase(ownerID int64, repository ChannelRepository, telegram TelegramClient, processor updateProcessor) *ChannelUseCase {
	return &ChannelUseCase{ownerID: ownerID, repository: repository, telegram: telegram, processor: processor}
}

func (u *ChannelUseCase) Run(ctx context.Context) error {
	offset, err := u.processor.GetOffset(ctx)
	if err != nil {
		return fmt.Errorf("load Telegram update offset: %w", err)
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

func (u *ChannelUseCase) setSchedule(ctx context.Context, chatID int64, args []string) error {
	if len(args) < 2 {
		return u.telegram.Send(ctx, chatID, "Формат: /daily-schedule <ID канала> <cron: минута час день месяц день-недели> или /daily-schedule <ID канала> lead on|off")
	}
	channelID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || models.ValidateChannelID(channelID) != nil {
		return u.telegram.Send(ctx, chatID, "Проверьте ID канала.")
	}
	cronExpression := strings.Join(args[1:], " ")
	_, cronErr := models.ParseCron(cronExpression)
	if cronErr != nil && len(args) == 3 {
		weekdays, daysErr := models.ParseWeekdays(args[1])
		hour, minute, clockErr := models.ParseClock(args[2])
		if daysErr == nil && clockErr == nil {
			cronExpression = models.CronFromLegacy(weekdays, hour, minute)
			_, cronErr = models.ParseCron(cronExpression)
		}
	}
	if cronErr != nil {
		return u.telegram.Send(ctx, chatID, "Проверьте cron-выражение из пяти полей (минута час день месяц день-недели).")
	}
	now := time.Now().In(models.Moscow)
	current, err := u.scheduleForChannel(ctx, channelID)
	if errors.Is(err, models.ErrInvalidChannelID) {
		return u.telegram.Send(ctx, chatID, "Канал не подключён.")
	}
	if err != nil {
		return u.telegram.Send(ctx, chatID, "Не удалось прочитать настройки канала.")
	}
	schedule := models.Schedule{ChannelID: channelID, Cron: cronExpression, LeadEnabled: current.LeadEnabled}
	keepOverride := models.KeepOverrideAfterScheduleEdit(now, schedule, current.Override)
	if err := u.repository.SetSchedule(ctx, schedule, keepOverride); err != nil {
		return u.telegram.Send(ctx, chatID, "Не удалось сохранить расписание; проверьте, что канал подключён.")
	}
	message := "Cron-расписание дейли сохранено (UTC+3). Публикация выполняется в заданное время."
	if current.Override != nil && !keepOverride {
		message += " Существовавший перенос сброшен: для нового расписания он не проходит ограничение до второго обычного слота."
	}
	return u.telegram.Send(ctx, chatID, message)
}

func (u *ChannelUseCase) setDailySchedule(ctx context.Context, chatID int64, args []string) error {
	if len(args) == 3 && args[1] == "lead" {
		if args[2] != "on" && args[2] != "off" {
			return u.telegram.Send(ctx, chatID, "Формат: /daily-schedule <ID канала> lead on|off")
		}
		channelID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || models.ValidateChannelID(channelID) != nil {
			return u.telegram.Send(ctx, chatID, "Проверьте ID канала.")
		}
		if _, err := u.scheduleForChannel(ctx, channelID); errors.Is(err, models.ErrInvalidChannelID) {
			return u.telegram.Send(ctx, chatID, "Канал не подключён.")
		} else if err != nil {
			return u.telegram.Send(ctx, chatID, "Не удалось прочитать настройки канала.")
		}
		enabled := args[2] == "on"
		if err := u.repository.SetLeadEnabled(ctx, channelID, enabled); err != nil {
			return u.telegram.Send(ctx, chatID, "Не удалось сохранить настройку ведущего.")
		}
		if enabled {
			return u.telegram.Send(ctx, chatID, "Случайное назначение ведущего включено.")
		}
		return u.telegram.Send(ctx, chatID, "Упоминание ведущего отключено.")
	}
	return u.setSchedule(ctx, chatID, args)
}

func (u *ChannelUseCase) move(ctx context.Context, chatID int64, args []string) error {
	if len(args) != 3 {
		return u.telegram.Send(ctx, chatID, "Формат: /move <ID канала> <YYYY-MM-DD> <HH:MM>")
	}
	channelID, err := strconv.ParseInt(args[0], 10, 64)
	target, dateErr := time.ParseInLocation("2006-01-02 15:04", args[1]+" "+args[2], models.Moscow)
	if err != nil || models.ValidateChannelID(channelID) != nil || dateErr != nil || target.Format("2006-01-02 15:04") != args[1]+" "+args[2] {
		return u.telegram.Send(ctx, chatID, "Укажите ID канала и дату/время YYYY-MM-DD HH:MM (UTC+3).")
	}
	now := time.Now().In(models.Moscow)
	schedule, err := u.scheduleForChannel(ctx, channelID)
	if errors.Is(err, models.ErrInvalidChannelID) {
		return u.telegram.Send(ctx, chatID, "Канал не подключён.")
	}
	if err != nil {
		return u.telegram.Send(ctx, chatID, "Не удалось прочитать настройки канала.")
	}
	if err := models.ValidateMove(now, schedule, target); err != nil {
		return u.telegram.Send(ctx, chatID, "Перенос требует настроенного расписания и будущей даты раньше второго ближайшего обычного слота.")
	}
	if err := u.repository.SetOverride(ctx, channelID, target); err != nil {
		if errors.Is(err, models.ErrInvalidSchedule) {
			return u.telegram.Send(ctx, chatID, "Канал не подключён или расписание ещё не задано.")
		}
		return u.telegram.Send(ctx, chatID, "Не удалось сохранить перенос; проверьте подключение и расписание.")
	}
	return u.telegram.Send(ctx, chatID, "Разовый перенос сохранён. Перенос меняет только ближайший слот; расписание остаётся прежним.")
}

func (u *ChannelUseCase) handle(ctx context.Context, update models.Update) error {
	text := strings.TrimSpace(update.Text)
	if text == "/start" || text == "/connect" {
		return u.telegram.Send(ctx, update.ChatID, "Используйте /connect <ID канала> для подключения канала; /daily-schedule <ID> <cron из 5 полей> для расписания, /daily-schedule <ID> lead on|off для ведущего; /move <ID> <YYYY-MM-DD> <HH:MM>.")
	}
	if strings.HasPrefix(text, "/daily-schedule ") {
		return u.setDailySchedule(ctx, update.ChatID, strings.Fields(text)[1:])
	}
	if strings.HasPrefix(text, "/move ") {
		return u.move(ctx, update.ChatID, strings.Fields(text)[1:])
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
