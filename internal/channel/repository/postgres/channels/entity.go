package channels

import "time"

type channelEntity struct {
	ChannelID int64 `db:"channel_id"`
	GroupID   int64 `db:"linked_chat_id"`
}

type scheduleEntity struct {
	ChannelID         int64      `db:"channel_id"`
	PostOffsetMinutes int        `db:"post_offset_minutes"`
	LeadEnabled       bool       `db:"daily_lead_enabled"`
	ResetPending      bool       `db:"schedule_reset_pending"`
	CheckedAt         *time.Time `db:"schedule_checked_at"`
}

type dailyTimeEntity struct {
	ChannelID       int64 `db:"channel_id"`
	Weekday         int16 `db:"weekday"`
	StartsAtMinutes int   `db:"starts_at_minutes"`
}

type ignoredUserEntity struct {
	ChannelID int64 `db:"channel_id"`
	UserID    int64 `db:"user_id"`
}
