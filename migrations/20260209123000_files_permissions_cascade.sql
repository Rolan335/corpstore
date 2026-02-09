-- +goose Up
-- +goose StatementBegin
ALTER TABLE files_permissions
    DROP CONSTRAINT IF EXISTS files_permissions_file_id_fkey;

ALTER TABLE files_permissions
    ADD CONSTRAINT files_permissions_file_id_fkey
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE files_permissions
    DROP CONSTRAINT IF EXISTS files_permissions_file_id_fkey;

ALTER TABLE files_permissions
    ADD CONSTRAINT files_permissions_file_id_fkey
    FOREIGN KEY (file_id) REFERENCES files(id);
-- +goose StatementEnd
