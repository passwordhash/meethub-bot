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
	ownerID     int64
	repository  ChannelRepository
	telegram    TelegramClient
	processor   updateProcessor
	pendingCron map[int64]int64
}

func NewChannelUseCase(ownerID int64, repository ChannelRepository, telegram TelegramClient, processor updateProcessor) *ChannelUseCase {
	return &ChannelUseCase{ownerID: ownerID, repository: repository, telegram: telegram, processor: processor, pendingCron: make(map[int64]int64)}
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

func (u *ChannelUseCase) setDailySchedule(ctx context.Context, chatID int64, arguments string) error {
	arguments = strings.TrimSpace(arguments)
	fields := strings.Fields(arguments)
	if len(fields) == 3 && fields[1] == "lead" {
		if fields[2] != "on" && fields[2] != "off" {
			return u.telegram.Send(ctx, chatID, "Формат: /daily-schedule <ID канала> lead on|off")
		}
		channelID, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || models.ValidateChannelID(channelID) != nil {
			return u.telegram.Send(ctx, chatID, "Проверьте ID канала.")
		}
		if _, err := u.scheduleForChannel(ctx, channelID); errors.Is(err, models.ErrInvalidChannelID) {
			return u.telegram.Send(ctx, chatID, "Канал не подключён.")
		} else if err != nil {
			return u.telegram.Send(ctx, chatID, "Не удалось прочитать настройки канала.")
		}
		enabled := fields[2] == "on"
		if err := u.repository.SetLeadEnabled(ctx, channelID, enabled); err != nil {
			return u.telegram.Send(ctx, chatID, "Не удалось сохранить настройку ведущего.")
		}
		if enabled {
			return u.telegram.Send(ctx, chatID, "Случайное назначение ведущего включено.")
		}
		return u.telegram.Send(ctx, chatID, "Упоминание ведущего отключено.")
	}
	if len(fields) < 2 {
		return u.telegram.Send(ctx, chatID, "Формат: /daily-schedule <ID канала> \"<cron-выражение>\"; ведущего: /daily-schedule <ID канала> lead on|off")
	}
	channelID, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || models.ValidateChannelID(channelID) != nil {
		return u.telegram.Send(ctx, chatID, "Проверьте ID канала.")
	}
	cronExpression := strings.TrimSpace(strings.TrimPrefix(arguments, fields[0]))
	if len(cronExpression) < 2 || cronExpression[0] != '"' || cronExpression[len(cronExpression)-1] != '"' {
		return u.telegram.Send(ctx, chatID, "Формат: /daily-schedule <ID канала> \"<cron-выражение>\"")
	}
	cronExpression = cronExpression[1 : len(cronExpression)-1]
	if strings.ContainsRune(cronExpression, '"') {
		return u.telegram.Send(ctx, chatID, "Укажите cron-выражение в двойных кавычках.")
	}
	if _, err := models.ParseCron(cronExpression); err != nil {
		return u.telegram.Send(ctx, chatID, "Проверьте пяти-полевое cron-выражение.")
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

func (u *ChannelUseCase) sendMenu(ctx context.Context, chatID int64) error {
	return u.telegram.SendKeyboard(ctx, chatID, "Настройки дейли Cascade:", [][]models.InlineButton{
		{{Text: "🗓 Расписание", CallbackData: "ui:schedule"}},
		{{Text: "👤 Случайный ведущий", CallbackData: "ui:lead"}},
	})
}

func (u *ChannelUseCase) sendChannelMenu(ctx context.Context, chatID int64, leadMenu bool) error {
	schedules, err := u.repository.GetSchedules(ctx, models.GetSchedulesFilters{})
	if err != nil {
		return u.telegram.Send(ctx, chatID, "Не удалось загрузить каналы.")
	}
	if len(schedules) == 0 {
		return u.telegram.Send(ctx, chatID, "Сначала подключите канал командой /connect <ID канала>.")
	}
	keyboard := make([][]models.InlineButton, 0, len(schedules)+1)
	for _, schedule := range schedules {
		button := models.InlineButton{Text: strconv.FormatInt(schedule.ChannelID, 10)}
		if leadMenu {
			if schedule.LeadEnabled {
				button.Text += " · ведущий вкл."
				button.CallbackData = fmt.Sprintf("ui:lead:%d:off", schedule.ChannelID)
			} else {
				button.Text += " · ведущий выкл."
				button.CallbackData = fmt.Sprintf("ui:lead:%d:on", schedule.ChannelID)
			}
		} else {
			button.Text += " · расписание"
			button.CallbackData = fmt.Sprintf("ui:schedule:%d", schedule.ChannelID)
		}
		keyboard = append(keyboard, []models.InlineButton{button})
	}
	keyboard = append(keyboard, []models.InlineButton{{Text: "⬅️ Меню", CallbackData: "ui:menu"}})
	prompt := "Выберите канал для расписания:"
	if leadMenu {
		prompt = "Нажмите на канал, чтобы переключить случайного ведущего:"
	}
	return u.telegram.SendKeyboard(ctx, chatID, prompt, keyboard)
}

func (u *ChannelUseCase) handleCallback(ctx context.Context, update models.Update) error {
	if err := u.telegram.AnswerCallback(ctx, update.CallbackQueryID, ""); err != nil {
		return err
	}
	parts := strings.Split(update.CallbackData, ":")
	switch {
	case update.CallbackData == "ui:menu":
		return u.sendMenu(ctx, update.ChatID)
	case update.CallbackData == "ui:schedule":
		return u.sendChannelMenu(ctx, update.ChatID, false)
	case update.CallbackData == "ui:lead":
		return u.sendChannelMenu(ctx, update.ChatID, true)
	case len(parts) == 3 && parts[0] == "ui" && parts[1] == "schedule":
		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || models.ValidateChannelID(channelID) != nil {
			return u.telegram.Send(ctx, update.ChatID, "Не удалось распознать канал.")
		}
		if _, err := u.scheduleForChannel(ctx, channelID); err != nil {
			return u.telegram.Send(ctx, update.ChatID, "Канал больше не подключён.")
		}
		u.pendingCron[update.ChatID] = channelID
		return u.telegram.SendKeyboard(ctx, update.ChatID, "Отправьте cron-выражение сообщением, например: 30 9 * * 1,3,5", [][]models.InlineButton{{{Text: "Отмена", CallbackData: "ui:cancel"}}})
	case len(parts) == 4 && parts[0] == "ui" && parts[1] == "lead":
		channelID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || models.ValidateChannelID(channelID) != nil || (parts[3] != "on" && parts[3] != "off") {
			return u.telegram.Send(ctx, update.ChatID, "Не удалось распознать настройку ведущего.")
		}
		if _, err := u.scheduleForChannel(ctx, channelID); err != nil {
			return u.telegram.Send(ctx, update.ChatID, "Канал больше не подключён.")
		}
		enabled := parts[3] == "on"
		if err := u.repository.SetLeadEnabled(ctx, channelID, enabled); err != nil {
			return u.telegram.Send(ctx, update.ChatID, "Не удалось сохранить настройку ведущего.")
		}
		return u.sendChannelMenu(ctx, update.ChatID, true)
	case update.CallbackData == "ui:cancel":
		delete(u.pendingCron, update.ChatID)
		return u.sendMenu(ctx, update.ChatID)
	default:
		return u.telegram.Send(ctx, update.ChatID, "Эта кнопка устарела. Откройте /start.")
	}
}

func (u *ChannelUseCase) handle(ctx context.Context, update models.Update) error {
	if update.Kind == "callback_query" {
		return u.handleCallback(ctx, update)
	}
	text := strings.TrimSpace(update.Text)
	if channelID, pending := u.pendingCron[update.ChatID]; pending {
		if strings.HasPrefix(text, "/") {
			delete(u.pendingCron, update.ChatID)
		} else {
			if _, err := models.ParseCron(text); err != nil {
				return u.telegram.Send(ctx, update.ChatID, "Некорректный cron. Отправьте пять полей, например: 30 9 * * 1,3,5")
			}
			delete(u.pendingCron, update.ChatID)
			return u.setDailySchedule(ctx, update.ChatID, fmt.Sprintf("%d %q", channelID, text))
		}
	}
	if text == "/start" {
		delete(u.pendingCron, update.ChatID)
		return u.sendMenu(ctx, update.ChatID)
	}
	if text == "/connect" {
		return u.telegram.Send(ctx, update.ChatID, "Для подключения используйте /connect <ID канала>.")
	}
	if strings.HasPrefix(text, "/daily-schedule ") {
		return u.setDailySchedule(ctx, update.ChatID, strings.TrimPrefix(text, "/daily-schedule"))
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
