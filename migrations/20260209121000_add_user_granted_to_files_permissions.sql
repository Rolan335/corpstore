-- +goose Up
-- +goose StatementBegin
ALTER TABLE files_permissions
    ADD COLUMN IF NOT EXISTS user_granted UUID REFERENCES users(id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE files_permissions
    DROP COLUMN IF EXISTS user_granted;
-- +goose StatementEnd
