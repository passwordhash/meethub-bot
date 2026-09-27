package domain

import (
	"strings"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func TestRenderSummary(t *testing.T) {
	tests := []struct {
		name  string
		plans []models.Plan
		want  []string
	}{
		{name: "empty", want: []string{}},
		{name: "multiple authors", plans: []models.Plan{{Author: "Анна", Text: "план"}, {Author: "李", Text: "задача"}}, want: []string{"Анна:\nплан", "李:\nзадача"}},
		{name: "exact boundary", plans: []models.Plan{{Author: "А", Text: strings.Repeat("x", telegramMessageLimit-3)}}, want: []string{"А:\n" + strings.Repeat("x", telegramMessageLimit-3)}},
		{name: "fallback ID and special text", plans: []models.Plan{{AuthorID: 42, Text: "😀 & <текст>\nстрока"}}, want: []string{"Участник 42:\n😀 & <текст>\nстрока"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderSummary(tt.plans)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d messages, want %d: %#v", len(got), len(tt.want), got)
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("message %d = %q, want %q", index, got[index], tt.want[index])
				}
				if size := utf16Length(got[index]); size > telegramMessageLimit {
					t.Fatalf("message %d has %d UTF-16 units", index, size)
				}
			}
		})
	}
}

func TestRenderSummaryPreservesLongPlan(t *testing.T) {
	tests := []struct {
		name  string
		plan  models.Plan
		label string
	}{
		{name: "normal author", plan: models.Plan{Author: "Автор", Text: strings.Repeat("я😀<&\n", 900)}, label: "Автор"},
		{name: "huge author with zero ID", plan: models.Plan{Author: strings.Repeat("名", telegramMessageLimit+1), AuthorID: 0, Text: strings.Repeat("😀план<&\n", 900)}, label: "Участник 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := RenderSummary([]models.Plan{tt.plan})
			var reconstructed strings.Builder
			for index, message := range messages {
				if utf16Length(message) > telegramMessageLimit {
					t.Fatalf("message %d exceeds limit", index)
				}
				prefix := tt.label + ":\n"
				if index > 0 {
					prefix = tt.label + " (продолжение):\n"
				}
				if !strings.HasPrefix(message, prefix) {
					t.Fatalf("message %d missing label %q", index, prefix)
				}
				reconstructed.WriteString(strings.TrimPrefix(message, prefix))
			}
			if reconstructed.String() != tt.plan.Text {
				t.Fatal("chunk reconstruction differs from original report")
			}
		})
	}
}
