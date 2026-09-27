package slots

import (
	"time"

	"github.com/google/uuid"
)

type slotsCountEntity struct {
	Count int `db:"count"`
}

type slotEntity struct {
	ID               uuid.UUID  `db:"id"`
	ChannelID        int64      `db:"channel_id"`
	ScheduledAt      time.Time  `db:"scheduled_at"`
	State            string     `db:"state"`
	ChannelMessageID *int64     `db:"channel_message_id"`
	LinkedMessageID  *int64     `db:"linked_message_id"`
	LeadUserID       *int64     `db:"lead_user_id"`
	PublishedAt      *time.Time `db:"published_at"`
	LinkAlertedAt    *time.Time `db:"link_alerted_at"`
}
