-- +goose Up
CREATE TABLE IF NOT EXISTS channels (
    channel_id BIGINT PRIMARY KEY,
    linked_chat_id BIGINT NOT NULL UNIQUE,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
