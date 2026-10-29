-- +goose Up
CREATE TABLE IF NOT EXISTS channel_participants (
    channel_id BIGINT NOT NULL REFERENCES channels(channel_id),
    gitlab_username TEXT NOT NULL CHECK (gitlab_username <> '' AND gitlab_username = lower(gitlab_username)),
    telegram_label TEXT NOT NULL CHECK (telegram_label <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, gitlab_username)
);

CREATE TABLE IF NOT EXISTS channel_mr_boards (
    channel_id BIGINT PRIMARY KEY REFERENCES channels(channel_id),
    message_id BIGINT NOT NULL CHECK (message_id > 0),
    body TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS mr_sections (
    project_id BIGINT NOT NULL,
    mr_iid BIGINT NOT NULL,
    section SMALLINT NOT NULL CHECK (section BETWEEN 1 AND 3),
    since TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (project_id, mr_iid)
);

CREATE TABLE IF NOT EXISTS mr_reviewer_states (
    project_id BIGINT NOT NULL,
    mr_iid BIGINT NOT NULL,
    username TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('unreviewed', 'reviewed', 'requested_changes', 'approved')),
    reviewed_once BOOLEAN NOT NULL DEFAULT FALSE,
    changed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (project_id, mr_iid, username)
);
