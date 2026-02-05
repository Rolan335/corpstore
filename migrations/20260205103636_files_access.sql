-- +goose Up
-- +goose StatementBegin
CREATE TABLE files_permissions(
    id uuid primary key default gen_random_uuid(),
    file_id UUID not null references files(id),
    user_granted UUID not null references users(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
