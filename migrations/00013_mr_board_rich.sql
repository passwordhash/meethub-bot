-- +goose Up
ALTER TABLE channels
    ADD COLUMN board_view SMALLINT NOT NULL DEFAULT 1 CHECK (board_view BETWEEN 1 AND 3);

ALTER TABLE channel_mr_boards
    ADD COLUMN format TEXT NOT NULL DEFAULT 'plain' CHECK (format IN ('plain', 'rich'));

-- +goose Down
ALTER TABLE channel_mr_boards DROP COLUMN format;
ALTER TABLE channels DROP COLUMN board_view;
