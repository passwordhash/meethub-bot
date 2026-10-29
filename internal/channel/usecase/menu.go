package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

type pendingKind int

const (
	pendingDailyTime   pendingKind = 1
	pendingPostOffset  pendingKind = 2
	pendingIgnoredUser pendingKind = 3
	pendingParticipant pendingKind = 4
)

type pendingInput struct {
	kind      pendingKind
	channelID int64
	weekday   time.Weekday
}

const (
	actionList              = "list"
	actionChannel           = "ch"
	actionDay               = "day"
	actionRemoveDay         = "del"
	actionOffset            = "off"
	actionOffsetPreset      = "offp"
	actionOffsetCustom      = "offc"
	actionLead              = "host"
	actionIgnored           = "ign"
	actionIgnoreAdd         = "igna"
	actionIgnoreRemove      = "ignd"
	actionParticipants      = "prt"
	actionParticipantAdd    = "prta"
	actionParticipantRemove = "prtd"
	actionBoardView         = "view"
	staleButtonResponse     = "Эта кнопка устарела. Откройте /start."
)

var callbackArgumentCounts = map[string]int{
	actionChannel:           0,
	actionDay:               1,
	actionRemoveDay:         1,
	actionOffset:            0,
	actionOffsetPreset:      1,
	actionOffsetCustom:      0,
	actionLead:              1,
	actionIgnored:           0,
	actionIgnoreAdd:         0,
	actionIgnoreRemove:      1,
	actionParticipants:      0,
	actionParticipantAdd:    0,
	actionParticipantRemove: 1,
	actionBoardView:         0,
}

var weekdaysFromMonday = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}

var postOffsetPresets = []int{15, 30, 60, 120}

func callbackData(action string, channelID int64, arguments ...string) string {
	return strings.Join(append([]string{"ui", action, strconv.FormatInt(channelID, 10)}, arguments...), ":")
}

func weekdayLabel(weekday time.Weekday) string {
	short := models.ShortWeekday(weekday)
	first, size := utf8.DecodeRuneInString(short)
	return string(unicode.ToUpper(first)) + short[size:]
}

func (u *ChannelUseCase) sendChannelList(ctx context.Context, chatID int64) error {
	schedules, err := u.repository.GetSchedules(ctx, models.GetSchedulesFilters{})
	if err != nil {
		slog.Error("load channels failed", "error", err)
		return u.telegram.Send(ctx, chatID, "Не удалось загрузить каналы.")
	}
	if len(schedules) == 0 {
		return u.telegram.Send(ctx, chatID, "Сначала подключите канал командой /connect <ID канала>.")
	}
	keyboard := make([][]models.InlineButton, 0, len(schedules))
	for _, schedule := range schedules {
		keyboard = append(keyboard, []models.InlineButton{{Text: strconv.FormatInt(schedule.ChannelID, 10), CallbackData: callbackData(actionChannel, schedule.ChannelID)}})
	}
	return u.telegram.SendKeyboard(ctx, chatID, "Выберите канал:", keyboard)
}

func (u *ChannelUseCase) sendChannelScreen(ctx context.Context, chatID, channelID int64, notice string) error {
	schedule, err := u.scheduleForChannel(ctx, channelID)
	if err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось прочитать настройки канала.")
	}
	text := fmt.Sprintf("Канал %d (UTC+3):", channelID)
	if notice != "" {
		text = notice + "\n\n" + text
	}
	keyboard := make([][]models.InlineButton, 0, len(weekdaysFromMonday)+4)
	for _, weekday := range weekdaysFromMonday {
		startsAt := "—"
		if dailyTime, scheduled := schedule.DailyTimes[weekday]; scheduled {
			startsAt = models.FormatStartsAt(dailyTime)
		}
		keyboard = append(keyboard, []models.InlineButton{{Text: weekdayLabel(weekday) + " · " + startsAt, CallbackData: callbackData(actionDay, channelID, strconv.Itoa(int(weekday)))}})
	}
	leadText, leadTarget := "👤 Ведущий: вкл.", "off"
	if !schedule.LeadEnabled {
		leadText, leadTarget = "👤 Ведущий: выкл.", "on"
	}
	keyboard = append(keyboard,
		[]models.InlineButton{{Text: fmt.Sprintf("⏱ Пост за %d мин", schedule.PostOffsetMinutes), CallbackData: callbackData(actionOffset, channelID)}},
		[]models.InlineButton{{Text: leadText, CallbackData: callbackData(actionLead, channelID, leadTarget)}},
		[]models.InlineButton{{Text: "🚫 Игнор-лист", CallbackData: callbackData(actionIgnored, channelID)}},
	)
	if u.mrBoard {
		keyboard = append(keyboard,
			[]models.InlineButton{{Text: "🧑‍💻 Участники MR", CallbackData: callbackData(actionParticipants, channelID)}},
			[]models.InlineButton{{Text: "🎲 Доска: " + schedule.BoardView.Label(), CallbackData: callbackData(actionBoardView, channelID)}},
		)
	}
	keyboard = append(keyboard, []models.InlineButton{{Text: "⬅️ Каналы", CallbackData: "ui:" + actionList}})
	return u.telegram.SendKeyboard(ctx, chatID, text, keyboard)
}

