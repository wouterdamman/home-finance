-- name: UpsertUser :one
INSERT INTO users (oidc_subject, email, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (oidc_subject) DO UPDATE SET email = EXCLUDED.email, display_name = EXCLUDED.display_name
RETURNING *;

-- name: GetUserBySubject :one
SELECT * FROM users WHERE oidc_subject = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;
