-- +goose Up
ALTER TABLE publication_slots ADD COLUMN IF NOT EXISTS link_alerted_at TIMESTAMPTZ;
