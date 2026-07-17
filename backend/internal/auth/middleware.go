package auth

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey string

const ctxUserID ctxKey = "userID"

func Require(sm *scs.SessionManager, devFakeAuth bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if devFakeAuth {
				ctx := context.WithValue(r.Context(), ctxUserID, int64(1))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			uid, ok := sm.Get(r.Context(), sessionUserKey).(int64)
			if !ok || uid == 0 {
				http.Error(w, `{"error":{"code":"unauthorized","message":"unauthorized"}}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin looks the caller's role up fresh on every request (rather
// than caching it in the session) so a role change in Settings > Users
// takes effect immediately, without forcing the affected user to log out
// and back in.
func RequireAdmin(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, ok := UserIDFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":{"code":"unauthorized","message":"unauthorized"}}`, http.StatusUnauthorized)
				return
			}
			var role string
			if err := pool.QueryRow(r.Context(), `SELECT role FROM users WHERE id=$1`, uid).Scan(&role); err != nil || role != "admin" {
				http.Error(w, `{"error":{"code":"forbidden","message":"admin required"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			http.Error(w, `{"error":{"code":"csrf","message":"missing X-Requested-With"}}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserIDFromCtx(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(ctxUserID).(int64)
	return id, ok
}
