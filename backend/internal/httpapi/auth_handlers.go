package httpapi

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

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

// handleAuthReauth starts the "step-up" re-authentication popup used to
// confirm identity before a destructive action (delete period, lock/unlock
// year, import wipe/reset). It reuses the existing /auth/callback redirect
// URI — Authentik only needs one registered redirect URI — and is told
// apart from a normal login purely by which session key the returned
// `state` matches (see handleAuthCallback).
func (s *Server) handleAuthReauth(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevFakeAuth {
		// No real Authentik account in dev — mirror handleAuthLogin's bypass.
		s.sm.Put(r.Context(), "reauthAt", time.Now())
		writeReauthPage(w, true, "")
		return
	}
	if s.oidc == nil {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}
	state, _ := auth.RandomString(16)
	nonce, _ := auth.RandomString(16)
	s.sm.Put(r.Context(), "reauth_state", state)
	s.sm.Put(r.Context(), "reauth_nonce", nonce)
	// prompt=login forces Authentik to re-show its login form even if it
	// still has an SSO session for this browser — that's the actual proof
	// of "you still know the password," never seen by our backend.
	url := s.oidc.AuthCodeURL(state, nonce, oauth2.SetAuthURLParam("prompt", "login"))
	http.Redirect(w, r, url, http.StatusFound)
}

// finishReauthCallback verifies that whoever just completed the Authentik
// login popup is the SAME account already logged into this session — not
// just "some successful Authentik login" — before granting a fresh
// reauthAt. Without this check, re-authenticating as a different Authentik
// account in the popup would let that account approve destructive actions
// on this session.
func (s *Server) finishReauthCallback(w http.ResponseWriter, r *http.Request) {
	claims, stage, err := s.exchangeAndVerifyClaims(r, "reauth_nonce")
	if stage != "" {
		slog.Error("reauth exchange failed", "stage", stage, "err", err)
		writeReauthPage(w, false, "Could not verify your identity. Please try again.")
		return
	}

	userID, _ := s.sm.Get(r.Context(), "userID").(int64)
	var storedSub, storedEmail string
	if err := s.pool.QueryRow(r.Context(), `SELECT oidc_subject, email FROM users WHERE id=$1`, userID).Scan(&storedSub, &storedEmail); err != nil {
		slog.Error("reauth user lookup", "err", err)
		writeReauthPage(w, false, "Could not verify your identity. Please try again.")
		return
	}

	if err := verifyReauthIdentity(storedSub, storedEmail, claims.Sub, claims.Email); err != nil {
		slog.Warn("reauth identity mismatch", "sessionSub", storedSub, "sessionEmail", storedEmail, "claimsSub", claims.Sub, "claimsEmail", claims.Email)
		writeReauthPage(w, false, "That wasn't the currently logged-in account. Please close this window and try again.")
		return
	}

	s.sm.Put(r.Context(), "reauthAt", time.Now())
	s.sm.Remove(r.Context(), "reauth_state")
	s.sm.Remove(r.Context(), "reauth_nonce")
	writeReauthPage(w, true, "")
}

// verifyReauthIdentity is a pure function so the identity-match logic is
// unit-testable without a real OIDC round trip. sub is authoritative (the
// stable OIDC identifier); email is checked too, defense-in-depth, but a
// sub match alone is sufficient (email can change in Authentik's profile
// between logins).
func verifyReauthIdentity(storedSub, storedEmail, claimsSub, claimsEmail string) error {
	if storedSub != "" && claimsSub == storedSub {
		return nil
	}
	if storedEmail != "" && strings.EqualFold(claimsEmail, storedEmail) {
		return nil
	}
	return fmt.Errorf("reauth identity mismatch: session sub=%q email=%q, callback sub=%q email=%q", storedSub, storedEmail, claimsSub, claimsEmail)
}

func writeReauthPage(w http.ResponseWriter, ok bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if ok {
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding-top:40px">
<p>Identity confirmed. You can close this window.</p>
<script>window.close()</script>
</body></html>`)
		return
	}
	fmt.Fprintf(w, `<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding-top:40px">
<p>%s</p>
<button onclick="window.close()">Close</button>
</body></html>`, html.EscapeString(message))
}

// handleReauthStatus lets the frontend poll while the reauth popup is open,
// so it can auto-close the popup and proceed the moment reauthAt is fresh.
func (s *Server) handleReauthStatus(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]bool{"fresh": s.reauthIsFresh(r)})
}

// oidcClaims is the subset of ID token claims both the login and step-up
// reauth flows care about.
type oidcClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Nonce string `json:"nonce"`
}

// exchangeAndVerifyClaims does the code exchange + ID token verification +
// nonce check shared by both the normal login callback and the step-up
// reauth callback. nonceKey names which session key holds the nonce to
// check against (distinct keys for login vs. reauth so the two flows can
// never cross-contaminate each other's in-flight state). stage identifies
// which part failed, so callers can map it to the same HTTP status codes
// they used before this was factored out.
func (s *Server) exchangeAndVerifyClaims(r *http.Request, nonceKey string) (claims oidcClaims, stage string, err error) {
	idToken, err := s.oidc.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		return claims, "exchange", err
	}
	if err := idToken.Claims(&claims); err != nil {
		return claims, "claims", err
	}
	storedNonce, _ := s.sm.Get(r.Context(), nonceKey).(string)
	if storedNonce == "" || claims.Nonce != storedNonce {
		return claims, "nonce", nil
	}
	return claims, "", nil
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DevFakeAuth {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	reauthState, _ := s.sm.Get(r.Context(), "reauth_state").(string)
	if reauthState != "" && r.URL.Query().Get("state") == reauthState {
		s.finishReauthCallback(w, r)
		return
	}

	storedState, _ := s.sm.Get(r.Context(), "oidc_state").(string)
	if r.URL.Query().Get("state") != storedState || storedState == "" {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	claims, stage, err := s.exchangeAndVerifyClaims(r, "oidc_nonce")
	switch stage {
	case "exchange":
		slog.Error("oidc exchange", "err", err)
		http.Error(w, "auth failed", http.StatusUnauthorized)
		return
	case "claims":
		http.Error(w, "claims error", http.StatusInternalServerError)
		return
	case "nonce":
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
