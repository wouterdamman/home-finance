package avatarstorage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStorage struct {
	pool *pgxpool.Pool
}

// NewPostgres is the default avatar storage backend — stores the image
// bytes directly in the users table. No extra infra required.
func NewPostgres(pool *pgxpool.Pool) Storage {
	return &postgresStorage{pool: pool}
}

func (s *postgresStorage) Put(ctx context.Context, userID int64, contentType string, data []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET avatar_data=$2, avatar_content_type=$3 WHERE id=$1`, userID, data, contentType)
	return err
}

func (s *postgresStorage) Get(ctx context.Context, userID int64) (string, []byte, error) {
	var contentType *string
	var data []byte
	if err := s.pool.QueryRow(ctx, `SELECT avatar_content_type, avatar_data FROM users WHERE id=$1`, userID).Scan(&contentType, &data); err != nil {
		return "", nil, err
	}
	if contentType == nil || data == nil {
		return "", nil, ErrNotFound
	}
	return *contentType, data, nil
}

func (s *postgresStorage) Delete(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET avatar_data=NULL, avatar_content_type=NULL WHERE id=$1`, userID)
	return err
}
