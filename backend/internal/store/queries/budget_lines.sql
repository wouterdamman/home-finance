-- name: CreateBudgetLine :one
INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListBudgetLines :many
SELECT bl.*,
       COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
                 WHERE t.period_id = bl.period_id AND t.category_id = bl.category_id), 0)::BIGINT AS transactions_total_cents
FROM budget_lines bl
WHERE bl.period_id = $1
ORDER BY bl.sort_order, bl.id;

-- name: UpdateBudgetLine :one
UPDATE budget_lines SET label = $2, amount_cents = $3, tracks_transactions = $4, sort_order = $5
WHERE id = $1 RETURNING *;

-- name: DeleteBudgetLine :exec
DELETE FROM budget_lines WHERE id = $1;

-- name: SumEffectiveBudgetLines :one
SELECT COALESCE(SUM(
  CASE WHEN bl.tracks_transactions THEN
    COALESCE((SELECT SUM(t.amount_cents) FROM transactions t
              WHERE t.period_id = bl.period_id AND t.category_id = bl.category_id), 0)
  ELSE bl.amount_cents END
), 0)::BIGINT AS total
FROM budget_lines bl WHERE bl.period_id = $1;
