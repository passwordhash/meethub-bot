package reports

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/report/models"
)

func saveLatestQuery(report models.Report) (string, pgx.NamedArgs) {
	return `-- saveLatestReport
INSERT INTO reports (slot_id, user_id, display_name, text, message_id, message_at, edit_at)
VALUES (@slot_id, @user_id, @display_name, @text, @message_id, @message_at, @edit_at)
ON CONFLICT (slot_id, user_id) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    text = EXCLUDED.text,
    message_id = EXCLUDED.message_id,
    message_at = EXCLUDED.message_at,
    edit_at = EXCLUDED.edit_at,
    updated_at = now()
WHERE (reports.message_at, reports.message_id, COALESCE(reports.edit_at, reports.message_at))
    < (EXCLUDED.message_at, EXCLUDED.message_id, COALESCE(EXCLUDED.edit_at, EXCLUDED.message_at))`, pgx.NamedArgs{
			"slot_id": report.SlotID, "user_id": report.AuthorID, "display_name": report.Author,
			"text": report.Text, "message_id": report.MessageID, "message_at": report.MessageAt, "edit_at": report.EditAt,
		}
}

func getLatestQuery(filters models.GetReportsFilters) (string, pgx.NamedArgs) {
	return `-- getLatestReports
SELECT id, slot_id, user_id, display_name, text, message_id, message_at, edit_at, updated_at
FROM reports WHERE slot_id = @slot_id ORDER BY user_id`, pgx.NamedArgs{"slot_id": *filters.SlotID}
}

func getLatestCountQuery(filters models.GetReportsFilters) (string, pgx.NamedArgs) {
	return `-- getLatestReportsCount
SELECT COUNT(*) FROM reports WHERE slot_id = @slot_id`, pgx.NamedArgs{"slot_id": *filters.SlotID}
}

func createSnapshotQuery(editionSlotID, sourceSlotID uuid.UUID) (string, pgx.NamedArgs) {
	return `-- createReportSnapshot
WITH source AS MATERIALIZED (
    SELECT id FROM publication_slots WHERE id = @source_slot_id AND state = 'published'
), existing AS MATERIALIZED (
    SELECT source_slot_id FROM report_snapshot_editions WHERE edition_slot_id = @edition_slot_id
), edition AS (
    INSERT INTO report_snapshot_editions (edition_slot_id, source_slot_id)
    SELECT @edition_slot_id, source.id FROM source
    ON CONFLICT (edition_slot_id) DO NOTHING
    RETURNING edition_slot_id, source_slot_id
), snapshot_rows AS (
    INSERT INTO report_snapshots (edition_slot_id, source_slot_id, user_id, display_name, text, message_id, message_at, edit_at)
    SELECT edition.edition_slot_id, edition.source_slot_id, reports.user_id, reports.display_name, reports.text, reports.message_id, reports.message_at, reports.edit_at
    FROM edition JOIN reports ON reports.slot_id = edition.source_slot_id
    WHERE reports.text <> ''
    RETURNING id
)
SELECT CASE
    WHEN EXISTS (SELECT 1 FROM existing WHERE source_slot_id <> @source_slot_id) THEN 'source_conflict'
    WHEN EXISTS (SELECT 1 FROM existing) THEN 'existing'
    WHEN NOT EXISTS (SELECT 1 FROM source) THEN 'source_unavailable'
    WHEN EXISTS (SELECT 1 FROM edition) THEN 'created'
    ELSE 'concurrent_conflict'
END AS outcome`, pgx.NamedArgs{"edition_slot_id": editionSlotID, "source_slot_id": sourceSlotID}
}

func getSnapshotEditionQuery(editionSlotID uuid.UUID) (string, pgx.NamedArgs) {
	return `-- getSnapshotEdition
SELECT source_slot_id FROM report_snapshot_editions WHERE edition_slot_id = @edition_slot_id`, pgx.NamedArgs{"edition_slot_id": editionSlotID}
}

func getSnapshotsQuery(filters models.GetSnapshotsFilters) (string, pgx.NamedArgs) {
	return `-- getReportSnapshots
SELECT id, edition_slot_id, source_slot_id, user_id, display_name, text, message_id, message_at, edit_at, created_at
FROM report_snapshots WHERE edition_slot_id = @edition_slot_id ORDER BY user_id`, pgx.NamedArgs{"edition_slot_id": *filters.EditionSlotID}
}

func getSnapshotsCountQuery(filters models.GetSnapshotsFilters) (string, pgx.NamedArgs) {
	return `-- getReportSnapshotsCount
SELECT COUNT(*) FROM report_snapshots WHERE edition_slot_id = @edition_slot_id`, pgx.NamedArgs{"edition_slot_id": *filters.EditionSlotID}
}

func purgeReportsQuery() string {
	return `-- purgeReportText
UPDATE reports SET text = '' WHERE COALESCE(edit_at, message_at) < now() - interval '30 days'`
}

func purgeSnapshotsQuery() string {
	return `-- purgeSnapshotText
UPDATE report_snapshots SET text = '' WHERE created_at < now() - interval '30 days'`
}
