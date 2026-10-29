package models

import "time"

type ReviewerState string

const (
	StateUnreviewed       ReviewerState = "unreviewed"
	StateReviewed         ReviewerState = "reviewed"
	StateRequestedChanges ReviewerState = "requested_changes"
	StateApproved         ReviewerState = "approved"
)

func (s ReviewerState) Valid() bool {
	switch s {
	case StateUnreviewed, StateReviewed, StateRequestedChanges, StateApproved:
		return true
	}
	return false
}

type Section int

const (
	SectionAwaitingReview  Section = 1
	SectionAwaitingChanges Section = 2
	SectionApproved        Section = 3
)

type MergeRequest struct {
	ProjectID int64
	IID       int64
	Reference string
	WebURL    string
	Title     string
	Author    string
	Draft     bool
	UpdatedAt time.Time
	Reviewers []Reviewer
}

type Reviewer struct {
	Username string
	State    ReviewerState
}

// ReviewerTrack is the remembered status of a "merge request × reviewer" pair:
// GitLab keeps no history, so the bot derives re-reviews from it.
type ReviewerTrack struct {
	ProjectID    int64
	IID          int64
	Username     string
	State        ReviewerState
	ReviewedOnce bool
	ChangedAt    time.Time
}

type SectionTrack struct {
	ProjectID int64
	IID       int64
	Section   Section
	Since     time.Time
}

// Board message formats; plain is the pre-rich board, remembered only to migrate its pin.
type BoardFormat string

const (
	BoardFormatPlain BoardFormat = "plain"
	BoardFormatRich  BoardFormat = "rich"
)

type Board struct {
	ChannelID int64
	MessageID int64
	Body      string
	Format    BoardFormat
	UpdatedAt time.Time
}

type GetBoardsFilters struct {
	ChannelID *int64
}
