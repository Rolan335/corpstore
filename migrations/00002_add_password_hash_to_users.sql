-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
