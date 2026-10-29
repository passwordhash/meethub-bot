package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

// MaxMessageLength is Telegram's limit for a message text.
const MaxMessageLength = 4096

var moscow = time.FixedZone("UTC+3", 3*60*60)

type Item struct {
	MergeRequest models.MergeRequest
	Section      models.Section
	Since        time.Time
	Rereviews    map[string]bool
}

type trackKey struct {
	projectID int64
	iid       int64
	username  string
}

// Tracked reports whether a merge request can reach a board, so its review history is worth keeping.
func Tracked(mr models.MergeRequest) bool {
	return !mr.Draft && len(mr.Reviewers) > 0
}

// IsTeam reports whether a merge request belongs on the board: participants are keyed by lowercase GitLab username.
func IsTeam(mr models.MergeRequest, participants map[string]string) bool {
	if !Tracked(mr) {
		return false
	}
	if _, found := participants[strings.ToLower(mr.Author)]; found {
		return true
	}
	for _, reviewer := range mr.Reviewers {
		if _, found := participants[strings.ToLower(reviewer.Username)]; found {
			return true
		}
	}
	return false
}

// Ball names who has to act: "reviewer", "author" or "" when nobody does.
func Ball(state models.ReviewerState) string {
	switch state {
	case models.StateUnreviewed, models.StateReviewed:
		return "reviewer"
	case models.StateRequestedChanges:
		return "author"
	}
	return ""
}

// SectionOf picks the board section by the worst reviewer status, in board order.
func SectionOf(reviewers []models.Reviewer) models.Section {
	section := models.SectionApproved
	for _, reviewer := range reviewers {
		switch Ball(reviewer.State) {
		case "reviewer":
			return models.SectionAwaitingReview
		case "author":
			section = models.SectionAwaitingChanges
		}
	}
	return section
}

func firstSeen(mr models.MergeRequest, now time.Time) time.Time {
	if mr.UpdatedAt.IsZero() || mr.UpdatedAt.After(now) {
		return now
	}
	return mr.UpdatedAt
}

// Track advances the remembered reviewer statuses and sections to the merge requests GitLab reports now.
// Pairs and merge requests that are no longer tracked are dropped from the result.
func Track(mrs []models.MergeRequest, reviewers []models.ReviewerTrack, sections []models.SectionTrack, now time.Time) ([]models.ReviewerTrack, []models.SectionTrack) {
	previousReviewers := make(map[trackKey]models.ReviewerTrack, len(reviewers))
	for _, track := range reviewers {
		previousReviewers[trackKey{track.ProjectID, track.IID, track.Username}] = track
	}
	previousSections := make(map[trackKey]models.SectionTrack, len(sections))
	for _, track := range sections {
		previousSections[trackKey{projectID: track.ProjectID, iid: track.IID}] = track
	}
	nextReviewers := make([]models.ReviewerTrack, 0, len(reviewers))
	nextSections := make([]models.SectionTrack, 0, len(sections))
	for _, mr := range mrs {
		if !Tracked(mr) {
			continue
		}
		seen := firstSeen(mr, now)
		for _, reviewer := range mr.Reviewers {
			track, found := previousReviewers[trackKey{mr.ProjectID, mr.IID, reviewer.Username}]
			switch {
			case !found:
				track = models.ReviewerTrack{ProjectID: mr.ProjectID, IID: mr.IID, Username: reviewer.Username, State: reviewer.State, ReviewedOnce: reviewer.State != models.StateUnreviewed, ChangedAt: seen}
			case track.State != reviewer.State:
				track.ReviewedOnce = track.ReviewedOnce || track.State != models.StateUnreviewed || reviewer.State != models.StateUnreviewed
				track.State, track.ChangedAt = reviewer.State, now
			}
			nextReviewers = append(nextReviewers, track)
		}
		section := SectionOf(mr.Reviewers)
		track, found := previousSections[trackKey{projectID: mr.ProjectID, iid: mr.IID}]
		switch {
		case !found:
			track = models.SectionTrack{ProjectID: mr.ProjectID, IID: mr.IID, Section: section, Since: seen}
		case track.Section != section:
			track.Section, track.Since = section, now
		}
		nextSections = append(nextSections, track)
	}
	return nextReviewers, nextSections
}

