package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationTokenPrefix = "hf_"

// KnownIntegrationScopes is the allowlist of scopes a token may be created
// with. pots:read, periods:read and years:read are planned but deliberately
// not listed until the endpoints that honour them exist.
var KnownIntegrationScopes = []string{"kids:read"}

func ValidIntegrationScope(s string) bool {
	return slices.Contains(KnownIntegrationScopes, s)
}

// NewIntegrationToken mints a token: "hf_" + 32 random bytes, base64url. Only
// the sha256 hash and a short display prefix are ever persisted; the raw value
// is shown to the admin once. A 256-bit random secret needs no slow hash.
func NewIntegrationToken() (raw string, hash []byte, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, "", err
	}
	raw = integrationTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return raw, HashIntegrationToken(raw), raw[:8], nil
}

func HashIntegrationToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

type IntegrationToken struct {
	ID     int64
	Name   string
	Scopes []string
}

const ctxIntegrationToken ctxKey = "integrationToken"

func IntegrationTokenFromCtx(ctx context.Context) (IntegrationToken, bool) {
	t, ok := ctx.Value(ctxIntegrationToken).(IntegrationToken)
	return t, ok
}

// bearerToken extracts the token from "Authorization: Bearer <token>". The
// scheme is case-insensitive; anything but exactly two fields is rejected.
func bearerToken(header string) (string, bool) {
	f := strings.Fields(header)
	if len(f) != 2 || !strings.EqualFold(f[0], "Bearer") {
		return "", false
	}
	return f[1], true
}

func integrationUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, `{"error":{"code":"unauthorized","message":"unauthorized"}}`, http.StatusUnauthorized)
}

// RequireIntegrationToken authenticates a request by bearer token against
// integration_tokens. It never consults the session and is deliberately not
// bypassed by DevFakeAuth: a token endpoint that opens up in dev mode would
// hide auth bugs from the tests that exercise it.
func RequireIntegrationToken(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				integrationUnauthorized(w)
				return
			}
			hash := HashIntegrationToken(raw)
			var tok IntegrationToken
			var dbHash []byte
			var stale bool
			err := pool.QueryRow(r.Context(), `
				SELECT id, name, scopes, token_hash,
				       last_used_at IS NULL OR last_used_at < now() - interval '1 minute'
				FROM integration_tokens
				WHERE token_hash=$1 AND revoked_at IS NULL`, hash).
				Scan(&tok.ID, &tok.Name, &tok.Scopes, &dbHash, &stale)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				// A DB outage is not a bad token: answering 401 would make an
				// integration think its credential was revoked.
				slog.Error("integration token lookup failed", "err", err)
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":{"code":"db_error","message":"internal error"}}`, http.StatusInternalServerError)
				return
			}
			if err != nil || subtle.ConstantTimeCompare(hash, dbHash) != 1 {
				integrationUnauthorized(w)
				return
			}
			if stale {
				// Throttled to once a minute so a polling client doesn't turn
				// every read into a write. Best-effort: never fail the request.
				if _, err := pool.Exec(r.Context(), `
					UPDATE integration_tokens SET last_used_at=now()
					WHERE id=$1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, tok.ID); err != nil {
					slog.Warn("integration token last_used_at update failed", "err", err, "tokenID", tok.ID)
				}
			}
			ctx := context.WithValue(r.Context(), ctxIntegrationToken, tok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope must run after RequireIntegrationToken.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := IntegrationTokenFromCtx(r.Context())
			if !ok {
				integrationUnauthorized(w)
				return
			}
			if !slices.Contains(tok.Scopes, scope) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":{"code":"insufficient_scope","message":"token lacks scope `+scope+`"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
