package boards

import "time"

type boardEntity struct {
	ChannelID int64     `db:"channel_id"`
	MessageID int64     `db:"message_id"`
	Body      string    `db:"body"`
	Format    string    `db:"format"`
	UpdatedAt time.Time `db:"updated_at"`
}

type reviewerTrackEntity struct {
	ProjectID    int64     `db:"project_id"`
	IID          int64     `db:"mr_iid"`
	Username     string    `db:"username"`
	State        string    `db:"state"`
	ReviewedOnce bool      `db:"reviewed_once"`
	ChangedAt    time.Time `db:"changed_at"`
}

type sectionTrackEntity struct {
	ProjectID int64     `db:"project_id"`
	IID       int64     `db:"mr_iid"`
	Section   int16     `db:"section"`
	Since     time.Time `db:"since"`
}
