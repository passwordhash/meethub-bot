-- +goose Up
CREATE TABLE IF NOT EXISTS channel_ignored_users (
    channel_id BIGINT NOT NULL REFERENCES channels(channel_id),
    user_id BIGINT NOT NULL CHECK (user_id > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, user_id)
);
