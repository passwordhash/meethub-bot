-- +goose Up
ALTER TABLE channels ADD COLUMN board_pin_last BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE channels DROP COLUMN board_pin_last;
