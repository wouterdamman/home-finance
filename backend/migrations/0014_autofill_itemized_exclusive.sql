-- +goose Up

-- autofill_actual is meaningless for itemized categories: the effective
-- amount always comes from summed transactions once tracks_transactions is
-- true, so a stored default is written but never read. Clean up existing
-- rows where both flags ended up true (silent bug, no UI guard existed).
UPDATE categories SET autofill_actual = false WHERE is_itemized = true AND autofill_actual = true;

-- +goose Down

-- no-op: cannot restore which rows previously had both flags set