func (u *ChannelUseCase) failChannelAction(ctx context.Context, chatID, channelID int64, err error, failure string) error {
	if errors.Is(err, models.ErrInvalidChannelID) {
		return u.telegram.Send(ctx, chatID, "Канал больше не подключён.")
	}
	slog.Error(failure, "channel_id", channelID, "error", err)
	return u.telegram.Send(ctx, chatID, failure)
}

func (u *ChannelUseCase) handleCallback(ctx context.Context, update models.Update) error {
	if err := u.telegram.AnswerCallback(ctx, update.CallbackQueryID, ""); err != nil {
		return err
	}
	chatID := update.ChatID
	if update.CallbackData == "ui:"+actionList {
		delete(u.pending, chatID)
		return u.sendChannelList(ctx, chatID)
	}
	parts := strings.Split(update.CallbackData, ":")
	if len(parts) < 3 || parts[0] != "ui" {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	action, arguments := parts[1], parts[3:]
	argumentCount, known := callbackArgumentCounts[action]
	if (strings.HasPrefix(action, actionParticipants) || action == actionBoardView) && !u.mrBoard {
		known = false
	}
	channelID, err := strconv.ParseInt(parts[2], 10, 64)
	if !known || len(arguments) != argumentCount || err != nil || models.ValidateChannelID(channelID) != nil {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	delete(u.pending, chatID)
	switch action {
	case actionChannel:
		return u.sendChannelScreen(ctx, chatID, channelID, "")
	case actionDay:
		return u.askDailyTime(ctx, chatID, channelID, arguments[0])
	case actionRemoveDay:
		return u.removeDailyTime(ctx, chatID, channelID, arguments[0])
	case actionOffset:
		return u.sendOffsetMenu(ctx, chatID, channelID)
	case actionOffsetPreset:
		return u.setPostOffsetFromButton(ctx, chatID, channelID, arguments[0])
	case actionOffsetCustom:
		u.pending[chatID] = pendingInput{kind: pendingPostOffset, channelID: channelID}
		return u.telegram.SendKeyboard(ctx, chatID, fmt.Sprintf("Отправьте целое число минут от 0 до %d: за сколько до дейлика выйдет пост.", models.MaxPostOffsetMinutes), [][]models.InlineButton{{cancelButton(channelID)}})
	case actionLead:
		return u.setLeadFromButton(ctx, chatID, channelID, arguments[0])
	case actionIgnored:
		return u.sendIgnoredUsers(ctx, chatID, channelID)
	case actionIgnoreAdd:
		u.pending[chatID] = pendingInput{kind: pendingIgnoredUser, channelID: channelID}
		back := models.InlineButton{Text: "Отмена", CallbackData: callbackData(actionIgnored, channelID)}
		return u.telegram.SendKeyboard(ctx, chatID, "Отправьте Telegram ID пользователя (целое положительное число), чьи отчёты бот не будет учитывать.", [][]models.InlineButton{{back}})
	case actionIgnoreRemove:
		return u.removeIgnoredUser(ctx, chatID, channelID, arguments[0])
	case actionParticipants:
		return u.sendParticipants(ctx, chatID, channelID)
	case actionParticipantAdd:
		u.pending[chatID] = pendingInput{kind: pendingParticipant, channelID: channelID}
		back := models.InlineButton{Text: "Отмена", CallbackData: callbackData(actionParticipants, channelID)}
		return u.telegram.SendKeyboard(ctx, chatID, "Отправьте GitLab username и имя для доски через пробел, например: yarik @yarik. Повторная отправка того же username заменит имя.", [][]models.InlineButton{{back}})
	case actionParticipantRemove:
		return u.removeParticipant(ctx, chatID, channelID, arguments[0])
	case actionBoardView:
		return u.cycleBoardView(ctx, chatID, channelID)
	}
	return u.telegram.Send(ctx, chatID, staleButtonResponse)
}

func cancelButton(channelID int64) models.InlineButton {
	return models.InlineButton{Text: "Отмена", CallbackData: callbackData(actionChannel, channelID)}
}

func parseWeekday(text string) (time.Weekday, bool) {
	number, err := strconv.Atoi(text)
	return time.Weekday(number), err == nil && number >= int(time.Sunday) && number <= int(time.Saturday)
}

func (u *ChannelUseCase) askDailyTime(ctx context.Context, chatID, channelID int64, weekdayText string) error {
	weekday, valid := parseWeekday(weekdayText)
	if !valid {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	schedule, err := u.scheduleForChannel(ctx, channelID)
	if err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось прочитать настройки канала.")
	}
	u.pending[chatID] = pendingInput{kind: pendingDailyTime, channelID: channelID, weekday: weekday}
	buttons := make([]models.InlineButton, 0, 2)
	if _, scheduled := schedule.DailyTimes[weekday]; scheduled {
		buttons = append(buttons, models.InlineButton{Text: "Убрать день", CallbackData: callbackData(actionRemoveDay, channelID, weekdayText)})
	}
	buttons = append(buttons, cancelButton(channelID))
	text := fmt.Sprintf("Отправьте время дейлика (%s) в формате ЧЧ:ММ, UTC+3, например 10:30.", models.ShortWeekday(weekday))
	return u.telegram.SendKeyboard(ctx, chatID, text, [][]models.InlineButton{buttons})
}

func (u *ChannelUseCase) removeDailyTime(ctx context.Context, chatID, channelID int64, weekdayText string) error {
	weekday, valid := parseWeekday(weekdayText)
	if !valid {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	if err := u.repository.DeleteDailyTime(ctx, channelID, weekday); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось убрать день.")
	}
	return u.sendChannelScreen(ctx, chatID, channelID, "")
}

func (u *ChannelUseCase) sendOffsetMenu(ctx context.Context, chatID, channelID int64) error {
	presets := make([]models.InlineButton, 0, len(postOffsetPresets))
	for _, minutes := range postOffsetPresets {
		presets = append(presets, models.InlineButton{Text: fmt.Sprintf("%d мин", minutes), CallbackData: callbackData(actionOffsetPreset, channelID, strconv.Itoa(minutes))})
	}
	keyboard := [][]models.InlineButton{
		presets,
		{{Text: "Своё значение", CallbackData: callbackData(actionOffsetCustom, channelID)}},
		{{Text: "⬅️ Назад", CallbackData: callbackData(actionChannel, channelID)}},
	}
	return u.telegram.SendKeyboard(ctx, chatID, "За сколько минут до дейлика публиковать пост?", keyboard)
}

func (u *ChannelUseCase) setPostOffsetFromButton(ctx context.Context, chatID, channelID int64, minutesText string) error {
	minutes, err := strconv.Atoi(minutesText)
	if err != nil || models.ValidatePostOffset(minutes) != nil {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	return u.savePostOffset(ctx, chatID, channelID, minutes)
}

func (u *ChannelUseCase) savePostOffset(ctx context.Context, chatID, channelID int64, minutes int) error {
	if err := u.repository.SetPostOffset(ctx, channelID, minutes); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось сохранить упреждение.")
	}
	return u.sendChannelScreen(ctx, chatID, channelID, "")
}

func (u *ChannelUseCase) setLeadFromButton(ctx context.Context, chatID, channelID int64, target string) error {
	if target != "on" && target != "off" {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	if err := u.repository.SetLeadEnabled(ctx, channelID, target == "on"); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось сохранить настройку ведущего.")
	}
	return u.sendChannelScreen(ctx, chatID, channelID, "")
}

// cycleBoardView switches the MR board to the next grouping view; the board re-renders on the next GitLab poll.
func (u *ChannelUseCase) cycleBoardView(ctx context.Context, chatID, channelID int64) error {
	schedule, err := u.scheduleForChannel(ctx, channelID)
	if err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось прочитать настройки канала.")
	}
	if err := u.repository.SetBoardView(ctx, channelID, schedule.BoardView.Next()); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось сохранить вид доски.")
	}
	return u.sendChannelScreen(ctx, chatID, channelID, "")
}

