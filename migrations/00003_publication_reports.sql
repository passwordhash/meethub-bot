-- +goose Up
CREATE TABLE IF NOT EXISTS publication_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id BIGINT NOT NULL REFERENCES channels(channel_id),
    scheduled_at TIMESTAMPTZ NOT NULL,
    state TEXT NOT NULL DEFAULT 'planned' CHECK (state IN ('planned', 'attempting', 'unknown', 'published')),
    channel_message_id BIGINT,
    linked_message_id BIGINT,
    previous_lead_user_id BIGINT,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel_id, scheduled_at)
);
CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_id UUID NOT NULL REFERENCES publication_slots(id),
    user_id BIGINT NOT NULL,
    display_name TEXT NOT NULL,
    text TEXT NOT NULL,
    message_id BIGINT NOT NULL,
    message_at TIMESTAMPTZ NOT NULL,
    edit_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (slot_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_reports_slot_message_at ON reports (slot_id, message_at DESC, message_id DESC, edit_at DESC);
CREATE TABLE IF NOT EXISTS report_snapshot_editions (
    edition_slot_id UUID PRIMARY KEY REFERENCES publication_slots(id),
    source_slot_id UUID NOT NULL REFERENCES publication_slots(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS report_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_slot_id UUID NOT NULL REFERENCES report_snapshot_editions(edition_slot_id),
    source_slot_id UUID NOT NULL REFERENCES publication_slots(id),
    user_id BIGINT NOT NULL,
    display_name TEXT NOT NULL,
    text TEXT NOT NULL,
    message_id BIGINT NOT NULL,
    message_at TIMESTAMPTZ NOT NULL,
    edit_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (edition_slot_id, user_id)
);
CREATE TABLE IF NOT EXISTS telegram_update_offset (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    next_update_id BIGINT NOT NULL DEFAULT 0
);
INSERT INTO telegram_update_offset (id, next_update_id) VALUES (1, 0) ON CONFLICT (id) DO NOTHING;

-- +goose Down
