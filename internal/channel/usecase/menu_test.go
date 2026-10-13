package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	pubmodels "github.com/passwordhash/cascade-hub-bot/internal/publication/models"
	"go.uber.org/mock/gomock"
)

const testChannelID = int64(-1001234567890)

func TestStartListsChannels(t *testing.T) {
	f := newMenuFixture(t)
	f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return([]models.Schedule{{ChannelID: testChannelID}, {ChannelID: -5}}, nil)
	f.send("/start")
	if got := f.lastKeyboard(); len(got) != 2 || got[0][0].Text != "-1001234567890" || got[0][0].CallbackData != "ui:ch:-1001234567890" || got[1][0].CallbackData != "ui:ch:-5" {
		t.Fatalf("channel list = %+v", got)
	}
}

func TestStartWithoutChannels(t *testing.T) {
	f := newMenuFixture(t)
	f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return(nil, nil)
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), "Сначала подключите канал командой /connect <ID канала>.").Return(nil)
	f.send("/start")
}

func TestChannelScreen(t *testing.T) {
	tests := []struct {
		name     string
		schedule models.Schedule
		want     []string
	}{
		{
			name: "grid offset and lead on",
			schedule: models.Schedule{ChannelID: testChannelID, PostOffsetMinutes: 60, LeadEnabled: true, DailyTimes: map[time.Weekday]time.Duration{
				time.Monday: 10 * time.Hour, time.Wednesday: 12*time.Hour + 30*time.Minute, time.Friday: 10 * time.Hour,
			}},
			want: []string{"Пн · 10:00", "Вт · —", "Ср · 12:30", "Чт · —", "Пт · 10:00", "Сб · —", "Вс · —", "⏱ Пост за 60 мин", "👤 Ведущий: вкл.", "⬅️ Каналы"},
		},
		{
			name:     "empty grid lead off",
			schedule: models.Schedule{ChannelID: testChannelID, PostOffsetMinutes: 0},
			want:     []string{"Пн · —", "Вт · —", "Ср · —", "Чт · —", "Пт · —", "Сб · —", "Вс · —", "⏱ Пост за 0 мин", "👤 Ведущий: выкл.", "⬅️ Каналы"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMenuFixture(t)
			f.expectSchedule(tt.schedule)
			f.callback("ui:ch:-1001234567890")
			if f.lastText != "Канал -1001234567890 (UTC+3):" {
				t.Fatalf("text = %q", f.lastText)
			}
			var got []string
			for _, row := range f.lastKeyboard() {
				if len(row) != 1 {
					t.Fatalf("row = %+v", row)
				}
				got = append(got, row[0].Text)
				if len(row[0].CallbackData) > 64 {
					t.Errorf("callback data %q exceeds Telegram limit", row[0].CallbackData)
				}
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Fatalf("buttons = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestSetDailyTimeFromButtonAndMessage(t *testing.T) {
	tests := []struct {
		name       string
		now        time.Time
		claimed    bool
		wantNotice string
	}{
		{name: "saved without notice", now: time.Date(2026, 10, 12, 9, 10, 0, 0, models.Moscow)},
		{name: "post already out for nearest date", now: time.Date(2026, 10, 12, 9, 10, 0, 0, models.Moscow), claimed: true, wantNotice: "Пост на пн, 12.10.2026 уже вышел: правка подействует со следующей недели."},
		{name: "nearest date is next week", now: time.Date(2026, 10, 13, 9, 10, 0, 0, models.Moscow), claimed: true, wantNotice: "Пост на пн, 19.10.2026 уже вышел: правка подействует со следующей недели."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMenuFixture(t)
			f.uc.now = func() time.Time { return tt.now }
			f.expectSchedule(models.Schedule{ChannelID: testChannelID, PostOffsetMinutes: 60})
			f.callback("ui:day:-1001234567890:1")
			if got := f.lastKeyboard(); len(got) != 1 || len(got[0]) != 1 || got[0][0].Text != "Отмена" {
				t.Fatalf("prompt keyboard = %+v", got)
			}
			f.repository.EXPECT().SetDailyTime(gomock.Any(), testChannelID, time.Monday, 11*time.Hour+7*time.Minute).Return(nil)
			date, _ := models.MoscowDay(models.NextDateOf(tt.now, time.Monday))
			slots := []pubmodels.Slot(nil)
			if tt.claimed {
				slots = []pubmodels.Slot{{ChannelID: testChannelID, ScheduledAt: date.Add(10 * time.Hour), State: pubmodels.StatePublished}}
			}
			dayEnd := date.AddDate(0, 0, 1)
			f.slots.EXPECT().GetSlots(gomock.Any(), pubmodels.GetSlotsFilters{ChannelID: ptr(testChannelID), State: pubmodels.ClaimedStates, ScheduledFrom: &date, ScheduledTo: &dayEnd}).Return(slots, nil)
			f.expectSchedule(models.Schedule{ChannelID: testChannelID, PostOffsetMinutes: 60, DailyTimes: map[time.Weekday]time.Duration{time.Monday: 11*time.Hour + 7*time.Minute}})
			f.send("11:07")
			wantText := "Канал -1001234567890 (UTC+3):"
			if tt.wantNotice != "" {
				wantText = tt.wantNotice + "\n\n" + wantText
			}
			if f.lastText != wantText {
				t.Fatalf("text = %q; want %q", f.lastText, wantText)
			}
			if f.lastKeyboard()[0][0].Text != "Пн · 11:07" {
				t.Fatalf("screen was not refreshed: %+v", f.lastKeyboard())
			}
			if _, waiting := f.uc.pending[1]; waiting {
				t.Fatal("pending input was not cleared after saving")
			}
		})
	}
}

func TestInvalidInputKeepsWaiting(t *testing.T) {
	tests := []struct {
		name     string
		callback string
		invalid  []string
		valid    string
		expect   func(*menuFixture)
	}{
		{
			name:     "daily time",
			callback: "ui:day:-1001234567890:3",
			invalid:  []string{"9:30", "24:00", "10:60", "утро", "10.30"},
			valid:    "23:59",
			expect: func(f *menuFixture) {
				f.repository.EXPECT().SetDailyTime(gomock.Any(), testChannelID, time.Wednesday, 23*time.Hour+59*time.Minute).Return(nil)
				f.slots.EXPECT().GetSlots(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},
		{
			name:     "custom offset",
			callback: "ui:offc:-1001234567890",
			invalid:  []string{"-1", "1440", "1.5", "час", ""},
			valid:    "1439",
			expect: func(f *menuFixture) {
				f.repository.EXPECT().SetPostOffset(gomock.Any(), testChannelID, 1439).Return(nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMenuFixture(t)
			f.expectSchedule(models.Schedule{ChannelID: testChannelID}).AnyTimes()
			f.callback(tt.callback)
			for _, text := range tt.invalid {
				f.telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Any()).Return(nil)
				f.send(text)
				if _, waiting := f.uc.pending[1]; !waiting {
					t.Fatalf("%q dropped the pending input", text)
				}
			}
			tt.expect(f)
			f.send(tt.valid)
			if _, waiting := f.uc.pending[1]; waiting {
				t.Fatal("pending input survived a valid value")
			}
		})
	}
}

func TestRemoveDay(t *testing.T) {
	f := newMenuFixture(t)
	withDay := models.Schedule{ChannelID: testChannelID, DailyTimes: map[time.Weekday]time.Duration{time.Tuesday: time.Hour}}
	f.expectSchedule(withDay)
	f.callback("ui:day:-1001234567890:2")
	row := f.lastKeyboard()[0]
	if len(row) != 2 || row[0].Text != "Убрать день" || row[0].CallbackData != "ui:del:-1001234567890:2" || row[1].Text != "Отмена" {
		t.Fatalf("prompt row = %+v", row)
	}
	f.repository.EXPECT().DeleteDailyTime(gomock.Any(), testChannelID, time.Tuesday).Return(nil)
	f.expectSchedule(models.Schedule{ChannelID: testChannelID})
	f.callback(row[0].CallbackData)
	if f.lastKeyboard()[1][0].Text != "Вт · —" {
		t.Fatalf("screen = %+v", f.lastKeyboard())
	}
	if _, waiting := f.uc.pending[1]; waiting {
		t.Fatal("pending input survived day removal")
	}
}

func TestOffsetMenu(t *testing.T) {
	f := newMenuFixture(t)
	f.callback("ui:off:-1001234567890")
	var presets []string
	for _, button := range f.lastKeyboard()[0] {
		presets = append(presets, button.CallbackData)
	}
	if strings.Join(presets, " ") != "ui:offp:-1001234567890:15 ui:offp:-1001234567890:30 ui:offp:-1001234567890:60 ui:offp:-1001234567890:120" {
		t.Fatalf("presets = %v", presets)
	}
	if f.lastKeyboard()[1][0].Text != "Своё значение" {
		t.Fatalf("keyboard = %+v", f.lastKeyboard())
	}
	f.repository.EXPECT().SetPostOffset(gomock.Any(), testChannelID, 120).Return(nil)
	f.expectSchedule(models.Schedule{ChannelID: testChannelID, PostOffsetMinutes: 120})
	f.callback("ui:offp:-1001234567890:120")
	if f.lastKeyboard()[7][0].Text != "⏱ Пост за 120 мин" {
		t.Fatalf("screen = %+v", f.lastKeyboard())
	}
}

func TestLeadToggle(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "turn on", data: "ui:host:-1001234567890:on", want: true},
		{name: "turn off", data: "ui:host:-1001234567890:off", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMenuFixture(t)
			f.repository.EXPECT().SetLeadEnabled(gomock.Any(), testChannelID, tt.want).Return(nil)
			f.expectSchedule(models.Schedule{ChannelID: testChannelID, LeadEnabled: tt.want})
			f.callback(tt.data)
			wantText := "👤 Ведущий: выкл."
			if tt.want {
				wantText = "👤 Ведущий: вкл."
			}
			if got := f.lastKeyboard()[8][0]; got.Text != wantText {
				t.Fatalf("lead button = %+v", got)
			}
		})
	}
}

func TestPendingInputResets(t *testing.T) {
	tests := []struct {
		name  string
		reset func(*menuFixture)
	}{
		{name: "command", reset: func(f *menuFixture) {
			f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return(nil, nil)
			f.telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			f.send("/start")
		}},
		{name: "unknown command", reset: func(f *menuFixture) { f.send("/unknown") }},
		{name: "cancel button", reset: func(f *menuFixture) {
			f.expectSchedule(models.Schedule{ChannelID: testChannelID})
			f.callback("ui:ch:-1001234567890")
		}},
		{name: "back to channels", reset: func(f *menuFixture) {
			f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return([]models.Schedule{{ChannelID: testChannelID}}, nil)
			f.callback("ui:list")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMenuFixture(t)
			f.expectSchedule(models.Schedule{ChannelID: testChannelID})
			f.callback("ui:day:-1001234567890:1")
			tt.reset(f)
			if _, waiting := f.uc.pending[1]; waiting {
				t.Fatal("pending input was not reset")
			}
			f.send("10:00")
		})
	}
}

func TestStaleAndMalformedButtons(t *testing.T) {
	tests := []string{"ui:menu", "ui:schedule", "ui:schedule:-1001234567890", "ui:lead", "ui:lead:-1001234567890:on", "ui:cancel", "ui:day:-1001234567890", "ui:day:1:2", "ui:ch:abc", "ui:host:-1001234567890:maybe", "ui:day:-1001234567890:9", "ui:offp:-1001234567890:1440", "other", ""}
	for _, data := range tests {
		t.Run(data, func(t *testing.T) {
			f := newMenuFixture(t)
			f.telegram.EXPECT().Send(gomock.Any(), int64(1), "Эта кнопка устарела. Откройте /start.").Return(nil)
			f.expectSchedule(models.Schedule{ChannelID: testChannelID}).AnyTimes()
			f.callback(data)
		})
	}
}

func TestDisconnectedChannelButtons(t *testing.T) {
	f := newMenuFixture(t)
	f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: ptr(testChannelID)}).Return(nil, nil)
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), "Канал больше не подключён.").Return(nil)
	f.callback("ui:ch:-1001234567890")

	f.repository.EXPECT().SetLeadEnabled(gomock.Any(), testChannelID, true).Return(models.ErrInvalidChannelID)
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), "Канал больше не подключён.").Return(nil)
	f.callback("ui:host:-1001234567890:on")

	f.repository.EXPECT().SetPostOffset(gomock.Any(), testChannelID, 15).Return(errors.New("database is down"))
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), "Не удалось сохранить упреждение.").Return(nil)
	f.callback("ui:offp:-1001234567890:15")
}

