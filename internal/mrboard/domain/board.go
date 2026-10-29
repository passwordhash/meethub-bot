package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	channelmodels "github.com/passwordhash/cascade-hub-bot/internal/channel/models"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

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

// maxTitleLength keeps board table rows readable; longer titles are cut with an ellipsis.
const maxTitleLength = 48

var ballBadges = map[models.Section]string{
	models.SectionAwaitingReview:  "👀",
	models.SectionAwaitingChanges: "✍️",
	models.SectionApproved:        "✅",
}

// cellTitle shortens a merge request title for its table cell and keeps pipes and newlines from breaking the table.
func cellTitle(title string) string {
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > maxTitleLength {
		title = string([]rune(title)[:maxTitleLength]) + "…"
	}
	return strings.ReplaceAll(strings.ReplaceAll(title, "|", `\|`), "\n", " ")
}

// referenceCell hyperlinks the merge request reference, the only place the board links to GitLab.
func referenceCell(mr models.MergeRequest) string {
	if mr.WebURL == "" {
		return mr.Reference
	}
	return fmt.Sprintf("[%s](%s)", mr.Reference, mr.WebURL)
}

// rowCells builds the shared MR, title, who and age cells; arrow carries the section's ball direction.
func rowCells(item Item, arrow string, participants map[string]string, now time.Time) []string {
	mr := item.MergeRequest
	reviewers := make([]string, 0, len(mr.Reviewers))
	for _, reviewer := range mr.Reviewers {
		badge := stateBadges[reviewer.State]
		if item.Rereviews[reviewer.Username] {
			badge = " 🔁"
		}
		reviewers = append(reviewers, person(reviewer.Username, participants)+badge)
	}
	who := person(mr.Author, participants) + arrow + strings.Join(reviewers, ", ")
	return []string{referenceCell(mr), cellTitle(mr.Title), who, FormatAge(now.Sub(item.Since))}
}

// table renders a GitHub Flavored Markdown table, the markup of Telegram rich messages.
func table(header []string, rows [][]string) string {
	row := func(cells []string) string {
		parts := make([]string, len(cells))
		for index, cell := range cells {
			parts[index] = " " + cell + " "
		}
		return "|" + strings.Join(parts, "|") + "|"
	}
	separator := "|"
	for range header {
		separator += "---|"
	}
	lines := []string{row(header), separator}
	for _, cells := range rows {
		lines = append(lines, row(cells))
	}
	return strings.Join(lines, "\n")
}

var ballsHeader = []string{"MR", "Титул", "Кто", "⏱"}

func arrowOf(section models.Section) string {
	for _, entry := range sectionTitles {
		if entry.section == section {
			return entry.arrow
		}
	}
	return " · "
}

func titleOf(section models.Section) string {
	for _, entry := range sectionTitles {
		if entry.section == section {
			return entry.title
		}
	}
	return ""
}

// renderByBalls gives each section one table, rows by waiting age.
func renderByBalls(sorted []Item, participants map[string]string, now time.Time) []string {
	var blocks []string
	for _, section := range sectionTitles {
		rows := make([][]string, 0, len(sorted))
		for _, item := range sorted {
			if item.Section == section.section {
				rows = append(rows, rowCells(item, section.arrow, participants, now))
			}
		}
		if len(rows) > 0 {
			blocks = append(blocks, "**"+section.title+"**\n\n"+table(ballsHeader, rows))
		}
	}
	return blocks
}

