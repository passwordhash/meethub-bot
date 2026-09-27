package reports

import (
	"time"

	"github.com/google/uuid"
)

type snapshotCreationEntity struct {
	Outcome string `db:"outcome"`
}

type snapshotEditionEntity struct {
	SourceSlotID uuid.UUID `db:"source_slot_id"`
}

type countEntity struct {
	Count int `db:"count"`
}

type reportEntity struct {
	ID        uuid.UUID  `db:"id"`
	SlotID    uuid.UUID  `db:"slot_id"`
	UserID    int64      `db:"user_id"`
	Name      string     `db:"display_name"`
	Text      string     `db:"text"`
	MessageID int64      `db:"message_id"`
	MessageAt time.Time  `db:"message_at"`
	EditAt    *time.Time `db:"edit_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

type snapshotEntity struct {
	ID            uuid.UUID  `db:"id"`
	EditionSlotID uuid.UUID  `db:"edition_slot_id"`
	SourceSlotID  uuid.UUID  `db:"source_slot_id"`
	UserID        int64      `db:"user_id"`
	Name          string     `db:"display_name"`
	Text          string     `db:"text"`
	MessageID     int64      `db:"message_id"`
	MessageAt     time.Time  `db:"message_at"`
	EditAt        *time.Time `db:"edit_at"`
	CreatedAt     time.Time  `db:"created_at"`
}
