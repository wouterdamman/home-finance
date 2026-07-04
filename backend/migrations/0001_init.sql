-- +goose Up

CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    oidc_subject  TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE periods (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    year      INT NOT NULL CHECK (year BETWEEN 2000 AND 2100),
    month     INT NOT NULL CHECK (month BETWEEN 1 AND 12),
    status    TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    closed_at TIMESTAMPTZ,
    UNIQUE (year, month)
);

CREATE TABLE income_sources (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                 TEXT NOT NULL,
    default_amount_cents BIGINT NOT NULL DEFAULT 0,
    sort_order           INT NOT NULL DEFAULT 0,
    archived_at          TIMESTAMPTZ
);

CREATE TABLE income_entries (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_id        BIGINT NOT NULL REFERENCES periods(id) ON DELETE CASCADE,
    source_id        BIGINT REFERENCES income_sources(id),
    label            TEXT,
    amount_cents     BIGINT NOT NULL,
    entry_type       TEXT NOT NULL DEFAULT 'normal'
                     CHECK (entry_type IN ('normal','carryover')),
    source_period_id BIGINT REFERENCES periods(id) ON DELETE CASCADE,
    notes            TEXT NOT NULL DEFAULT '',
    sort_order       INT NOT NULL DEFAULT 0,
    CHECK (source_id IS NOT NULL OR label IS NOT NULL)
);
CREATE INDEX income_entries_period_idx ON income_entries (period_id);

CREATE TABLE categories (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                 TEXT NOT NULL,
    default_amount_cents BIGINT NOT NULL DEFAULT 0,
    is_itemized          BOOLEAN NOT NULL DEFAULT false,
    sort_order           INT NOT NULL DEFAULT 0,
    archived_at          TIMESTAMPTZ
);

CREATE TABLE budget_lines (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_id           BIGINT NOT NULL REFERENCES periods(id) ON DELETE CASCADE,
    category_id         BIGINT REFERENCES categories(id),
    label               TEXT,
    amount_cents        BIGINT NOT NULL DEFAULT 0,
    tracks_transactions BOOLEAN NOT NULL DEFAULT false,
    sort_order          INT NOT NULL DEFAULT 0,
    UNIQUE (period_id, category_id),
    CHECK (category_id IS NOT NULL OR label IS NOT NULL)
);

CREATE TABLE transactions (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_id    BIGINT NOT NULL REFERENCES periods(id) ON DELETE CASCADE,
    category_id  BIGINT NOT NULL REFERENCES categories(id),
    amount_cents BIGINT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    tx_date      DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_period_cat_idx ON transactions (period_id, category_id);

CREATE TABLE pots (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'normal' CHECK (kind IN ('normal','carryover')),
    sort_order  INT NOT NULL DEFAULT 0,
    archived_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX pots_one_carryover_idx ON pots (kind) WHERE kind = 'carryover' AND archived_at IS NULL;

CREATE TABLE pot_splits (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_id  BIGINT NOT NULL REFERENCES periods(id) ON DELETE CASCADE,
    pot_id     BIGINT NOT NULL REFERENCES pots(id),
    percentage NUMERIC(5,2) NOT NULL CHECK (percentage >= 0),
    UNIQUE (period_id, pot_id)
);

CREATE TABLE pot_ledger (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pot_id           BIGINT NOT NULL REFERENCES pots(id),
    period_id        BIGINT REFERENCES periods(id) ON DELETE CASCADE,
    source_period_id BIGINT REFERENCES periods(id) ON DELETE CASCADE,
    entry_type       TEXT NOT NULL CHECK (entry_type IN
                     ('allocation','carryover_out','withdrawal','deposit','adjustment','opening_balance')),
    amount_cents     BIGINT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    entry_date       DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX pot_ledger_pot_idx ON pot_ledger (pot_id, entry_date);

-- +goose Down
DROP TABLE IF EXISTS pot_ledger;
DROP TABLE IF EXISTS pot_splits;
DROP TABLE IF EXISTS pots;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS budget_lines;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS income_entries;
DROP TABLE IF EXISTS income_sources;
DROP TABLE IF EXISTS periods;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
