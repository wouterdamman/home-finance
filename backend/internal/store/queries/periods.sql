-- name: CreatePeriod :one
INSERT INTO periods (year, month) VALUES ($1, $2) RETURNING *;

-- name: GetPeriod :one
SELECT * FROM periods WHERE id = $1;

-- name: GetPeriodByYearMonth :one
SELECT * FROM periods WHERE year = $1 AND month = $2;

-- name: ListPeriodsByYear :many
SELECT * FROM periods WHERE year = $1 ORDER BY month;

-- name: ClosePeriod :exec
UPDATE periods SET status = 'closed', closed_at = now() WHERE id = $1;

-- name: ReopenPeriod :exec
UPDATE periods SET status = 'open', closed_at = NULL WHERE id = $1;
