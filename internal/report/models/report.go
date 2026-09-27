package models

import (
	"time"

	"github.com/google/uuid"
)

type Report struct {
	SlotID    uuid.UUID
	AuthorID  int64
	Author    string
	Text      string
	MessageID int64
	MessageAt time.Time
	EditAt    *time.Time
	UpdatedAt time.Time
}

type GetReportsFilters struct {
	SlotID *uuid.UUID
}

type GetSnapshotsFilters struct {
	EditionSlotID *uuid.UUID
}

type Snapshot struct {
	ID            uuid.UUID
	EditionSlotID uuid.UUID
	SourceSlotID  uuid.UUID
	AuthorID      int64
	Author        string
	Text          string
	MessageID     int64
	MessageAt     time.Time
	EditAt        *time.Time
	CreatedAt     time.Time
}

type Plan struct {
	AuthorID int64
	Author   string
	Text     string
}
