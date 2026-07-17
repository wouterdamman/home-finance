-- +goose Up
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user'));
ALTER TABLE users ADD COLUMN avatar_object_key TEXT NULL;

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_object_key;
ALTER TABLE users DROP COLUMN role;
