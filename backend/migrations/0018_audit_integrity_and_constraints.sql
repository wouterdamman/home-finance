-- +goose Up

-- ---------------------------------------------------------------------------
-- 1. Audit log: stable actor identity.
--
-- audit_log recorded the actor only as user_email, which is a copy of the OIDC
-- email claim refreshed on every login (see upsertUserCtx). An IdP-side address
-- change therefore silently re-pointed every future row and left historical
-- rows unjoinable to the account that wrote them. user_id is the stable key;
-- user_email stays for display and for the admin UI's filter.
-- ---------------------------------------------------------------------------
ALTER TABLE audit_log ADD COLUMN user_id BIGINT REFERENCES users(id);

CREATE INDEX audit_log_user_id_idx ON audit_log (user_id);

-- Backfill what can be matched unambiguously. Rows whose email no longer
-- resolves to exactly one user keep user_id NULL rather than guessing.
UPDATE audit_log al
SET user_id = u.id
FROM users u
WHERE lower(u.email) = lower(al.user_email)
  AND al.user_email <> ''
  AND (SELECT count(*) FROM users u2 WHERE lower(u2.email) = lower(al.user_email)) = 1;

-- ---------------------------------------------------------------------------
-- 2. Audit log: block in-place rewrites.
--
-- The app only ever INSERTs and SELECTs here, but it connects as the schema
-- owner, so anything holding DATABASE_URL could rewrite history undetectably.
-- UPDATE has no legitimate use against an append-only trail. DELETE is left
-- permitted deliberately: retention pruning is a real operational need, and
-- blocking it would force a migration every time the table is trimmed.
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_log_no_update() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log rows are append-only and cannot be updated';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_no_update();

-- ---------------------------------------------------------------------------
-- 3. users.email uniqueness.
--
-- The login upsert conflicts on oidc_subject, so without this a second OIDC
-- identity presenting an existing address created a *new* row rather than
-- colliding — which is what made the INITIAL_ADMIN_EMAILS grant reachable by
-- anyone able to set that address at the IdP.
--
-- If this fails, two user rows already share an address: resolve them by hand
-- (decide which oidc_subject is real) and re-run. Failing the migration is
-- deliberate — it blocks the rollout rather than silently leaving the hole open.
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

-- ---------------------------------------------------------------------------
-- 4. income_entries: one entry per (period, source).
--
-- The importer inserts income entries with ON CONFLICT DO NOTHING, but no
-- matching constraint existed, so the clause only ever covered the identity PK
-- and re-importing a year silently duplicated every income row — inflating the
-- period's income and therefore the surplus that close allocates to pots.
--
-- Partial, because label-only entries (source_id IS NULL) are legitimately
-- repeatable within a period, and carryover entries are keyed separately.
--
-- Guarded: raise with a readable count instead of a bare index-build failure,
-- so an operator hitting pre-existing duplicates knows what to clean up.
-- ---------------------------------------------------------------------------
-- +goose StatementBegin
DO $$
DECLARE
    dupes INT;
BEGIN
    SELECT count(*) INTO dupes FROM (
        SELECT period_id, source_id
        FROM income_entries
        WHERE source_id IS NOT NULL
        GROUP BY period_id, source_id
        HAVING count(*) > 1
    ) d;
    IF dupes > 0 THEN
        RAISE EXCEPTION
            'cannot add income_entries uniqueness: % (period_id, source_id) pair(s) already duplicated. '
            'These are almost certainly the result of a repeated xlsx import. '
            'Review them (SELECT period_id, source_id, count(*) FROM income_entries '
            'WHERE source_id IS NOT NULL GROUP BY 1,2 HAVING count(*) > 1) and delete the extras, then re-run.',
            dupes;
    END IF;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX income_entries_period_source_key
    ON income_entries (period_id, source_id)
    WHERE source_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 5. Missing indexes on foreign keys and hot filter columns.
--
-- pot_ledger and income_entries are cascade targets of periods and are deleted
-- by source_period_id on every reopen, both of which were sequential scans. The
-- two description-suggestion endpoints are the hottest read path in the app
-- (one request per autocomplete keystroke) and filter on category_id/source_id
-- alone, which the existing (period_id, ...) composites cannot serve.
-- ---------------------------------------------------------------------------
CREATE INDEX pot_ledger_source_period_idx   ON pot_ledger (source_period_id);
CREATE INDEX pot_ledger_period_idx          ON pot_ledger (period_id);
CREATE INDEX income_entries_source_period_idx ON income_entries (source_period_id);
CREATE INDEX transactions_category_idx      ON transactions (category_id);
CREATE INDEX income_transactions_source_idx ON income_transactions (source_id);

-- ---------------------------------------------------------------------------
-- 6. pot_splits: bound the percentage.
--
-- CHECK (percentage >= 0) existed, but NaN satisfies it — in Postgres NaN is
-- greater than every non-NaN value, so NaN >= 0 is true. The application now
-- rejects non-finite input at the boundary; this is the backstop that keeps a
-- corrupt value from ever reaching LargestRemainderSplit again.
-- ---------------------------------------------------------------------------
ALTER TABLE pot_splits
    ADD CONSTRAINT pot_splits_percentage_finite
    CHECK (percentage >= 0 AND percentage <= 100 AND percentage = percentage);

-- ---------------------------------------------------------------------------
-- 7. Audit export: record the range each object covers.
--
-- The exporter advances a keyset cursor over audit_log.id. Identity values are
-- allocated at INSERT but only become visible at COMMIT, so a row still in
-- flight when the exporter takes its snapshot was stepped over permanently —
-- an undetectable hole in the off-box copy, which is precisely the thing meant
-- to survive a compromise of this database. The exporter now also skips rows
-- newer than a visibility lag; these columns let a gap or a duplicated range be
-- detected after the fact from the object names alone.
-- ---------------------------------------------------------------------------
ALTER TABLE audit_log_export_state
    ADD COLUMN last_export_first_id BIGINT,
    ADD COLUMN last_export_last_id  BIGINT,
    ADD COLUMN last_export_key      TEXT;

-- +goose Down

ALTER TABLE audit_log_export_state
    DROP COLUMN last_export_key,
    DROP COLUMN last_export_last_id,
    DROP COLUMN last_export_first_id;

ALTER TABLE pot_splits DROP CONSTRAINT pot_splits_percentage_finite;

DROP INDEX income_transactions_source_idx;
DROP INDEX transactions_category_idx;
DROP INDEX income_entries_source_period_idx;
DROP INDEX pot_ledger_period_idx;
DROP INDEX pot_ledger_source_period_idx;

DROP INDEX income_entries_period_source_key;

DROP INDEX users_email_lower_key;

DROP TRIGGER audit_log_no_update ON audit_log;
DROP FUNCTION audit_log_no_update();

DROP INDEX audit_log_user_id_idx;
ALTER TABLE audit_log DROP COLUMN user_id;
