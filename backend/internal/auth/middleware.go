package auth

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
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
