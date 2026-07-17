package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/wouterdamman/home-finance/internal/auth"
)

func sanitizeReturnTo(returnTo string) string {
	if returnTo == "" {
		return "/"
	}
	u, err := url.Parse(returnTo)
	if err != nil || u.IsAbs() || !strings.HasPrefix(returnTo, "/") {
		return "/"
	}
	return returnTo
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := sanitizeReturnTo(r.URL.Query().Get("return_to"))
	if s.cfg.DevFakeAuth {
		s.ensureDevUser(r.Context())
		if err := s.sm.RenewToken(r.Context()); err != nil {
			slog.Error("session renew", "err", err)
		}
		s.sm.Put(r.Context(), "userID", int64(1))
		s.sm.Put(r.Context(), "userEmail", "dev@example.com")
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
		Nonce string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "claims error", http.StatusInternalServerError)
		return
	}

	storedNonce, _ := s.sm.Get(r.Context(), "oidc_nonce").(string)
	if storedNonce == "" || claims.Nonce != storedNonce {
		http.Error(w, "invalid nonce", http.StatusBadRequest)
		return
	}

	if len(s.cfg.AllowedEmails) > 0 {
		allowed := false
		for _, e := range s.cfg.AllowedEmails {
			if strings.EqualFold(e, claims.Email) {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
	}

	userID, err := s.upsertUserCtx(r.Context(), claims.Sub, claims.Email, claims.Name)
	if err != nil {
		slog.Error("upsert user", "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if err := s.sm.RenewToken(r.Context()); err != nil {
		slog.Error("session renew", "err", err)
	}
	s.sm.Put(r.Context(), "userID", userID)
	s.sm.Put(r.Context(), "userEmail", claims.Email)
	s.sm.Remove(r.Context(), "oidc_state")
	s.sm.Remove(r.Context(), "oidc_nonce")

	returnTo := sanitizeReturnTo(s.sm.GetString(r.Context(), "return_to"))
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sm.Destroy(r.Context()); err != nil {
		slog.Error("session destroy", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
