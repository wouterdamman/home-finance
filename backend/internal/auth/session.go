package auth

import (
	"context"
	"encoding/gob"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scs's default GobCodec stores session data as map[string]interface{},
// gob-encoded. Any concrete type kept in that interface — beyond gob's
// built-in string/int/etc. — must be registered or encoding fails at
// Commit time with "gob: type not registered for interface: time.Time".
// reauthAt (see auth_handlers.go) is the only time.Time value put in the
// session, so this was silently breaking every reauth completion.
func init() {
	gob.Register(time.Time{})
}

const sessionUserKey = "userID"

// sessionCleanupInterval controls how often expired rows are purged from
// the sessions table. scs's built-in cleanup goroutine only fires for
// stores implementing its cleanup interface, which pgxStore doesn't, so
// without this the table (indexed via sessions_expiry_idx, migration 0001)
// grows one row per login forever.
const sessionCleanupInterval = 1 * time.Hour

func NewSessionManager(pool *pgxpool.Pool, secure bool) *scs.SessionManager {
	sm := scs.New()
	store := &pgxStore{pool: pool}
	store.startCleanup(sessionCleanupInterval)
	sm.Store = store
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

// startCleanup runs for the lifetime of the process — the server has a
// single long-lived session manager, so there's no case where it needs to
// be stopped before process exit.
func (s *pgxStore) startCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := s.pool.Exec(context.Background(), `DELETE FROM sessions WHERE expiry < now()`); err != nil {
				slog.Error("session cleanup", "err", err)
			}
		}
	}()
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
