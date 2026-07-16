-- name: CreatePot :one
INSERT INTO pots (name, kind, sort_order, target_cents, target_date) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListPots :many
SELECT * FROM pots WHERE archived_at IS NULL ORDER BY sort_order, id;

-- name: GetPot :one
SELECT * FROM pots WHERE id = $1;

-- name: UpdatePot :one
UPDATE pots SET name = $2, sort_order = $3, target_cents = $4, target_date = $5 WHERE id = $1 RETURNING *;

-- name: ArchivePot :exec
UPDATE pots SET archived_at = now() WHERE id = $1;

-- name: GetCarryoverPot :one
SELECT * FROM pots WHERE kind = 'carryover' AND archived_at IS NULL LIMIT 1;
