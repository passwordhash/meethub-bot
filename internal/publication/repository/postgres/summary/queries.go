package summary

import (
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/publication/models"
)

func createPartsQuery(editionSlotID uuid.UUID, texts []string) (string, pgx.NamedArgs) {
	ids := make([]uuid.UUID, len(texts))
	indices := make([]int32, len(texts))
	for i := range texts {
		ids[i], indices[i] = editionSlotID, int32(i)
	}
	return `-- createSummaryParts
WITH incoming AS (
    SELECT edition_slot_id, part_index, text
    FROM unnest(@edition_slot_ids::uuid[], @part_indices::int[], @texts::text[]) AS p(edition_slot_id, part_index, text)
), existing_parts AS (
    SELECT part_index, text FROM publication_summary_parts WHERE edition_slot_id = @edition_slot_id
), preflight AS (
    SELECT count(*) AS count,
        NOT EXISTS (
            SELECT 1 FROM existing_parts AS stored
            LEFT JOIN incoming ON incoming.part_index = stored.part_index
            WHERE incoming.part_index IS NULL OR incoming.text IS DISTINCT FROM stored.text
        ) AS content_matches
    FROM existing_parts
), upserted AS (
    INSERT INTO publication_summary_parts (edition_slot_id, part_index, text)
    SELECT incoming.edition_slot_id, incoming.part_index, incoming.text
    FROM incoming CROSS JOIN preflight
    WHERE preflight.content_matches AND (preflight.count = 0 OR preflight.count = @part_count)
    ON CONFLICT (edition_slot_id, part_index) DO UPDATE
    SET text = publication_summary_parts.text
    WHERE publication_summary_parts.text = EXCLUDED.text
    RETURNING part_index
)
SELECT (SELECT count(*) FROM upserted)::int AS accepted, preflight.count::int AS total FROM preflight`, pgx.NamedArgs{"edition_slot_ids": ids, "part_indices": indices, "texts": texts, "edition_slot_id": editionSlotID, "part_count": len(texts)}
}

func getSummaryPartsQuery(filters models.GetSummaryPartsFilters) (string, pgx.NamedArgs) {
	where, args := summaryPartsWhere(filters)
	return "-- getSummaryParts\nSELECT edition_slot_id, part_index, text, status, group_message_id FROM publication_summary_parts WHERE " + where + " ORDER BY edition_slot_id, part_index", args
}

func summaryPartsWhere(filters models.GetSummaryPartsFilters) (string, pgx.NamedArgs) {
	conditions := []string{"1 = 1"}
	args := pgx.NamedArgs{}
	if filters.EditionSlotID != nil {
		conditions = append(conditions, "edition_slot_id = @edition_slot_id")
		args["edition_slot_id"] = *filters.EditionSlotID
	}
	return strings.Join(conditions, " AND "), args
}

func listAttemptingChannelsQuery() string {
	return `-- listAttemptingSummaryChannels
SELECT DISTINCT publication_slots.channel_id
FROM publication_slots
JOIN publication_summary_parts ON publication_summary_parts.edition_slot_id = publication_slots.id
WHERE publication_summary_parts.status = 'attempting'`
}

func transitionQuery(id uuid.UUID, index int, from, to string, messageID *int64) (string, pgx.NamedArgs) {
	return `-- transitionSummaryPart
UPDATE publication_summary_parts SET status = @to_status, group_message_id = @group_message_id
WHERE edition_slot_id = @edition_slot_id AND part_index = @part_index AND status = @from_status AND text IS NOT NULL`, pgx.NamedArgs{"edition_slot_id": id, "part_index": index, "from_status": from, "to_status": to, "group_message_id": messageID}
}
