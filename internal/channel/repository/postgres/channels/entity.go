package channels

import "time"

type channelEntity struct {
	ChannelID int64 `db:"channel_id"`
	GroupID   int64 `db:"linked_chat_id"`
}

type scheduleEntity struct {
	ChannelID   int64      `db:"channel_id"`
	Cron        *string    `db:"schedule_cron"`
	LeadEnabled bool       `db:"daily_lead_enabled"`
	Override    *time.Time `db:"next_override"`
	CheckedAt   *time.Time `db:"schedule_checked_at"`
}
