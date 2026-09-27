-- +goose Up
ALTER TABLE publication_slots DROP CONSTRAINT IF EXISTS publication_slots_state_check;
ALTER TABLE publication_slots ADD CONSTRAINT publication_slots_state_check CHECK (state IN ('planned', 'attempting', 'unknown', 'published', 'missed'));
ALTER TABLE channels ADD COLUMN IF NOT EXISTS schedule_checked_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE channels DROP COLUMN IF EXISTS schedule_checked_at;