func TestNotifyScheduleResets(t *testing.T) {
	f := newMenuFixture(t)
	f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{}).Return([]models.Schedule{
		{ChannelID: -1, ResetPending: true},
		{ChannelID: -2},
		{ChannelID: -3, ResetPending: true},
	}, nil)
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Cond(func(text string) bool { return strings.Contains(text, "-1 сброшено") })).Return(nil)
	f.repository.EXPECT().ClearScheduleReset(gomock.Any(), int64(-1)).Return(nil)
	f.telegram.EXPECT().Send(gomock.Any(), int64(1), gomock.Cond(func(text string) bool { return strings.Contains(text, "-3 сброшено") })).Return(errors.New("telegram is down"))
	if err := f.uc.notifyScheduleResets(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type menuFixture struct {
	t          *testing.T
	uc         *ChannelUseCase
	repository *MockChannelRepository
	slots      *MockslotRepository
	telegram   *MockTelegramClient
	lastText   string
	keyboards  [][][]models.InlineButton
}

func newMenuFixture(t *testing.T) *menuFixture {
	ctrl := gomock.NewController(t)
	f := &menuFixture{t: t, repository: NewMockChannelRepository(ctrl), slots: NewMockslotRepository(ctrl), telegram: NewMockTelegramClient(ctrl)}
	f.uc = NewChannelUseCase(1, f.repository, f.slots, f.telegram, nil)
	f.telegram.EXPECT().AnswerCallback(gomock.Any(), gomock.Any(), "").Return(nil).AnyTimes()
	f.telegram.EXPECT().SendKeyboard(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, text string, keyboard [][]models.InlineButton) error {
		f.lastText = text
		f.keyboards = append(f.keyboards, keyboard)
		return nil
	}).AnyTimes()
	return f
}

func (f *menuFixture) expectSchedule(schedule models.Schedule) *gomock.Call {
	return f.repository.EXPECT().GetSchedules(gomock.Any(), models.GetSchedulesFilters{ChannelID: &schedule.ChannelID}).Return([]models.Schedule{schedule}, nil)
}

func (f *menuFixture) callback(data string) {
	f.t.Helper()
	update := models.Update{Kind: "callback_query", CallbackQueryID: "query", CallbackData: data, ChatID: 1}
	if err := f.uc.handle(context.Background(), update); err != nil {
		f.t.Fatal(err)
	}
}

func (f *menuFixture) send(text string) {
	f.t.Helper()
	if err := f.uc.handle(context.Background(), models.Update{Kind: "message", ChatID: 1, Text: text}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *menuFixture) lastKeyboard() [][]models.InlineButton {
	f.t.Helper()
	if len(f.keyboards) == 0 {
		f.t.Fatal("no keyboard was sent")
	}
	return f.keyboards[len(f.keyboards)-1]
}

func ptr[T any](value T) *T { return &value }
