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
SELECT channel_id, schedule_cron, daily_lead_enabled, next_override, schedule_checked_at
FROM channels`
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		query += `
WHERE channel_id = @channel_id`
		args["channel_id"] = *filters.ChannelID
	}
	return query, args
}

func setScheduleQuery(schedule models.Schedule, keepOverride bool) (string, pgx.NamedArgs) {
	return `-- setScheduleQuery
UPDATE channels SET schedule_cron = @cron,
next_override = CASE WHEN @keep_override THEN next_override ELSE NULL END, schedule_checked_at = now()
WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": schedule.ChannelID, "cron": schedule.Cron, "keep_override": keepOverride}
}

func setLeadEnabledQuery(channelID int64, enabled bool) (string, pgx.NamedArgs) {
	return `-- setDailyLeadEnabled
UPDATE channels SET daily_lead_enabled = @enabled WHERE channel_id = @channel_id
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "enabled": enabled}
}

func setOverrideQuery(channelID int64, target time.Time) (string, pgx.NamedArgs) {
	return `-- setOverrideQuery
UPDATE channels SET next_override = @target, schedule_checked_at = now()
WHERE channel_id = @channel_id AND schedule_cron IS NOT NULL
RETURNING channel_id`, pgx.NamedArgs{"channel_id": channelID, "target": target}
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
