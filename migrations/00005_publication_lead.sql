-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'publication_slots' AND column_name = 'previous_lead_user_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'publication_slots' AND column_name = 'lead_user_id'
    ) THEN
        ALTER TABLE publication_slots RENAME COLUMN previous_lead_user_id TO lead_user_id;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'publication_slots' AND column_name = 'lead_user_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'publication_slots' AND column_name = 'previous_lead_user_id'
    ) THEN
        ALTER TABLE publication_slots RENAME COLUMN lead_user_id TO previous_lead_user_id;
    END IF;
END
$$;
-- +goose StatementEnd
