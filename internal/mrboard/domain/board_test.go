package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

var now = time.Date(2026, 10, 29, 11, 32, 0, 0, time.UTC)

func reviewers(pairs ...string) []models.Reviewer {
	result := make([]models.Reviewer, 0, len(pairs)/2)
	for index := 0; index < len(pairs); index += 2 {
		result = append(result, models.Reviewer{Username: pairs[index], State: models.ReviewerState(pairs[index+1])})
	}
	return result
}

func TestIsTeam(t *testing.T) {
	participants := map[string]string{"yarik": "@yarik"}
	tests := []struct {
		name string
		mr   models.MergeRequest
		want bool
	}{
		{"participant author", models.MergeRequest{Author: "Yarik", Reviewers: reviewers("dima", "unreviewed")}, true},
		{"participant reviewer", models.MergeRequest{Author: "tema", Reviewers: reviewers("dima", "approved", "yarik", "unreviewed")}, true},
		{"strangers only", models.MergeRequest{Author: "tema", Reviewers: reviewers("dima", "unreviewed")}, false},
		{"no reviewers", models.MergeRequest{Author: "yarik"}, false},
		{"draft", models.MergeRequest{Author: "yarik", Draft: true, Reviewers: reviewers("dima", "unreviewed")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTeam(tt.mr, participants); got != tt.want {
				t.Fatalf("IsTeam = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestBall(t *testing.T) {
	tests := []struct {
		state models.ReviewerState
		want  string
	}{
		{models.StateUnreviewed, "reviewer"},
		{models.StateReviewed, "reviewer"},
		{models.StateRequestedChanges, "author"},
		{models.StateApproved, ""},
	}
	for _, tt := range tests {
		if got := Ball(tt.state); got != tt.want {
			t.Errorf("Ball(%s) = %q; want %q", tt.state, got, tt.want)
		}
	}
}

func TestSectionOf(t *testing.T) {
	tests := []struct {
		name      string
		reviewers []models.Reviewer
		want      models.Section
	}{
		{"unreviewed", reviewers("a", "unreviewed"), models.SectionAwaitingReview},
		{"reviewed", reviewers("a", "reviewed", "b", "approved"), models.SectionAwaitingReview},
		{"review beats changes", reviewers("a", "requested_changes", "b", "unreviewed"), models.SectionAwaitingReview},
		{"changes", reviewers("a", "approved", "b", "requested_changes"), models.SectionAwaitingChanges},
		{"all approved", reviewers("a", "approved", "b", "approved"), models.SectionApproved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SectionOf(tt.reviewers); got != tt.want {
				t.Fatalf("SectionOf = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestTrack(t *testing.T) {
	earlier := now.Add(-2 * time.Hour)
	updated := now.Add(-5 * time.Hour)
	mr := func(pairs ...string) models.MergeRequest {
		return models.MergeRequest{ProjectID: 1, IID: 7, UpdatedAt: updated, Reviewers: reviewers(pairs...)}
	}
	tests := []struct {
		name          string
		mrs           []models.MergeRequest
		reviewers     []models.ReviewerTrack
		sections      []models.SectionTrack
		wantReviewers []models.ReviewerTrack
		wantSections  []models.SectionTrack
	}{
		{
			name:          "first sight starts at the last update",
			mrs:           []models.MergeRequest{mr("a", "unreviewed", "b", "approved")},
			wantReviewers: []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateUnreviewed, ChangedAt: updated}, {ProjectID: 1, IID: 7, Username: "b", State: models.StateApproved, ReviewedOnce: true, ChangedAt: updated}},
			wantSections:  []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: updated}},
		},
		{
			name:          "re-request after changes marks a re-review",
			mrs:           []models.MergeRequest{mr("a", "unreviewed")},
			reviewers:     []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateRequestedChanges, ChangedAt: earlier}},
			sections:      []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingChanges, Since: earlier}},
			wantReviewers: []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateUnreviewed, ReviewedOnce: true, ChangedAt: now}},
			wantSections:  []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: now}},
		},
		{
			name:          "unchanged status keeps its time",
			mrs:           []models.MergeRequest{mr("a", "reviewed")},
			reviewers:     []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateReviewed, ReviewedOnce: true, ChangedAt: earlier}},
			sections:      []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: earlier}},
			wantReviewers: []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateReviewed, ReviewedOnce: true, ChangedAt: earlier}},
			wantSections:  []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: earlier}},
		},
		{
			name:          "reviewed to unreviewed within the same section keeps the section time",
			mrs:           []models.MergeRequest{mr("a", "unreviewed")},
			reviewers:     []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateReviewed, ChangedAt: earlier}},
			sections:      []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: earlier}},
			wantReviewers: []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateUnreviewed, ReviewedOnce: true, ChangedAt: now}},
			wantSections:  []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionAwaitingReview, Since: earlier}},
		},
		{
			name:          "merged and draft merge requests are forgotten",
			mrs:           []models.MergeRequest{{ProjectID: 1, IID: 8, Draft: true, Reviewers: reviewers("a", "unreviewed")}},
			reviewers:     []models.ReviewerTrack{{ProjectID: 1, IID: 7, Username: "a", State: models.StateApproved}},
			sections:      []models.SectionTrack{{ProjectID: 1, IID: 7, Section: models.SectionApproved}},
			wantReviewers: []models.ReviewerTrack{},
			wantSections:  []models.SectionTrack{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReviewers, gotSections := Track(tt.mrs, tt.reviewers, tt.sections, now)
			if !reflect.DeepEqual(gotReviewers, tt.wantReviewers) {
				t.Errorf("reviewers = %+v; want %+v", gotReviewers, tt.wantReviewers)
			}
			if !reflect.DeepEqual(gotSections, tt.wantSections) {
				t.Errorf("sections = %+v; want %+v", gotSections, tt.wantSections)
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		age  time.Duration
		want string
	}{
		{-time.Minute, "0м"},
		{59 * time.Minute, "59м"},
		{3*time.Hour + 59*time.Minute, "3ч"},
		{47 * time.Hour, "1д"},
		{50 * time.Hour, "2д"},
	}
	for _, tt := range tests {
		if got := FormatAge(tt.age); got != tt.want {
			t.Errorf("FormatAge(%s) = %q; want %q", tt.age, got, tt.want)
		}
	}
}

func TestRenderBoard(t *testing.T) {
	participants := map[string]string{"tema": "Тёма", "yarik": "@yarik", "dima": "Дима", "danya": "Даня", "lesha": "Лёша"}
	mrs := []models.MergeRequest{
		{ProjectID: 1, IID: 406, Reference: "cascade!406", Title: "лимит 10000", Author: "tema", Reviewers: reviewers("yarik", "unreviewed")},
		{ProjectID: 2, IID: 8, Reference: "support-bot!8", Title: "сапорт бота", Author: "dima", Reviewers: reviewers("yarik", "unreviewed")},
		{ProjectID: 1, IID: 410, Reference: "cascade!410", Title: "Dofobet", Author: "danya", Reviewers: reviewers("dima", "requested_changes", "stranger", "approved")},
		{ProjectID: 1, IID: 399, Reference: "cascade!399", Title: "ProxyPay", Author: "lesha", Reviewers: reviewers("yarik", "approved")},
		{ProjectID: 1, IID: 500, Reference: "cascade!500", Title: "чужой", Author: "x", Reviewers: reviewers("y", "unreviewed")},
	}
	reviewerTracks := []models.ReviewerTrack{{ProjectID: 2, IID: 8, Username: "yarik", State: models.StateUnreviewed, ReviewedOnce: true}}
	sectionTracks := []models.SectionTrack{
		{ProjectID: 1, IID: 406, Since: now.Add(-50 * time.Hour)},
		{ProjectID: 2, IID: 8, Since: now.Add(-3 * time.Hour)},
		{ProjectID: 1, IID: 410, Since: now.Add(-25 * time.Hour)},
		{ProjectID: 1, IID: 399, Since: now.Add(-5 * time.Hour)},
	}
	items := Items(mrs, reviewerTracks, sectionTracks, participants, now)
	got := Render(RenderBody(items, participants, now), now)
	want := `📋 MR команды · обновлено 14:32

👀 Ждёт ревью
• cascade!406 лимит 10000 — Тёма → @yarik · 2д
• support-bot!8 сапорт бота — Дима → @yarik 🔁 · 3ч

✍️ Ждёт правок автора
• cascade!410 Dofobet — Даня ← Дима ✍️, stranger ✅ · 1д

✅ Апрув, ждёт merge
• cascade!399 ProxyPay — Лёша · @yarik ✅ · 5ч`
	if got != want {
		t.Fatalf("board =\n%s\nwant\n%s", got, want)
	}
}

func TestRenderEmptyBoard(t *testing.T) {
	if got := RenderBody(nil, nil, now); got != "Открытых MR команды нет." {
		t.Fatalf("empty body = %q", got)
	}
}

func TestRenderBodyFitsTelegramLimit(t *testing.T) {
	items := make([]Item, 0, 200)
	for index := range 200 {
		items = append(items, Item{
			MergeRequest: models.MergeRequest{Reference: "project!1", Title: strings.Repeat("т", 40), Author: "author", Reviewers: reviewers("reviewer", "unreviewed")},
			Section:      models.SectionAwaitingReview,
			Since:        now.Add(-time.Duration(index) * time.Minute),
		})
	}
	body := RenderBody(items, nil, now)
	if !strings.HasSuffix(body, " MR") || !strings.Contains(body, "…и ещё ") {
		t.Fatalf("truncated body has no counter: %q", body[len(body)-40:])
	}
	if length := utf8.RuneCountInString(Render(body, now)); length > MaxMessageLength {
		t.Fatalf("board length = %d", length)
	}
}
