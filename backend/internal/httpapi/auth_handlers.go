package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/TheIronRock95/home-finance/internal/auth"
)

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("return_to")
	if returnTo == "" {
		returnTo = "/"
	}
	if s.cfg.DevFakeAuth {
		s.ensureDevUser(r.Context())
		s.sm.Put(r.Context(), "userID", int64(1))
		http.Redirect(w, r, returnTo, http.StatusFound)
		return
	}
	if s.oidc == nil {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}
	state, _ := auth.RandomString(16)
	nonce, _ := auth.RandomString(16)
	s.sm.Put(r.Context(), "oidc_state", state)
	s.sm.Put(r.Context(), "oidc_nonce", nonce)
	s.sm.Put(r.Context(), "return_to", returnTo)
	http.Redirect(w, r, s.oidc.AuthCodeURL(state, nonce), http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevFakeAuth {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	storedState, _ := s.sm.Get(r.Context(), "oidc_state").(string)
	if r.URL.Query().Get("state") != storedState || storedState == "" {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	idToken, err := s.oidc.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("oidc exchange", "err", err)
		http.Error(w, "auth failed", http.StatusUnauthorized)
		return
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "claims error", http.StatusInternalServerError)
		return
	}
	userID, err := s.upsertUserCtx(r.Context(), claims.Sub, claims.Email, claims.Name)
	if err != nil {
		slog.Error("upsert user", "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.sm.Put(r.Context(), "userID", userID)
	s.sm.Remove(r.Context(), "oidc_state")
	s.sm.Remove(r.Context(), "oidc_nonce")
	returnTo, _ := s.sm.Get(r.Context(), "return_to").(string)
	if returnTo == "" {
		returnTo = "/"
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sm.Destroy(r.Context()); err != nil {
		slog.Error("session destroy", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var id int64
	var email, displayName string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT id, email, display_name FROM users WHERE id = $1`, uid).
		Scan(&id, &email, &displayName); err != nil {
		Error(w, http.StatusUnauthorized, "unauthorized", "user not found")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"id": id, "email": email, "displayName": displayName})
}
