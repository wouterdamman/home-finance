package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxAmountCents caps a single money amount at 1,000,000.00 in the app's
// currency — generous for a family budget, tight enough to catch fat-finger
// or malicious input before it corrupts surplus/allocation math.
const maxAmountCents = 100_000_000

// reauthMaxAge is how long a step-up Authentik re-authentication (see
// auth_handlers.go's handleAuthReauth/finishReauthCallback) stays valid for
// destructive actions. Long enough that deleting a period and then also
// locking the year in the same sitting doesn't force a second popup; short
// enough that an unattended unlocked laptop isn't a standing risk.
const reauthMaxAge = 5 * time.Minute

// reauthIsFresh reports whether the session's last successful step-up
// reauth is still within reauthMaxAge.
func reauthIsFresh(sm sessionGetter, ctx context.Context) bool {
	t, ok := sm.Get(ctx, "reauthAt").(time.Time)
	return ok && time.Since(t) <= reauthMaxAge
}

// sessionGetter is the subset of *scs.SessionManager this file needs,
// declared locally so reauthIsFresh doesn't have to import scs just for a
// type name.
type sessionGetter interface {
	Get(ctx context.Context, key string) any
}

// reauthIsFresh reports whether the caller has completed a step-up reauth
// recently enough to perform a destructive action. Bypassed under
// DevFakeAuth, matching every other auth check in dev (no real Authentik
// account to re-authenticate against there).
func (s *Server) reauthIsFresh(r *http.Request) bool {
	if s.cfg.DevFakeAuth {
		return true
	}
	return reauthIsFresh(s.sm, r.Context())
}

// requireFreshReauth writes a 401 reauth_required response and returns
// false if the caller hasn't completed a step-up reauth recently — the
// frontend recognizes this error code and pops the Authentik reauth
// confirmation window rather than treating it as "session expired."
func (s *Server) requireFreshReauth(w http.ResponseWriter, r *http.Request) bool {
	if s.reauthIsFresh(r) {
		return true
	}
	Error(w, http.StatusUnauthorized, "reauth_required", "please confirm your identity again")
	return false
}

// validateAmountCents rejects negative or unreasonably large amounts.
func validateAmountCents(cents int64) error {
	if cents < 0 {
		return errors.New("amount must not be negative")
	}
	if cents > maxAmountCents {
		return fmt.Errorf("amount exceeds maximum of %d cents", maxAmountCents)
	}
	return nil
}

// validateSignedAmountCents allows negative amounts (e.g. pot withdrawals)
// but still bounds the magnitude.
func validateSignedAmountCents(cents int64) error {
	if cents > maxAmountCents || cents < -maxAmountCents {
		return fmt.Errorf("amount exceeds maximum of %d cents", maxAmountCents)
	}
	return nil
}

// upsertUserCtx creates or refreshes the user row on login. Role is only
// ever set on the INSERT branch (via INITIAL_ADMIN_EMAILS) — an existing
// user's role is deliberately left untouched here so a login never
// silently reverts an admin's role change made via Settings > Users.
func (s *Server) upsertUserCtx(ctx context.Context, sub, email, name string) (int64, error) {
	role := "user"
	for _, e := range s.cfg.InitialAdminEmails {
		if strings.EqualFold(e, email) {
			role = "admin"
			break
		}
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (oidc_subject, email, display_name, role)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (oidc_subject) DO UPDATE SET email = EXCLUDED.email, display_name = EXCLUDED.display_name
		 RETURNING id`,
		sub, email, name, role).Scan(&id)
	return id, err
}

// ensureDevUser bootstraps the single DEV_FAKE_AUTH account as admin —
// there's only ever one local dev user, and it needs full access to
// exercise every admin-gated screen.
func (s *Server) ensureDevUser(ctx context.Context) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, oidc_subject, email, display_name, role)
		 OVERRIDING SYSTEM VALUE
		 VALUES (1, 'dev-user', 'dev@example.com', 'Dev User', 'admin')
		 ON CONFLICT (id) DO UPDATE SET role = 'admin'`)
	if err != nil {
		slog.Warn("ensureDevUser", "err", err)
	}
}

func pathInt64(r *http.Request, key string) (int64, bool) {
	v, err := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return v, err == nil
}

func (s *Server) auditLog(ctx context.Context, action, entityType string, entityID int64, details any) {
	email, _ := s.sm.Get(ctx, "userEmail").(string)
	var detJSON []byte
	if details != nil {
		detJSON, _ = json.Marshal(details)
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (user_email, action, entity_type, entity_id, details) VALUES ($1,$2,$3,$4,$5)`,
		email, action, entityType, entityID, detJSON); err != nil {
		slog.Warn("audit_log insert failed", "err", err)
	}
}

func isPeriodClosed(ctx context.Context, pool *pgxpool.Pool, periodID int64) (bool, error) {
	var status string
	err := pool.QueryRow(ctx, `SELECT status FROM periods WHERE id = $1`, periodID).Scan(&status)
	if err != nil {
		return false, err
	}
	return status == "closed", nil
}

func isYearLocked(ctx context.Context, pool *pgxpool.Pool, year int) (bool, error) {
	var locked bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locked_years WHERE year=$1)`, year).Scan(&locked)
	return locked, err
}

func isPeriodWritable(ctx context.Context, pool *pgxpool.Pool, w http.ResponseWriter, periodID int64) bool {
	if closed, _ := isPeriodClosed(ctx, pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return false
	}
	if locked, _ := isYearLockedForPeriod(ctx, pool, periodID); locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return false
	}
	return true
}

func isYearLockedForPeriod(ctx context.Context, pool *pgxpool.Pool, periodID int64) (bool, error) {
	var locked bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM locked_years ly
			JOIN periods p ON p.year = ly.year
			WHERE p.id = $1
		)`, periodID).Scan(&locked)
	return locked, err
}
