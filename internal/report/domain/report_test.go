package domain

import (
	"reflect"
	"testing"

	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func TestExtractPlans(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "plain heading", text: "Итог недели\nСегодня:\nСделать релиз\nПроверить отчёты\nЗавтра:\nПозвонить", want: "Сделать релиз\nПроверить отчёты"},
		{name: "markdown and punctuation", text: "## На сегодня!\nПочинить 🐛\n### Итоги\nГотово", want: "Починить 🐛"},
		{name: "bold heading", text: "**Сделаю сегодня:**\nЗадача", want: "Задача"},
		{name: "similar heading", text: "Задачи на сегодня —\nПроверить ✅", want: "Проверить ✅"},
		{name: "inline plan and next heading", text: "Сегодня: Сделать релиз\nЗавтра: Позвонить 李", want: "Сделать релиз"},
		{name: "task label with colon is not a heading", text: "Сегодня:\nСделать релиз\nПодготовить: тесты\nПроверить отчёт", want: "Сделать релиз\nПодготовить: тесты\nПроверить отчёт"},
		{name: "task label with colon after empty heading avoids fallback", text: "Сегодня:\nПодготовить: тесты\nПроверить отчёт", want: "Подготовить: тесты\nПроверить отчёт"},
		{name: "recognized inline section boundary", text: "Сегодня:\nПодготовить релиз\nИтоги: выполнено", want: "Подготовить релиз"},
		{name: "empty plan section falls back", text: "Сегодня:\nЗавтра: Созвон", want: "Сегодня:\nЗавтра: Созвон"},
		{name: "inline full-width punctuation", text: "На сегодня： проверить Unicode — готово\nИтоги: отлично", want: "проверить Unicode — готово"},
		{name: "fallback without clear section", text: "Общий текст\n- пункт", want: "Общий текст\n- пункт"},
		{name: "no report text omitted", text: " \n ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPlans([]models.Report{{Author: "Автор", Text: tt.text}})
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("got %#v, want no plans", got)
				}
				return
			}
			if len(got) != 1 || got[0].Author != "Автор" || got[0].Text != tt.want {
				t.Fatalf("got %#v, want plan %q", got, tt.want)
			}
		})
	}
}

func TestExtractPlansMultipleAuthorsAndNoReports(t *testing.T) {
	tests := []struct {
		name    string
		reports []models.Report
		want    []models.Plan
	}{
		{name: "multiple authors", reports: []models.Report{{Author: "Анна", Text: "Сегодня:\nА"}, {Author: "李", Text: "Текст"}}, want: []models.Plan{{Author: "Анна", Text: "А"}, {Author: "李", Text: "Текст"}}},
		{name: "no reports", want: []models.Plan{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractPlans(tt.reports); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestEligibleLeadIDs(t *testing.T) {
	tests := []struct {
		name       string
		reports    []models.Report
		previousID int64
		want       []int64
	}{
		{name: "exclude previous when alternatives", reports: []models.Report{{AuthorID: 1}, {AuthorID: 2}, {AuthorID: 1}}, previousID: 1, want: []int64{2}},
		{name: "sole candidate repeats", reports: []models.Report{{AuthorID: 7}}, previousID: 7, want: []int64{7}},
		{name: "empty reports", want: []int64{}},
		{name: "owner reporting included and unique", reports: []models.Report{{AuthorID: -4}, {AuthorID: 8}, {AuthorID: -4}}, previousID: 8, want: []int64{-4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EligibleLeadIDs(tt.reports, tt.previousID); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
