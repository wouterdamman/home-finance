-- name: UpsertPotSplit :one
INSERT INTO pot_splits (period_id, pot_id, percentage)
VALUES ($1, $2, $3)
ON CONFLICT (period_id, pot_id) DO UPDATE SET percentage = EXCLUDED.percentage
RETURNING *;

-- name: ListPotSplits :many
SELECT ps.*, p.name AS pot_name, p.kind AS pot_kind
FROM pot_splits ps
JOIN pots p ON p.id = ps.pot_id
WHERE ps.period_id = $1
ORDER BY p.sort_order, p.id;

-- name: DeletePotSplitsForPeriod :exec
DELETE FROM pot_splits WHERE period_id = $1;

-- name: SumPotSplitPercentages :one
SELECT COALESCE(SUM(percentage), 0)::NUMERIC AS total FROM pot_splits WHERE period_id = $1;
