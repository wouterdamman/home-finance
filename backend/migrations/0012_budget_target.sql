-- +goose Up

ALTER TABLE categories ADD COLUMN autofill_actual BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE budget_lines ADD COLUMN target_cents_at_close BIGINT;

-- +goose Down

ALTER TABLE budget_lines DROP COLUMN target_cents_at_close;
ALTER TABLE categories DROP COLUMN autofill_actual;
