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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wouterdamman/home-finance/internal/auth"
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
//
// The INITIAL_ADMIN_EMAILS grant is additionally gated on "no admin exists
// yet", so it can only ever bootstrap the very first account. Previously any
// first-time login presenting a matching email claim was minted as an admin —
// and because the upsert conflicts on oidc_subject rather than email, a second
// identity claiming the same address took the INSERT branch and was granted
// admin instead of colliding with the existing row.
func (s *Server) upsertUserCtx(ctx context.Context, sub, email, name string) (int64, error) {
	role := "user"
	for _, e := range s.cfg.InitialAdminEmails {
		if strings.EqualFold(e, email) {
			var adminExists bool
			if err := s.pool.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM users WHERE role='admin')`).Scan(&adminExists); err != nil {
				return 0, err
			}
			if !adminExists {
				role = "admin"
			}
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

// auditLogTimeout bounds the detached audit insert. Long enough to survive a
// slow write, short enough that a wedged pool can't accumulate goroutines.
const auditLogTimeout = 5 * time.Second

// auditLog records a state-changing action. It runs on a context explicitly
// detached from the request's, because the insert happens *after* the mutation
// has already committed: on the request context, a caller who disconnects at
// the right moment cancels the audit write while the effect stands, and a
// failed attempt costs them nothing to retry until it lands. The record is
// still best-effort with respect to database failures (a hiccup must never
// fail a user-facing request) — it simply can no longer be suppressed by the
// caller.
//
// user_id is the stable identifier; user_email is kept denormalized for
// display and filtering, but it is a mutable claim refreshed on every login,
// so it must not be the only thing tying a row to an account.
func (s *Server) auditLog(ctx context.Context, action, entityType string, entityID int64, details any) {
	email, _ := s.sm.Get(ctx, "userEmail").(string)
	var userID *int64
	if uid, ok := auth.UserIDFromCtx(ctx); ok && uid != 0 {
		userID = &uid
	}
	var detJSON []byte
	if details != nil {
		detJSON, _ = json.Marshal(details)
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditLogTimeout)
	defer cancel()
	if _, err := s.pool.Exec(writeCtx,
		`INSERT INTO audit_log (user_id, user_email, action, entity_type, entity_id, details) VALUES ($1,$2,$3,$4,$5,$6)`,
		userID, email, action, entityType, entityID, detJSON); err != nil {
		// Include enough context to reconstruct the lost record from logs.
		slog.Warn("audit_log insert failed",
			"err", err, "action", action, "entityType", entityType, "entityID", entityID, "userEmail", email)
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

// isPeriodWritable is the only gate protecting a closed period or a locked
// financial year from further writes, so it must fail *closed*. It previously
// discarded both error returns, which collapsed any database failure — a
// connection reset, a pool-acquire timeout, a cancelled context — into
// "writable" and let the write through. Combined with an unbounded pool that
// made exhaustion reachable, that turned the immutability guarantee of a
// locked year into something an attacker could shake loose under load.
func isPeriodWritable(ctx context.Context, pool *pgxpool.Pool, w http.ResponseWriter, periodID int64) bool {
	closed, err := isPeriodClosed(ctx, pool, periodID)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return false
	}
	if err != nil {
		dbError(w, "isPeriodClosed", err)
		return false
	}
	if closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return false
	}
	locked, err := isYearLockedForPeriod(ctx, pool, periodID)
	if err != nil {
		dbError(w, "isYearLockedForPeriod", err)
		return false
	}
	if locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return false
	}
	return true
}

// isYearWritable guards writes keyed on a calendar year rather than a period —
// pot and kid ledger entries carry their own entry_date and have no period
// linkage, so they were never covered by the locked-year check at all.
func isYearWritable(ctx context.Context, pool *pgxpool.Pool, w http.ResponseWriter, year int) bool {
	locked, err := isYearLocked(ctx, pool, year)
	if err != nil {
		dbError(w, "isYearLocked", err)
		return false
	}
	if locked {
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