func (u *ChannelUseCase) sendIgnoredUsers(ctx context.Context, chatID, channelID int64) error {
	users, err := u.repository.GetIgnoredUsers(ctx, models.GetIgnoredUsersFilters{ChannelID: &channelID})
	if err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось прочитать игнор-лист.")
	}
	text := fmt.Sprintf("Игнор-лист канала %d: отчёты этих пользователей не попадают в сводку и выбор ведущего. Нажмите на ID, чтобы убрать его из списка.", channelID)
	if len(users) == 0 {
		text = fmt.Sprintf("Игнор-лист канала %d пуст.", channelID)
	}
	keyboard := make([][]models.InlineButton, 0, len(users)+2)
	for _, user := range users {
		userID := strconv.FormatInt(user.UserID, 10)
		keyboard = append(keyboard, []models.InlineButton{{Text: "❌ " + userID, CallbackData: callbackData(actionIgnoreRemove, channelID, userID)}})
	}
	keyboard = append(keyboard,
		[]models.InlineButton{{Text: "➕ Добавить", CallbackData: callbackData(actionIgnoreAdd, channelID)}},
		[]models.InlineButton{{Text: "⬅️ Назад", CallbackData: callbackData(actionChannel, channelID)}},
	)
	return u.telegram.SendKeyboard(ctx, chatID, text, keyboard)
}