// Items selects the team merge requests and attaches their section, its start and the re-review marks.
func Items(mrs []models.MergeRequest, reviewers []models.ReviewerTrack, sections []models.SectionTrack, participants map[string]string, now time.Time) []Item {
	rereviews := make(map[trackKey]bool, len(reviewers))
	for _, track := range reviewers {
		rereviews[trackKey{track.ProjectID, track.IID, track.Username}] = track.State == models.StateUnreviewed && track.ReviewedOnce
	}
	since := make(map[trackKey]time.Time, len(sections))
	for _, track := range sections {
		since[trackKey{projectID: track.ProjectID, iid: track.IID}] = track.Since
	}
	items := make([]Item, 0, len(mrs))
	for _, mr := range mrs {
		if !IsTeam(mr, participants) {
			continue
		}
		item := Item{MergeRequest: mr, Section: SectionOf(mr.Reviewers), Rereviews: make(map[string]bool)}
		item.Since = since[trackKey{projectID: mr.ProjectID, iid: mr.IID}]
		if item.Since.IsZero() {
			item.Since = firstSeen(mr, now)
		}
		for _, reviewer := range mr.Reviewers {
			if reviewer.State == models.StateUnreviewed && rereviews[trackKey{mr.ProjectID, mr.IID, reviewer.Username}] {
				item.Rereviews[reviewer.Username] = true
			}
		}
		items = append(items, item)
	}
	return items
}

var sectionTitles = []struct {
	section models.Section
	title   string
	arrow   string
}{
	{models.SectionAwaitingReview, "👀 Ждёт ревью", " → "},
	{models.SectionAwaitingChanges, "✍️ Ждёт правок автора", " ← "},
	{models.SectionApproved, "✅ Апрув, ждёт merge", " · "},
}

var stateBadges = map[models.ReviewerState]string{
	models.StateReviewed:         " 💬",
	models.StateRequestedChanges: " ✍️",
	models.StateApproved:         " ✅",
}

// FormatAge shortens how long a merge request has been in its section: minutes, hours, then days.
func FormatAge(age time.Duration) string {
	switch {
	case age < time.Hour:
		return fmt.Sprintf("%dм", max(age, 0)/time.Minute)
	case age < 24*time.Hour:
		return fmt.Sprintf("%dч", age/time.Hour)
	}
	return fmt.Sprintf("%dд", age/(24*time.Hour))
}

func person(username string, participants map[string]string) string {
	if label, found := participants[strings.ToLower(username)]; found {
		return label
	}
	return username
}

func renderLine(item Item, arrow string, participants map[string]string, now time.Time) string {
	mr := item.MergeRequest
	reviewers := make([]string, 0, len(mr.Reviewers))
	for _, reviewer := range mr.Reviewers {
		badge := stateBadges[reviewer.State]
		if item.Rereviews[reviewer.Username] {
			badge = " 🔁"
		}
		reviewers = append(reviewers, person(reviewer.Username, participants)+badge)
	}
	return fmt.Sprintf("• %s %s — %s%s%s · %s", mr.Reference, strings.TrimSpace(mr.Title), person(mr.Author, participants), arrow, strings.Join(reviewers, ", "), FormatAge(now.Sub(item.Since)))
}

// RenderBody lays out the board sections; longest waiting merge requests come first.
// Lines that do not fit into a Telegram message are summarised by a counter.
func RenderBody(items []Item, participants map[string]string, now time.Time) string {
	sorted := append([]Item(nil), items...)
	sort.SliceStable(sorted, func(left, right int) bool {
		if !sorted[left].Since.Equal(sorted[right].Since) {
			return sorted[left].Since.Before(sorted[right].Since)
		}
		return sorted[left].MergeRequest.Reference < sorted[right].MergeRequest.Reference
	})
	var blocks []string
	for _, section := range sectionTitles {
		lines := []string{section.title}
		for _, item := range sorted {
			if item.Section == section.section {
				lines = append(lines, renderLine(item, section.arrow, participants, now))
			}
		}
		if len(lines) > 1 {
			blocks = append(blocks, strings.Join(lines, "\n"))
		}
	}
	if len(blocks) == 0 {
		return "Открытых MR команды нет."
	}
	return fit(strings.Join(blocks, "\n\n"))
}

// headerReserve leaves room for the header that Render prepends.
const headerReserve = 64

func fit(body string) string {
	if utf8.RuneCountInString(body) <= MaxMessageLength-headerReserve {
		return body
	}
	lines := strings.Split(body, "\n")
	for hidden := 1; hidden <= len(lines); hidden++ {
		kept := lines[:len(lines)-hidden]
		dropped := 0
		for _, line := range lines[len(lines)-hidden:] {
			if strings.HasPrefix(line, "• ") {
				dropped++
			}
		}
		candidate := strings.TrimRight(strings.Join(kept, "\n"), "\n") + fmt.Sprintf("\n\n…и ещё %d MR", dropped)
		if utf8.RuneCountInString(candidate) <= MaxMessageLength-headerReserve {
			return candidate
		}
	}
	return ""
}

func Render(body string, updatedAt time.Time) string {
	return "📋 MR команды · обновлено " + updatedAt.In(moscow).Format("15:04") + "\n\n" + body
}
