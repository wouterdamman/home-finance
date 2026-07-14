package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxAmountCents caps a single money amount at 1,000,000.00 in the app's
// currency — generous for a family budget, tight enough to catch fat-finger
// or malicious input before it corrupts surplus/allocation math.
const maxAmountCents = 100_000_000

// checkPassword compares body against the configured password using a
// timing-safe comparison. Empty configured password always fails.
func checkPassword(configured, supplied string) bool {
	if configured == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(configured), []byte(supplied)) == 1
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

func (s *Server) upsertUserCtx(ctx context.Context, sub, email, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (oidc_subject, email, display_name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (oidc_subject) DO UPDATE SET email = EXCLUDED.email, display_name = EXCLUDED.display_name
		 RETURNING id`,
		sub, email, name).Scan(&id)
	return id, err
}

func (s *Server) ensureDevUser(ctx context.Context) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, oidc_subject, email, display_name)
		 OVERRIDING SYSTEM VALUE
		 VALUES (1, 'dev-user', 'dev@example.com', 'Dev User')
		 ON CONFLICT DO NOTHING`)
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
