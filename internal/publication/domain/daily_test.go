package domain

import (
	"strings"
	"testing"
	"time"

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
	}{
		{name: "UTC midnight maps to Moscow date", date: time.Date(2025, 4, 1, 22, 30, 0, 0, time.UTC), wantDate: "02.04.2025"},
		{name: "date before Moscow midnight", date: time.Date(2025, 4, 1, 20, 30, 0, 0, time.UTC), wantDate: "01.04.2025"},
		{name: "safe known user mention", date: time.Now(), lead: &models.Report{AuthorID: 42, Author: "<A&B>"}, mention: true, wantLead: `<a href="tg://user?id=42">&lt;A&amp;B&gt;</a>`},
		{name: "no mention when disallowed", date: time.Now(), lead: &models.Report{AuthorID: 42, Author: "A"}, wantLead: "Ведущий: A"},
		{name: "report text is not part of channel post", date: time.Now(), lead: &models.Report{AuthorID: 7, Author: "Lead", Text: strings.Repeat("must-not-appear", 1000)}, wantLead: "Ведущий: Lead"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDaily(tt.date, tt.lead, tt.mention, "")
			if !strings.Contains(got, tt.wantDate) {
				t.Errorf("missing Moscow date %q: %s", tt.wantDate, got)
			}
			if tt.wantLead != "" && !strings.Contains(got, tt.wantLead) {
				t.Errorf("missing lead %q in %s", tt.wantLead, got)
			}
			if strings.Contains(got, "must-not-appear") {
				t.Errorf("channel post contains report text")
			}
		})
	}
}
