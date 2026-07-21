-- +goose Up

ALTER TABLE users ADD COLUMN avatar_data BYTEA;
ALTER TABLE users ADD COLUMN avatar_content_type TEXT;

-- +goose Down

ALTER TABLE users DROP COLUMN avatar_content_type;
ALTER TABLE users DROP COLUMN avatar_data;
