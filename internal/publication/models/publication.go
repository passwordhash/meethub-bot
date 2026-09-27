package models

import (
	"time"

	"github.com/google/uuid"
)

type State string

const (
	StatePlanned    State = "planned"
	StateAttempting State = "attempting"
	StateUnknown    State = "unknown"
	StatePublished  State = "published"
	StateMissed     State = "missed"
)

type Slot struct {
	ID               uuid.UUID
	ChannelID        int64
	ScheduledAt      time.Time
	State            State
	ChannelMessageID *int64
	LinkedMessageID  *int64
	LeadUserID       *int64
	PublishedAt      *time.Time
	LinkAlertedAt    *time.Time
}

type GetSlotsFilters struct {
	ID        *uuid.UUID
	ChannelID *int64
	State     []State
}
