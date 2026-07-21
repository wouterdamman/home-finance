package avatarstorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

type filesystemStorage struct {
	root string
	pool *pgxpool.Pool
}

// NewFilesystem stores avatar bytes as files under root (e.g. a mounted
// PersistentVolume) and keeps only the content-type and file path in
// Postgres (avatar_content_type, avatar_object_key columns).
func NewFilesystem(root string, pool *pgxpool.Pool) Storage {
	return &filesystemStorage{root: root, pool: pool}
}

func (s *filesystemStorage) path(userID int64) string {
	return filepath.Join(s.root, fmt.Sprintf("avatar-%d", userID))
}

func (s *filesystemStorage) Put(ctx context.Context, userID int64, contentType string, data []byte) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	p := s.path(userID)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE users SET avatar_content_type=$2, avatar_object_key=$3 WHERE id=$1`, userID, contentType, p)
	return err
}

func (s *filesystemStorage) Get(ctx context.Context, userID int64) (string, []byte, error) {
	var contentType *string
	var objectKey *string
	if err := s.pool.QueryRow(ctx, `SELECT avatar_content_type, avatar_object_key FROM users WHERE id=$1`, userID).Scan(&contentType, &objectKey); err != nil {
		return "", nil, err
	}
	if contentType == nil || objectKey == nil {
		return "", nil, ErrNotFound
	}
	data, err := os.ReadFile(*objectKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, ErrNotFound
		}
		return "", nil, err
	}
	return *contentType, data, nil
}

func (s *filesystemStorage) Delete(ctx context.Context, userID int64) error {
	var objectKey *string
	if err := s.pool.QueryRow(ctx, `SELECT avatar_object_key FROM users WHERE id=$1`, userID).Scan(&objectKey); err == nil && objectKey != nil {
		_ = os.Remove(*objectKey)
	}
	_, err := s.pool.Exec(ctx, `UPDATE users SET avatar_content_type=NULL, avatar_object_key=NULL WHERE id=$1`, userID)
	return err
}
