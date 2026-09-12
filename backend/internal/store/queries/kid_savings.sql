-- name: ListKids :many
SELECT * FROM kids WHERE archived_at IS NULL ORDER BY sort_order, id;

-- name: UpdateKidReportedBalance :exec
UPDATE kids SET reported_balance_cents = $2, reported_balance_date = $3 WHERE id = $1;

-- name: CreateKidLedgerEntry :one
INSERT INTO kid_savings_ledger (kid_id, owner, entry_type, amount_cents, description, entry_date)
VALUES ($1, $2, $3, $4, $5, COALESCE($6, CURRENT_DATE)) RETURNING *;

-- name: ListKidLedger :many
SELECT id, kid_id, owner, entry_type, amount_cents, description, entry_date, created_at,
       SUM(amount_cents) FILTER (WHERE owner = 'ours') OVER (ORDER BY entry_date, id) AS running_ours,
       SUM(amount_cents) FILTER (WHERE owner = 'theirs') OVER (ORDER BY entry_date, id) AS running_theirs
FROM kid_savings_ledger WHERE kid_id = $1 ORDER BY entry_date, id;

-- name: UpdateKidLedgerEntry :exec
UPDATE kid_savings_ledger SET owner = $2, amount_cents = $3 WHERE id = $1;

-- name: DeleteKidLedgerEntry :exec
DELETE FROM kid_savings_ledger WHERE id = $1;

-- name: ListAllKidBalances :many
SELECT k.id AS kid_id, k.name, k.sort_order,
       COALESCE(SUM(l.amount_cents) FILTER (WHERE l.owner = 'ours'), 0)::BIGINT AS ours_cents,
       COALESCE(SUM(l.amount_cents) FILTER (WHERE l.owner = 'theirs'), 0)::BIGINT AS theirs_cents,
       COALESCE(SUM(l.amount_cents), 0)::BIGINT AS total_cents,
       k.reported_balance_cents, k.reported_balance_date
FROM kids k
LEFT JOIN kid_savings_ledger l ON l.kid_id = k.id
WHERE k.archived_at IS NULL
GROUP BY k.id, k.name, k.sort_order, k.reported_balance_cents, k.reported_balance_date
ORDER BY k.sort_order, k.id;
