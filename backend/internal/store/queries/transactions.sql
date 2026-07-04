-- name: CreateTransaction :one
INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE period_id = $1 AND ($2::BIGINT IS NULL OR category_id = $2)
ORDER BY tx_date DESC NULLS LAST, id DESC;

-- name: UpdateTransaction :one
UPDATE transactions SET amount_cents = $2, description = $3, tx_date = $4
WHERE id = $1 RETURNING *;

-- name: DeleteTransaction :exec
DELETE FROM transactions WHERE id = $1;