func (u *ChannelUseCase) removeIgnoredUser(ctx context.Context, chatID, channelID int64, userIDText string) error {
	userID, err := strconv.ParseInt(userIDText, 10, 64)
	if err != nil || models.ValidateUserID(userID) != nil {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	if err := u.repository.DeleteIgnoredUser(ctx, channelID, userID); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось убрать пользователя из игнор-листа.")
	}
	return u.sendIgnoredUsers(ctx, chatID, channelID)
}

func (u *ChannelUseCase) addIgnoredUser(ctx context.Context, chatID, channelID, userID int64) error {
	if err := u.repository.AddIgnoredUser(ctx, channelID, userID); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось добавить пользователя в игнор-лист.")
	}
	return u.sendIgnoredUsers(ctx, chatID, channelID)
}

func (u *ChannelUseCase) sendParticipants(ctx context.Context, chatID, channelID int64) error {
	participants, err := u.repository.GetParticipants(ctx, models.GetParticipantsFilters{ChannelID: &channelID})
	if err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось прочитать участников.")
	}
	text := fmt.Sprintf("Участники канала %d: доска MR показывает MR, где автор или ревьювер — участник, и подписывает их этими именами. Нажмите на участника, чтобы убрать его.", channelID)
	if len(participants) == 0 {
		text = fmt.Sprintf("Участников канала %d нет: доска MR для него не ведётся.", channelID)
	}
	keyboard := make([][]models.InlineButton, 0, len(participants)+2)
	for _, participant := range participants {
		keyboard = append(keyboard, []models.InlineButton{{Text: "❌ " + participant.GitLabUsername + " → " + participant.TelegramLabel, CallbackData: callbackData(actionParticipantRemove, channelID, participant.GitLabUsername)}})
	}
	keyboard = append(keyboard,
		[]models.InlineButton{{Text: "➕ Добавить", CallbackData: callbackData(actionParticipantAdd, channelID)}},
		[]models.InlineButton{{Text: "⬅️ Назад", CallbackData: callbackData(actionChannel, channelID)}},
	)
	return u.telegram.SendKeyboard(ctx, chatID, text, keyboard)
}

