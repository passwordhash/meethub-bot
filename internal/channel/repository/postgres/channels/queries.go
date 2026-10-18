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
	return "-- getChannels\nSELECT channel_id, linked_chat_id FROM channels WHERE " + strings.Join(conditions, " AND "), args
}

func getSchedulesQuery(filters models.GetSchedulesFilters) (string, pgx.NamedArgs) {
	query := `-- getSchedulesQuery
SELECT channel_id, post_offset_minutes, daily_lead_enabled, schedule_reset_pending, schedule_checked_at
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

func getConnectionConflictQuery(channel models.Channel) (string, pgx.NamedArgs) {
	return `-- getConnectionConflictQuery
SELECT channel_id, linked_chat_id
FROM channels
WHERE channel_id = @channel_id OR linked_chat_id = @group_id`, pgx.NamedArgs{"channel_id": channel.ChannelID, "group_id": channel.GroupID}
}
