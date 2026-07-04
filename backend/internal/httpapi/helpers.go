package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheIronRock95/home-finance/internal/auth"
)

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
	userID, _ := auth.UserIDFromCtx(ctx)
	var email string
	s.pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, userID).Scan(&email)
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
