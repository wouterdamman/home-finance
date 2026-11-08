package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/wouterdamman/home-finance/internal/auth"
)

// ── Integration API (bearer token, read-only) ────────────────────

func (s *Server) handleIntegrationInfo(w http.ResponseWriter, r *http.Request) {
	tok, _ := auth.IntegrationTokenFromCtx(r.Context())
	JSON(w, http.StatusOK, map[string]any{
		"apiVersion": 1,
		"tokenName":  tok.Name,
		"scopes":     tok.Scopes,
	})
}

// handleIntegrationKidBalances deliberately exposes less than the session
// endpoint: no ids, no reported balance, no ledger.
func (s *Server) handleIntegrationKidBalances(w http.ResponseWriter, r *http.Request) {
	type kid struct {
		Name        string `json:"name"`
		TotalCents  int64  `json:"totalCents"`
		OursCents   int64  `json:"oursCents"`
		TheirsCents int64  `json:"theirsCents"`
	}
	rows, err := s.queryKidBalances(r.Context())
	if err != nil {
		dbError(w, "integrationKidBalances", err)
		return
	}
	kids := make([]kid, 0, len(rows))
	for _, ro := range rows {
		kids = append(kids, kid{ro.Name, ro.TotalCents, ro.OursCents, ro.TheirsCents})
	}
	JSON(w, http.StatusOK, map[string]any{
		"kids":        kids,
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// ── Token management (admin) ─────────────────────────────────────

type integrationTokenView struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	TokenPrefix    string     `json:"tokenPrefix"`
	Scopes         []string   `json:"scopes"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastUsedAt     *time.Time `json:"lastUsedAt"`
	RevokedAt      *time.Time `json:"revokedAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	CreatedByEmail *string    `json:"createdByEmail"`
}

// The token hash is never selected.
const integrationTokenSelect = `
	SELECT t.id, t.name, t.token_prefix, t.scopes, t.created_at, t.last_used_at, t.revoked_at, t.expires_at, u.email
	FROM integration_tokens t
	LEFT JOIN users u ON u.id = t.created_by`

func (v *integrationTokenView) scan(row pgx.Row) error {
	return row.Scan(&v.ID, &v.Name, &v.TokenPrefix, &v.Scopes, &v.CreatedAt, &v.LastUsedAt, &v.RevokedAt, &v.ExpiresAt, &v.CreatedByEmail)
}

func (s *Server) handleListIntegrationTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), integrationTokenSelect+` ORDER BY t.created_at DESC, t.id DESC`)
	if err != nil {
		dbError(w, "listIntegrationTokens", err)
		return
	}
	defer rows.Close()
	out := make([]integrationTokenView, 0)
	for rows.Next() {
		var v integrationTokenView
		if err := v.scan(rows); err != nil {
			dbError(w, "listIntegrationTokens.scan", err)
			return
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listIntegrationTokens", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateIntegrationToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
		// RFC 3339; omitted or null means the default of one year.
		ExpiresAt *string `json:"expiresAt"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 100 {
		Error(w, http.StatusBadRequest, "bad_request", "name must be 1-100 characters")
		return
	}
	if len(body.Scopes) == 0 {
		Error(w, http.StatusBadRequest, "bad_request", "at least one scope is required")
		return
	}
	scopes := make([]string, 0, len(body.Scopes))
	seen := map[string]bool{}
	for _, sc := range body.Scopes {
		if !auth.ValidIntegrationScope(sc) {
			Error(w, http.StatusBadRequest, "bad_request", "unknown scope")
			return
		}
		if !seen[sc] {
			seen[sc] = true
			scopes = append(scopes, sc)
		}
	}
	now := time.Now()
	expiresAt := auth.IntegrationTokenDefaultExpiry(now)
	if body.ExpiresAt != nil {
		parsed, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "expiresAt must be an RFC 3339 timestamp")
			return
		}
		if !auth.ValidIntegrationTokenExpiry(parsed, now) {
			Error(w, http.StatusBadRequest, "bad_request", "expiresAt must be between 1 hour and 2 years from now")
			return
		}
		expiresAt = parsed
	}
	// Minting a long-lived credential is as sensitive as deleting a period.
	if !s.requireFreshReauth(w, r) {
		return
	}

	raw, hash, prefix, err := auth.NewIntegrationToken()
	if err != nil {
		dbError(w, "createIntegrationToken.generate", err)
		return
	}
	var createdBy *int64
	if uid, ok := auth.UserIDFromCtx(r.Context()); ok && uid != 0 {
		createdBy = &uid
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO integration_tokens (name, token_hash, token_prefix, scopes, created_by, expires_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, hash, prefix, scopes, createdBy, expiresAt).Scan(&id); err != nil {
		dbError(w, "createIntegrationToken", err)
		return
	}
	var v integrationTokenView
	if err := v.scan(s.pool.QueryRow(r.Context(), integrationTokenSelect+` WHERE t.id=$1`, id)); err != nil {
		dbError(w, "createIntegrationToken.reload", err)
		return
	}
	// Never log the raw token.
	s.auditLog(r.Context(), "integration_token.create", "integration_token", id,
		map[string]any{"name": name, "scopes": scopes, "prefix": prefix, "expiresAt": expiresAt.UTC().Format(time.RFC3339)})
	JSON(w, http.StatusCreated, struct {
		integrationTokenView
		Token string `json:"token"`
	}{v, raw})
}

// handleRevokeIntegrationToken is idempotent: revoking an already-revoked
// token answers 204 again (and writes no second audit row). Only an id that
// never existed is a 404.
func (s *Server) handleRevokeIntegrationToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var name, prefix string
	err := s.pool.QueryRow(r.Context(),
		`UPDATE integration_tokens SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL RETURNING name, token_prefix`,
		id).Scan(&name, &prefix)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := s.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM integration_tokens WHERE id=$1)`, id).Scan(&exists); err != nil {
			dbError(w, "revokeIntegrationToken.exists", err)
			return
		}
		if !exists {
			Error(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		dbError(w, "revokeIntegrationToken", err)
		return
	}
	s.auditLog(r.Context(), "integration_token.revoke", "integration_token", id,
		map[string]any{"name": name, "prefix": prefix})
	w.WriteHeader(http.StatusNoContent)
}
