-- +goose Up

ALTER TABLE integration_tokens ADD COLUMN expires_at TIMESTAMPTZ;
UPDATE integration_tokens SET expires_at = created_at + interval '1 year';
ALTER TABLE integration_tokens ALTER COLUMN expires_at SET NOT NULL;

-- +goose Down

ALTER TABLE integration_tokens DROP COLUMN expires_at;
