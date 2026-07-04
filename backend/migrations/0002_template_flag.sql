-- +goose Up
ALTER TABLE income_sources ADD COLUMN include_in_template BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE categories     ADD COLUMN include_in_template BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE income_sources DROP COLUMN include_in_template;
ALTER TABLE categories     DROP COLUMN include_in_template;
