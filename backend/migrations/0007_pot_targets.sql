-- +goose Up
ALTER TABLE pots ADD COLUMN target_cents BIGINT NULL;
ALTER TABLE pots ADD COLUMN target_date DATE NULL;

-- +goose Down
ALTER TABLE pots DROP COLUMN target_date;
ALTER TABLE pots DROP COLUMN target_cents;