// renderByReviewer splits the waiting sections into a block per person who owes action:
// a merge request with several waiting reviewers appears in each of their blocks.
func renderByReviewer(sorted []Item, participants map[string]string, now time.Time) []string {
	var blocks []string
	reviewerOrder := []string{}
	rowsByReviewer := map[string][][]string{}
	authorOrder := []string{}
	rowsByAuthor := map[string][][]string{}
	approvedRows := [][]string{}
	for _, item := range sorted {
		switch item.Section {
		case models.SectionAwaitingReview:
			for _, reviewer := range item.MergeRequest.Reviewers {
				if Ball(reviewer.State) != "reviewer" {
					continue
				}
				if _, seen := rowsByReviewer[reviewer.Username]; !seen {
					reviewerOrder = append(reviewerOrder, reviewer.Username)
					rowsByReviewer[reviewer.Username] = nil
				}
				rowsByReviewer[reviewer.Username] = append(rowsByReviewer[reviewer.Username], rowCells(item, arrowOf(item.Section), participants, now))
			}
		case models.SectionAwaitingChanges:
			author := item.MergeRequest.Author
			if _, seen := rowsByAuthor[author]; !seen {
				authorOrder = append(authorOrder, author)
				rowsByAuthor[author] = nil
			}
			rowsByAuthor[author] = append(rowsByAuthor[author], rowCells(item, arrowOf(item.Section), participants, now))
		case models.SectionApproved:
			approvedRows = append(approvedRows, rowCells(item, arrowOf(item.Section), participants, now))
		}
	}
	if len(reviewerOrder) > 0 {
		blocks = append(blocks, "**"+titleOf(models.SectionAwaitingReview)+"**")
		for _, username := range reviewerOrder {
			blocks = append(blocks, "**"+person(username, participants)+" смотрит**\n\n"+table(ballsHeader, rowsByReviewer[username]))
		}
	}
	if len(authorOrder) > 0 {
		blocks = append(blocks, "**"+titleOf(models.SectionAwaitingChanges)+"**")
		for _, author := range authorOrder {
			blocks = append(blocks, "**"+person(author, participants)+" должен**\n\n"+table(ballsHeader, rowsByAuthor[author]))
		}
	}
	if len(approvedRows) > 0 {
		blocks = append(blocks, "**"+titleOf(models.SectionApproved)+"**\n\n"+table(ballsHeader, approvedRows))
	}
	return blocks
}

var projectHeader = []string{"MR", "Титул", "⚡", "Кто", "⏱"}

// renderByProject gives each project one table, projects in alphabetical order, the ball as a row badge.
func renderByProject(sorted []Item, participants map[string]string, now time.Time) []string {
	projects := []string{}
	rowsByProject := map[string][][]string{}
	for _, item := range sorted {
		// shortcut: the project name is the reference prefix; carry ProjectName in the model if GitLab stops sending references
		project, _, _ := strings.Cut(item.MergeRequest.Reference, "!")
		if project == "" {
			project = "прочее"
		}
		if _, seen := rowsByProject[project]; !seen {
			projects = append(projects, project)
			rowsByProject[project] = nil
		}
		cells := rowCells(item, arrowOf(item.Section), participants, now)
		rowsByProject[project] = append(rowsByProject[project], []string{cells[0], cells[1], ballBadges[item.Section], cells[2], cells[3]})
	}
	sort.Strings(projects)
	blocks := make([]string, 0, len(projects))
	for _, project := range projects {
		blocks = append(blocks, "**"+project+"**\n\n"+table(projectHeader, rowsByProject[project]))
	}
	return blocks
}

func sortedItems(items []Item) []Item {
	sorted := append([]Item(nil), items...)
	sort.SliceStable(sorted, func(left, right int) bool {
		if !sorted[left].Since.Equal(sorted[right].Since) {
			return sorted[left].Since.Before(sorted[right].Since)
		}
		return sorted[left].MergeRequest.Reference < sorted[right].MergeRequest.Reference
	})
	return sorted
}

// RenderBody lays out the board in the channel's view; longest waiting merge requests come first.
func RenderBody(items []Item, participants map[string]string, view channelmodels.BoardView, now time.Time) string {
	sorted := sortedItems(items)
	var blocks []string
	switch view {
	case channelmodels.BoardViewReviewer:
		blocks = renderByReviewer(sorted, participants, now)
	case channelmodels.BoardViewProject:
		blocks = renderByProject(sorted, participants, now)
	default:
		blocks = renderByBalls(sorted, participants, now)
	}
	if len(blocks) == 0 {
		return "Открытых MR команды нет."
	}
	return strings.Join(blocks, "\n\n")
}

func Render(body string, updatedAt time.Time) string {
	return "📋 MR команды · обновлено " + updatedAt.In(moscow).Format("15:04") + "\n\n" + body
}
