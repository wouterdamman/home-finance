-- name: CreateCategory :one
INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListCategories :many
SELECT * FROM categories WHERE archived_at IS NULL ORDER BY sort_order, id;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: UpdateCategory :one
UPDATE categories SET name = $2, default_amount_cents = $3, is_itemized = $4, sort_order = $5
WHERE id = $1 RETURNING *;

-- name: ArchiveCategory :exec
UPDATE categories SET archived_at = now() WHERE id = $1;

-- name: ReorderCategories :exec
UPDATE categories SET sort_order = $2 WHERE id = $1;
