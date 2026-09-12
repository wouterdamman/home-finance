-- +goose Up

CREATE TABLE kids (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                  TEXT NOT NULL,
    sort_order            INT NOT NULL DEFAULT 0,
    archived_at           TIMESTAMPTZ,
    reported_balance_cents BIGINT,
    reported_balance_date  DATE
);

INSERT INTO kids (name, sort_order) VALUES
    ('Lize', 0),
    ('Stan', 1),
    ('Quinn', 2);

CREATE TABLE kid_savings_ledger (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kid_id       BIGINT NOT NULL REFERENCES kids(id),
    owner        TEXT NOT NULL CHECK (owner IN ('ours','theirs')),
    entry_type   TEXT NOT NULL CHECK (entry_type IN
                 ('opening_balance','deposit','withdrawal','adjustment')),
    amount_cents BIGINT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    entry_date   DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX kid_savings_ledger_kid_idx ON kid_savings_ledger (kid_id, entry_date);

-- +goose Down

DROP TABLE IF EXISTS kid_savings_ledger;
DROP TABLE IF EXISTS kids;
