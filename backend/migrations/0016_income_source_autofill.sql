-- +goose Up

ALTER TABLE income_sources ADD COLUMN autofill_actual BOOLEAN NOT NULL DEFAULT false;

-- +goose Down

ALTER TABLE income_sources DROP COLUMN autofill_actual;
