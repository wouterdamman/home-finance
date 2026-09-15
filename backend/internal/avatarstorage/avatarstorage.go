package avatarstorage

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Get when the user has no avatar stored.
var ErrNotFound = errors.New("avatar not found")

// Storage is implemented by the Postgres-bytea backend in postgres.go, which
// is the only backend — avatars moved out of S3/filesystem storage and into
// the database, and GET /api/users/:id/avatar proxies the bytes through
// session auth rather than serving them from a bucket URL.
type Storage interface {
	Put(ctx context.Context, userID int64, contentType string, data []byte) error
	Get(ctx context.Context, userID int64) (contentType string, data []byte, err error)
	Delete(ctx context.Context, userID int64) error
}
