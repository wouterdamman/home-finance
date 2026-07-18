-- +goose Up
ALTER TABLE income_sources ADD COLUMN is_itemized BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE income_transactions (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_id    BIGINT NOT NULL REFERENCES periods(id) ON DELETE CASCADE,
    source_id    BIGINT NOT NULL REFERENCES income_sources(id),
    amount_cents BIGINT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    tx_date      DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX income_transactions_period_source_idx ON income_transactions (period_id, source_id);

-- +goose Down
DROP TABLE income_transactions;
ALTER TABLE income_sources DROP COLUMN is_itemized;
