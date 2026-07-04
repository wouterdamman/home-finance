package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionUserKey = "userID"

func NewSessionManager(pool *pgxpool.Pool, secure bool) *scs.SessionManager {
	sm := scs.New()
	sm.Store = &pgxStore{pool: pool}
	sm.Lifetime = 7 * 24 * time.Hour
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = secure
	sm.Cookie.Name = "hf_session"
	return sm
}

type pgxStore struct {
	pool *pgxpool.Pool
}

func (s *pgxStore) Find(token string) ([]byte, bool, error) {
	ctx := context.Background()
	var data []byte
	var expiry time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT data, expiry FROM sessions WHERE token = $1`, token).
		Scan(&data, &expiry)
	if err != nil {
		return nil, false, nil
	}
	if expiry.Before(time.Now()) {
		return nil, false, nil
	}
	return data, true, nil
}

func (s *pgxStore) Commit(token string, data []byte, expiry time.Time) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token, data, expiry) VALUES ($1, $2, $3)
		 ON CONFLICT (token) DO UPDATE SET data = EXCLUDED.data, expiry = EXCLUDED.expiry`,
		token, data, expiry)
	return err
}

func (s *pgxStore) Delete(token string) error {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func GetUserID(sm *scs.SessionManager, ctx context.Context) (int64, bool) {
	id, ok := sm.Get(ctx, sessionUserKey).(int64)
	return id, ok
}
