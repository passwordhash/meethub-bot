package boards

import (
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

func getBoardsQuery(filters models.GetBoardsFilters) (string, pgx.NamedArgs) {
	query := `-- getBoardsQuery
SELECT channel_id, message_id, body, updated_at FROM channel_mr_boards`
	args := pgx.NamedArgs{}
	if filters.ChannelID != nil {
		query += `
WHERE channel_id = @channel_id`
		args["channel_id"] = *filters.ChannelID
	}
	return query, args
}

func saveBoardQuery(board models.Board) (string, pgx.NamedArgs) {
	return `-- saveBoardQuery
INSERT INTO channel_mr_boards (channel_id, message_id, body, updated_at)
VALUES (@channel_id, @message_id, @body, @updated_at)
ON CONFLICT (channel_id) DO UPDATE SET message_id = EXCLUDED.message_id, body = EXCLUDED.body, updated_at = EXCLUDED.updated_at`,
		pgx.NamedArgs{"channel_id": board.ChannelID, "message_id": board.MessageID, "body": board.Body, "updated_at": board.UpdatedAt}
}

func getReviewerTracksQuery() string {
	return `-- getReviewerTracksQuery
SELECT project_id, mr_iid, username, state, reviewed_once, changed_at FROM mr_reviewer_states`
}

func getSectionTracksQuery() string {
	return `-- getSectionTracksQuery
SELECT project_id, mr_iid, section, since FROM mr_sections`
}

// saveTracksQuery replaces the remembered history with the given one in a single statement:
// the deleted and upserted rows never overlap, so the data-modifying CTEs do not conflict.
func saveTracksQuery(reviewers []models.ReviewerTrack, sections []models.SectionTrack) (string, pgx.NamedArgs) {
	reviewerProjects, reviewerIIDs, usernames, states, reviewedOnce, changedAt := make([]int64, 0, len(reviewers)), make([]int64, 0, len(reviewers)), make([]string, 0, len(reviewers)), make([]string, 0, len(reviewers)), make([]bool, 0, len(reviewers)), make([]time.Time, 0, len(reviewers))
	for _, track := range reviewers {
		reviewerProjects, reviewerIIDs, usernames = append(reviewerProjects, track.ProjectID), append(reviewerIIDs, track.IID), append(usernames, track.Username)
		states, reviewedOnce, changedAt = append(states, string(track.State)), append(reviewedOnce, track.ReviewedOnce), append(changedAt, track.ChangedAt)
	}
	sectionProjects, sectionIIDs, sectionValues, since := make([]int64, 0, len(sections)), make([]int64, 0, len(sections)), make([]int16, 0, len(sections)), make([]time.Time, 0, len(sections))
	for _, track := range sections {
		sectionProjects, sectionIIDs = append(sectionProjects, track.ProjectID), append(sectionIIDs, track.IID)
		sectionValues, since = append(sectionValues, int16(track.Section)), append(since, track.Since)
	}
	return `-- saveTracksQuery
WITH reviewers AS (
    SELECT * FROM unnest(@reviewer_projects::BIGINT[], @reviewer_iids::BIGINT[], @usernames::TEXT[], @states::TEXT[], @reviewed_once::BOOLEAN[], @changed_at::TIMESTAMPTZ[])
        AS t(project_id, mr_iid, username, state, reviewed_once, changed_at)
), sections AS (
    SELECT * FROM unnest(@section_projects::BIGINT[], @section_iids::BIGINT[], @sections::SMALLINT[], @since::TIMESTAMPTZ[])
        AS t(project_id, mr_iid, section, since)
), stale_reviewers AS (
    DELETE FROM mr_reviewer_states s
    WHERE NOT EXISTS (SELECT 1 FROM reviewers r WHERE r.project_id = s.project_id AND r.mr_iid = s.mr_iid AND r.username = s.username)
), stale_sections AS (
    DELETE FROM mr_sections s
    WHERE NOT EXISTS (SELECT 1 FROM sections r WHERE r.project_id = s.project_id AND r.mr_iid = s.mr_iid)
), saved_reviewers AS (
    INSERT INTO mr_reviewer_states (project_id, mr_iid, username, state, reviewed_once, changed_at)
    SELECT project_id, mr_iid, username, state, reviewed_once, changed_at FROM reviewers
    ON CONFLICT (project_id, mr_iid, username) DO UPDATE
    SET state = EXCLUDED.state, reviewed_once = EXCLUDED.reviewed_once, changed_at = EXCLUDED.changed_at
)
INSERT INTO mr_sections (project_id, mr_iid, section, since)
SELECT project_id, mr_iid, section, since FROM sections
ON CONFLICT (project_id, mr_iid) DO UPDATE SET section = EXCLUDED.section, since = EXCLUDED.since`, pgx.NamedArgs{
		"reviewer_projects": reviewerProjects, "reviewer_iids": reviewerIIDs, "usernames": usernames, "states": states, "reviewed_once": reviewedOnce, "changed_at": changedAt,
		"section_projects": sectionProjects, "section_iids": sectionIIDs, "sections": sectionValues, "since": since,
	}
}
