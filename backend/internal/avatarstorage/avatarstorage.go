package avatarstorage

import (
	"context"
	"errors"
)

// ErrNotFound is returned by Get when the user has no avatar stored.
var ErrNotFound = errors.New("avatar not found")

// Storage is implemented by both the default Postgres-bytea backend and the
// optional filesystem backend (see postgres.go / filesystem.go).
type Storage interface {
	Put(ctx context.Context, userID int64, contentType string, data []byte) error
	Get(ctx context.Context, userID int64) (contentType string, data []byte, err error)
	Delete(ctx context.Context, userID int64) error
}
