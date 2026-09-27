-- +goose Up
CREATE TABLE IF NOT EXISTS publication_summary_parts (
    edition_slot_id UUID NOT NULL REFERENCES publication_slots(id),
    part_index INTEGER NOT NULL CHECK (part_index >= 0),
    text TEXT CHECK (text IS NULL OR length(text) > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'attempting', 'unknown', 'sent')),
    group_message_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (edition_slot_id, part_index),
    CHECK ((status = 'sent') = (group_message_id IS NOT NULL)),
    CHECK (status <> 'sent' OR group_message_id > 0)
);
