-- +goose Up
CREATE TABLE audit_log (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_email  TEXT,
    action      TEXT NOT NULL,
    entity_type TEXT,
    entity_id   BIGINT,
    details     JSONB
);

-- +goose Down
DROP TABLE audit_log;
