-- name: CreateIncomeEntry :one
INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, source_period_id, notes, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: ListIncomeEntries :many
SELECT * FROM income_entries WHERE period_id = $1 ORDER BY sort_order, id;

-- name: SumIncomeEntries :one
SELECT COALESCE(SUM(amount_cents), 0)::BIGINT AS total FROM income_entries WHERE period_id = $1;

-- name: UpdateIncomeEntry :one
UPDATE income_entries SET label = $2, amount_cents = $3, notes = $4, sort_order = $5
WHERE id = $1 RETURNING *;

-- name: DeleteIncomeEntry :exec
DELETE FROM income_entries WHERE id = $1;

-- name: DeleteCarryoverEntriesForSourcePeriod :exec
DELETE FROM income_entries WHERE entry_type = 'carryover' AND source_period_id = $1;