func (u *ChannelUseCase) removeParticipant(ctx context.Context, chatID, channelID int64, username string) error {
	if models.ValidateGitLabUsername(username) != nil {
		return u.telegram.Send(ctx, chatID, staleButtonResponse)
	}
	if err := u.repository.DeleteParticipant(ctx, channelID, username); err != nil {
		return u.failChannelAction(ctx, chatID, channelID, err, "Не удалось убрать участника.")
	}
	return u.sendParticipants(ctx, chatID, channelID)
}

func (u *ChannelUseCase) addParticipant(ctx context.Context, chatID int64, participant models.Participant) error {
	if err := u.repository.SetParticipant(ctx, participant); err != nil {
		return u.failChannelAction(ctx, chatID, participant.ChannelID, err, "Не удалось добавить участника.")
	}
	return u.sendParticipants(ctx, chatID, participant.ChannelID)
}

func (u *ChannelUseCase) handlePendingInput(ctx context.Context, chatID int64, input pendingInput, text string) error {
	switch input.kind {
	case pendingDailyTime:
		startsAt, err := models.ParseStartsAt(text)
		if err != nil {
			return u.telegram.Send(ctx, chatID, "Некорректное время. Отправьте время в формате ЧЧ:ММ, например 10:30.")
		}
		delete(u.pending, chatID)
		return u.saveDailyTime(ctx, chatID, input, startsAt)
	case pendingIgnoredUser:
		userID, err := strconv.ParseInt(text, 10, 64)
		if err != nil || models.ValidateUserID(userID) != nil {
			return u.telegram.Send(ctx, chatID, "Некорректный ID. Отправьте Telegram ID пользователя целым положительным числом, например 123456789.")
		}
		delete(u.pending, chatID)
		return u.addIgnoredUser(ctx, chatID, input.channelID, userID)
	case pendingParticipant:
		username, label, err := models.ParseParticipant(text)
		if err != nil {
			return u.telegram.Send(ctx, chatID, fmt.Sprintf("Некорректный ввод. Отправьте GitLab username (латиница, цифры, «_», «.», «-», до %d символов) и имя для доски до %d символов через пробел, например: yarik @yarik.", models.MaxGitLabUsernameLength, models.MaxTelegramLabelLength))
		}
		delete(u.pending, chatID)
		return u.addParticipant(ctx, chatID, models.Participant{ChannelID: input.channelID, GitLabUsername: username, TelegramLabel: label})
	default:
		minutes, err := strconv.Atoi(text)
		if err != nil || models.ValidatePostOffset(minutes) != nil {
			return u.telegram.Send(ctx, chatID, fmt.Sprintf("Некорректное значение. Отправьте целое число минут от 0 до %d.", models.MaxPostOffsetMinutes))
		}
		delete(u.pending, chatID)
		return u.savePostOffset(ctx, chatID, input.channelID, minutes)
	}
}

func (u *ChannelUseCase) saveDailyTime(ctx context.Context, chatID int64, input pendingInput, startsAt time.Duration) error {
	if err := u.repository.SetDailyTime(ctx, input.channelID, input.weekday, startsAt); err != nil {
		return u.failChannelAction(ctx, chatID, input.channelID, err, "Не удалось сохранить время дейлика.")
	}
	return u.sendChannelScreen(ctx, chatID, input.channelID, u.alreadyPostedNotice(ctx, input.channelID, input.weekday))
}

func (u *ChannelUseCase) alreadyPostedNotice(ctx context.Context, channelID int64, weekday time.Weekday) string {
	date := models.NextDateOf(u.now(), weekday)
	dayStart, dayEnd := models.MoscowDay(date)
	slots, err := u.slots.GetSlots(ctx, pubmodels.GetSlotsFilters{ChannelID: &channelID, State: pubmodels.ClaimedStates, ScheduledFrom: &dayStart, ScheduledTo: &dayEnd})
	if err != nil {
		slog.Error("check published slot of the day failed", "channel_id", channelID, "error", err)
		return "Не удалось проверить, вышел ли уже пост на ближайшую дату этого дня."
	}
	if len(slots) == 0 {
		return ""
	}
	return fmt.Sprintf("Пост на %s, %s уже вышел: правка подействует со следующей недели.", models.ShortWeekday(weekday), date.Format("02.01.2006"))
}
