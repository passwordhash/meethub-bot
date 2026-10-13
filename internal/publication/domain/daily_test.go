package domain

import (
	"strings"
	"testing"
	"time"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func TestRenderDaily(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		lead     *models.Report
		mention  bool
		wantDate string
		wantLead string
		wantTime string
	}{
		{name: "UTC midnight maps to Moscow date", date: time.Date(2025, 4, 1, 22, 30, 0, 0, time.UTC), wantDate: "ср, 02.04.2025", wantTime: "01:30"},
		{name: "date before Moscow midnight", date: time.Date(2025, 4, 1, 20, 30, 0, 0, time.UTC), wantDate: "вт, 01.04.2025", wantTime: "23:30"},
		{name: "actual daily date weekday and time", date: time.Date(2026, 10, 14, 12, 30, 0, 0, channelmodels.Moscow), wantDate: "ср, 14.10.2026", wantTime: "12:30"},
		{name: "Sunday short name", date: time.Date(2026, 10, 18, 9, 5, 0, 0, channelmodels.Moscow), wantDate: "вс, 18.10.2026", wantTime: "09:05"},
		{name: "safe known user mention", date: time.Now(), lead: &models.Report{AuthorID: 42, Author: "<A&B>"}, mention: true, wantLead: `👤 Ведущий: <a href="tg://user?id=42">&lt;A&amp;B&gt;</a>`},
		{name: "no mention when disallowed", date: time.Now(), lead: &models.Report{AuthorID: 42, Author: "A"}, wantLead: "👤 Ведущий: A"},
		{name: "report text is not part of channel post", date: time.Now(), lead: &models.Report{AuthorID: 7, Author: "Lead", Text: strings.Repeat("must-not-appear", 1000)}, wantLead: "👤 Ведущий: Lead"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDaily(tt.date, tt.lead, tt.mention, "https://meet.google.com/mmb-hczk-iqw")
			if !strings.Contains(got, tt.wantDate) {
				t.Errorf("missing Moscow date %q: %s", tt.wantDate, got)
			}
			if tt.wantLead != "" && !strings.Contains(got, tt.wantLead) {
				t.Errorf("missing lead %q in %s", tt.wantLead, got)
			}
			if tt.wantTime != "" && !strings.Contains(got, "📞 Созвон в "+tt.wantTime+" (UTC+3): https://meet.google.com/mmb-hczk-iqw") {
				t.Errorf("missing meeting time %q: %s", tt.wantTime, got)
			}
			if !strings.Contains(got, "📅 Дейли команды — ") || !strings.Contains(got, "Пишите отчёты в тред 👇") {
				t.Errorf("daily post template missing expected text: %s", got)
			}
			if strings.Contains(got, "must-not-appear") {
				t.Errorf("channel post contains report text")
			}
		})
	}
}
