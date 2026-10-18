package slots

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

func createQuery(channelID int64, scheduledAt time.Time) (string, pgx.NamedArgs) {
	return `-- createSlot
INSERT INTO publication_slots (channel_id, scheduled_at)
VALUES (@channel_id, @scheduled_at)
ON CONFLICT (channel_id, scheduled_at) DO NOTHING
RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at, publish_alerted_at`, pgx.NamedArgs{"channel_id": channelID, "scheduled_at": scheduledAt}
}

func getByOccurrenceQuery(channelID int64, scheduledAt time.Time) (string, pgx.NamedArgs) {
	return `-- getSlotByOccurrence
SELECT id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at, publish_alerted_at
FROM publication_slots WHERE channel_id = @channel_id AND scheduled_at = @scheduled_at`, pgx.NamedArgs{"channel_id": channelID, "scheduled_at": scheduledAt}
}

func recoverSlotsQuery() (string, pgx.NamedArgs) {
	return `-- recoverSlots
WITH missed AS (
    UPDATE publication_slots SET state = 'missed'
    WHERE state = 'planned' AND scheduled_at < date_trunc('minute', now())
    RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at
), unknown AS (
    UPDATE publication_slots SET state = 'unknown'
    WHERE state = 'attempting'
    RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at
)
SELECT * FROM missed
UNION ALL
SELECT * FROM unknown`, pgx.NamedArgs{}
}

func getSlotsQuery(filters models.GetSlotsFilters) (string, pgx.NamedArgs) {
	where, args := slotsWhere(filters)
	return "-- getSlots\nSELECT id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at, link_alerted_at, publish_alerted_at FROM publication_slots WHERE " + where + " ORDER BY scheduled_at", args
}

func getSlotsCountQuery(filters models.GetSlotsFilters) (string, pgx.NamedArgs) {
	where, args := slotsWhere(filters)
	return "-- getSlotsCount\nSELECT COUNT(*) FROM publication_slots WHERE " + where, args
}

func slotsWhere(filters models.GetSlotsFilters) (string, pgx.NamedArgs) {
	conditions := []string{"1 = 1"}
	args := pgx.NamedArgs{}
	if filters.ID != nil {
		conditions = append(conditions, "id = @id")
		args["id"] = *filters.ID
	}
	if filters.ChannelID != nil {
		conditions = append(conditions, "channel_id = @channel_id")
		args["channel_id"] = *filters.ChannelID
	}
	if len(filters.State) > 0 {
		conditions = append(conditions, "state = ANY(@states)")
		args["states"] = filters.State
	}
	if filters.ScheduledFrom != nil {
		conditions = append(conditions, "scheduled_at >= @scheduled_from")
		args["scheduled_from"] = *filters.ScheduledFrom
	}
	if filters.ScheduledTo != nil {
		conditions = append(conditions, "scheduled_at < @scheduled_to")
		args["scheduled_to"] = *filters.ScheduledTo
	}
	return strings.Join(conditions, " AND "), args
}

func bindLinkQuery(id uuid.UUID, channelID, groupID, channelMessageID, groupMessageID int64) (string, pgx.NamedArgs) {
	return `-- bindSlotLink
UPDATE publication_slots SET linked_message_id = @group_message_id
WHERE id = @id AND channel_id = @channel_id AND state = 'published' AND channel_message_id = @channel_message_id
AND EXISTS (SELECT 1 FROM channels WHERE channels.channel_id = publication_slots.channel_id AND linked_chat_id = @group_id)
AND (linked_message_id IS NULL OR linked_message_id = @group_message_id)
RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at`, pgx.NamedArgs{"id": id, "channel_id": channelID, "group_id": groupID, "channel_message_id": channelMessageID, "group_message_id": groupMessageID}
}

func setStateQuery(id uuid.UUID, from, to models.State, channelMessageID, linkedMessageID, leadUserID *int64) (string, pgx.NamedArgs) {
	return `-- setSlotState
UPDATE publication_slots SET state = @state,
    channel_message_id = COALESCE(@channel_message_id, channel_message_id),
    linked_message_id = COALESCE(@linked_message_id, linked_message_id),
    lead_user_id = COALESCE(@lead_user_id, lead_user_id),
    published_at = CASE WHEN @state = 'published' THEN now() ELSE published_at END
WHERE id = @id AND state = @from_state
RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at`, pgx.NamedArgs{"id": id, "from_state": from, "state": to, "channel_message_id": channelMessageID, "linked_message_id": linkedMessageID, "lead_user_id": leadUserID}
}

func markLinkAlertedQuery(id uuid.UUID) (string, pgx.NamedArgs) {
	return `-- markLinkAlerted
UPDATE publication_slots SET link_alerted_at = now()
WHERE id = @id AND link_alerted_at IS NULL`, pgx.NamedArgs{"id": id}
}

func markPublishAlertedQuery(id uuid.UUID) (string, pgx.NamedArgs) {
	return `-- markPublishAlerted
UPDATE publication_slots SET publish_alerted_at = now()
WHERE id = @id AND publish_alerted_at IS NULL`, pgx.NamedArgs{"id": id}
}

func setLeadQuery(id uuid.UUID, leadID *int64) (string, pgx.NamedArgs) {
	return `-- setSlotLead
UPDATE publication_slots SET lead_user_id = COALESCE(lead_user_id, @lead_user_id)
WHERE id = @id AND state = 'planned'
RETURNING id, channel_id, scheduled_at, state, channel_message_id, linked_message_id, lead_user_id, published_at`, pgx.NamedArgs{"id": id, "lead_user_id": leadID}
}
