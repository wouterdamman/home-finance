-- name: CreatePotLedgerEntry :one
INSERT INTO pot_ledger (pot_id, period_id, source_period_id, entry_type, amount_cents, description, entry_date)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetPotBalance :one
SELECT COALESCE(SUM(amount_cents), 0)::BIGINT AS balance FROM pot_ledger WHERE pot_id = $1;

-- name: ListPotLedger :many
SELECT *, SUM(amount_cents) OVER (PARTITION BY pot_id ORDER BY entry_date, id) AS running_balance
FROM pot_ledger WHERE pot_id = $1 ORDER BY entry_date, id;

-- name: DeletePotLedgerForSourcePeriod :exec
DELETE FROM pot_ledger WHERE source_period_id = $1;

-- name: ListAllPotBalances :many
SELECT p.id AS pot_id, p.name, p.kind, p.sort_order,
       COALESCE(SUM(pl.amount_cents), 0)::BIGINT AS balance
FROM pots p
LEFT JOIN pot_ledger pl ON pl.pot_id = p.id
WHERE p.archived_at IS NULL
GROUP BY p.id, p.name, p.kind, p.sort_order
ORDER BY p.sort_order, p.id;
