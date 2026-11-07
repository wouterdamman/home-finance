-- +goose Up

CREATE TABLE integration_tokens (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    token_hash   BYTEA NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes       TEXT[] NOT NULL,
    created_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

-- +goose Down

DROP TABLE integration_tokens;
