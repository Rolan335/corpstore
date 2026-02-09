-- +goose Up
-- +goose StatementBegin
ALTER TABLE files_permissions
    ADD COLUMN IF NOT EXISTS owner_tg_first_name TEXT,
    ADD COLUMN IF NOT EXISTS owner_tg_last_name TEXT,
    ADD COLUMN IF NOT EXISTS owner_tg_username TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE files_permissions
    DROP COLUMN IF EXISTS owner_tg_first_name,
    DROP COLUMN IF EXISTS owner_tg_last_name,
    DROP COLUMN IF EXISTS owner_tg_username;
-- +goose StatementEnd
