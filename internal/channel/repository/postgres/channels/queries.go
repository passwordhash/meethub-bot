package channels

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/channel/models"
)

func createQuery(channel models.Channel) (string, pgx.NamedArgs) {
	return `-- createQuery
INSERT INTO channels (channel_id, linked_chat_id)
VALUES (@channel_id, @group_id)
ON CONFLICT DO NOTHING
RETURNING channel_id, linked_chat_id`, pgx.NamedArgs{"channel_id": channel.ChannelID, "group_id": channel.GroupID}
}

func getChannelsQuery(filters models.GetChannelsFilters) (string, pgx.NamedArgs) {
	conditions := []string{"1 = 1"}
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		conditions = append(conditions, "channel_id = @channel_id")
		args["channel_id"] = *filters.ChannelID
	}
	if filters.GroupID != nil {
		conditions = append(conditions, "linked_chat_id = @group_id")
		args["group_id"] = *filters.GroupID
	}
	return "-- getChannels\nSELECT channel_id, linked_chat_id, board_view FROM channels WHERE " + strings.Join(conditions, " AND "), args
}

func getSchedulesQuery(filters models.GetSchedulesFilters) (string, pgx.NamedArgs) {
	query := `-- getSchedulesQuery
SELECT channel_id, post_offset_minutes, daily_lead_enabled, schedule_reset_pending, schedule_checked_at, board_view
FROM channels`
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		query += `
WHERE channel_id = @channel_id`
		args["channel_id"] = *filters.ChannelID
	}
	return query, args
}

func getDailyTimesQuery(filters models.GetSchedulesFilters) (string, pgx.NamedArgs) {
	query := `-- getDailyTimesQuery
SELECT channel_id, weekday, (EXTRACT(EPOCH FROM starts_at) / 60)::INT AS starts_at_minutes
FROM channel_daily_times`
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		query += `
WHERE channel_id = @channel_id`
		args["channel_id"] = *filters.ChannelID
	}
	return query, args
}

func setDailyTimeQuery(channelID int64, weekday time.Weekday, startsAt time.Duration) (string, pgx.NamedArgs) {
	return `-- setDailyTimeQuery
WITH channel AS (
    UPDATE channels SET schedule_checked_at = COALESCE(schedule_checked_at, now()) WHERE channel_id = @channel_id
    RETURNING channel_id
)
INSERT INTO channel_daily_times (channel_id, weekday, starts_at)
SELECT channel_id, @weekday::SMALLINT, make_time(@starts_at_minutes::INT / 60, @starts_at_minutes::INT % 60, 0) FROM channel
ON CONFLICT (channel_id, weekday) DO UPDATE SET starts_at = EXCLUDED.starts_at
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "weekday": int16(weekday), "starts_at_minutes": int32(startsAt / time.Minute)}
}

func deleteDailyTimeQuery(channelID int64, weekday time.Weekday) (string, pgx.NamedArgs) {
	return `-- deleteDailyTimeQuery
DELETE FROM channel_daily_times WHERE channel_id = @channel_id AND weekday = @weekday`, pgx.NamedArgs{"channel_id": channelID, "weekday": int16(weekday)}
}

func setPostOffsetQuery(channelID int64, minutes int) (string, pgx.NamedArgs) {
	return `-- setPostOffsetQuery
UPDATE channels SET post_offset_minutes = @minutes WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "minutes": minutes}
}

func setLeadEnabledQuery(channelID int64, enabled bool) (string, pgx.NamedArgs) {
	return `-- setDailyLeadEnabled
UPDATE channels SET daily_lead_enabled = @enabled WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "enabled": enabled}
}

func setBoardViewQuery(channelID int64, view models.BoardView) (string, pgx.NamedArgs) {
	return `-- setBoardViewQuery
UPDATE channels SET board_view = @view::SMALLINT WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "view": int16(view)}
}

func clearScheduleResetQuery(channelID int64) (string, pgx.NamedArgs) {
	return `-- clearScheduleResetQuery
UPDATE channels SET schedule_reset_pending = FALSE WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID}
}

func setScheduleCheckedAtQuery(channelID int64, checkedAt time.Time) (string, pgx.NamedArgs) {
	return `-- setScheduleCheckedAtQuery
UPDATE channels SET schedule_checked_at = @checked_at WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "checked_at": checkedAt}
}

func getIgnoredUsersQuery(filters models.GetIgnoredUsersFilters) (string, pgx.NamedArgs) {
	conditions := []string{"1 = 1"}
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		conditions = append(conditions, "channel_id = @channel_id")
		args["channel_id"] = *filters.ChannelID
	}
	if filters.UserID != nil {
		conditions = append(conditions, "user_id = @user_id")
		args["user_id"] = *filters.UserID
	}
	return "-- getIgnoredUsersQuery\nSELECT channel_id, user_id FROM channel_ignored_users WHERE " + strings.Join(conditions, " AND ") + "\nORDER BY created_at, user_id", args
}

func addIgnoredUserQuery(channelID, userID int64) (string, pgx.NamedArgs) {
	return `-- addIgnoredUserQuery
WITH channel AS (
    SELECT channel_id FROM channels WHERE channel_id = @channel_id
), inserted AS (
    INSERT INTO channel_ignored_users (channel_id, user_id)
    SELECT channel_id, @user_id FROM channel
    ON CONFLICT (channel_id, user_id) DO NOTHING
)
SELECT channel_id FROM channel`, pgx.NamedArgs{"channel_id": channelID, "user_id": userID}
}

func deleteIgnoredUserQuery(channelID, userID int64) (string, pgx.NamedArgs) {
	return `-- deleteIgnoredUserQuery
DELETE FROM channel_ignored_users WHERE channel_id = @channel_id AND user_id = @user_id`, pgx.NamedArgs{"channel_id": channelID, "user_id": userID}
}

func getParticipantsQuery(filters models.GetParticipantsFilters) (string, pgx.NamedArgs) {
	query := `-- getParticipantsQuery
SELECT channel_id, gitlab_username, telegram_label FROM channel_participants`
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		query += `
WHERE channel_id = @channel_id`
		args["channel_id"] = *filters.ChannelID
	}
	return query + "\nORDER BY created_at, gitlab_username", args
}

func setParticipantQuery(participant models.Participant) (string, pgx.NamedArgs) {
	return `-- setParticipantQuery
WITH channel AS (
    SELECT channel_id FROM channels WHERE channel_id = @channel_id
), upserted AS (
    INSERT INTO channel_participants (channel_id, gitlab_username, telegram_label)
    SELECT channel_id, @gitlab_username, @telegram_label FROM channel
    ON CONFLICT (channel_id, gitlab_username) DO UPDATE SET telegram_label = EXCLUDED.telegram_label
)
SELECT channel_id FROM channel`, pgx.NamedArgs{"channel_id": participant.ChannelID, "gitlab_username": participant.GitLabUsername, "telegram_label": participant.TelegramLabel}
}

func deleteParticipantQuery(channelID int64, gitLabUsername string) (string, pgx.NamedArgs) {
	return `-- deleteParticipantQuery
DELETE FROM channel_participants WHERE channel_id = @channel_id AND gitlab_username = @gitlab_username`, pgx.NamedArgs{"channel_id": channelID, "gitlab_username": gitLabUsername}
}

func getConnectionConflictQuery(channel models.Channel) (string, pgx.NamedArgs) {
	return `-- getConnectionConflictQuery
SELECT channel_id, linked_chat_id
FROM channels
WHERE channel_id = @channel_id OR linked_chat_id = @group_id`, pgx.NamedArgs{"channel_id": channel.ChannelID, "group_id": channel.GroupID}
}
