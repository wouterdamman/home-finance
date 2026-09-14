package httpapi

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/wouterdamman/home-finance/internal/auth"
)

// oidcFlowMaxAge bounds how long an in-flight login/reauth may take. Past
// it the callback is refused and the stored state/nonce/verifier are
// dropped, so an abandoned flow can't stay armed for the session's whole
// lifetime.
const oidcFlowMaxAge = 10 * time.Minute

// oidcFlowKeys are every in-flight artifact of both the login and the
// step-up reauth flow.
var oidcFlowKeys = []string{
	"oidc_state", "oidc_nonce", "oidc_verifier", "oidc_started_at",
	"reauth_state", "reauth_nonce", "reauth_verifier", "reauth_started_at",
}

// sanitizeReturnTo reduces a caller-supplied return_to to a same-origin
// path. A protocol-relative URL ("//evil.com", plus the "/\evil.com" form
// Chrome and Edge normalize identically) has no scheme, so u.IsAbs() is
// false, and it starts with "/" — a naive check lets it through and the
// browser resolves it off-site. The result is rebuilt from the parsed
// pieces rather than echoed back verbatim.
func sanitizeReturnTo(returnTo string) string {
	if !strings.HasPrefix(returnTo, "/") {
		return "/"
	}
	if len(returnTo) > 1 && (returnTo[1] == '/' || returnTo[1] == '\\') {
		return "/"
	}
	u, err := url.Parse(returnTo)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" {
		return "/"
	}
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/") {
		return "/"
	}
	if u.RawQuery != "" {
		return path + "?" + u.RawQuery
	}
	return path
}

// clearOIDCFlowState drops every in-flight login/reauth artifact. Both
// flows are cleared together on every terminal branch: the two pairs share
// one callback URL, and whichever one the user abandoned is just as stale
// as the one that failed.
func (s *Server) clearOIDCFlowState(ctx context.Context) {
	for _, k := range oidcFlowKeys {
		s.sm.Remove(ctx, k)
	}
}

// flowStartedRecently reports whether the flow whose start timestamp lives
// under key began within oidcFlowMaxAge.
func (s *Server) flowStartedRecently(ctx context.Context, key string) bool {
	started, ok := s.sm.Get(ctx, key).(time.Time)
	return ok && time.Since(started) <= oidcFlowMaxAge
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := sanitizeReturnTo(r.URL.Query().Get("return_to"))
	if s.cfg.DevFakeAuth {
		s.ensureDevUser(r.Context())
		if err := s.sm.RenewToken(r.Context()); err != nil {
			// Writing userID into a session token that wasn't rotated
			// would hand an attacker-fixated token a logged-in session.
			slog.Error("session renew", "err", err)
			http.Error(w, "session error", http.StatusInternalServerError)
			return
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
	verifier := oauth2.GenerateVerifier()
	s.sm.Put(r.Context(), "oidc_state", state)
	s.sm.Put(r.Context(), "oidc_nonce", nonce)
	s.sm.Put(r.Context(), "oidc_verifier", verifier)
	s.sm.Put(r.Context(), "oidc_started_at", time.Now())
	s.sm.Put(r.Context(), "return_to", returnTo)
	http.Redirect(w, r, s.oidc.AuthCodeURL(state, nonce, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// sameSiteNavigation reports whether this request is a same-origin
// navigation. Sec-Fetch-Site is set by the browser itself and cannot be
// forged from page JS; an absent header means a non-browser client (curl,
// tests), which is not the threat being defended against here.
func sameSiteNavigation(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin":
		return true
	}
	return false
}

// handleAuthReauth starts the "step-up" re-authentication popup used to
// confirm identity before a destructive action (delete period, lock/unlock
// year, import wipe/reset). It reuses the existing /auth/callback redirect
// URI — Authentik only needs one registered redirect URI — and is told
// apart from a normal login purely by which session key the returned
// `state` matches (see handleAuthCallback).
func (s *Server) handleAuthReauth(w http.ResponseWriter, r *http.Request) {
	// SameSite=Lax still sends the session cookie on a cross-site top-level
	// GET navigation, so without this an attacker page could pop a genuine
	// Authentik password prompt the user never asked for and overwrite an
	// in-flight reauth_state.
	if !sameSiteNavigation(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
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
	verifier := oauth2.GenerateVerifier()
	s.sm.Put(r.Context(), "reauth_state", state)
	s.sm.Put(r.Context(), "reauth_nonce", nonce)
	s.sm.Put(r.Context(), "reauth_verifier", verifier)
	s.sm.Put(r.Context(), "reauth_started_at", time.Now())
	// prompt=login forces Authentik to re-show its login form even if it
	// still has an SSO session for this browser — that's the actual proof
	// of "you still know the password," never seen by our backend.
	authURL := s.oidc.AuthCodeURL(state, nonce,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "login"))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// finishReauthCallback verifies that whoever just completed the Authentik
// login popup is the SAME account already logged into this session — not
// just "some successful Authentik login" — before granting a fresh
// reauthAt. Without this check, re-authenticating as a different Authentik
// account in the popup would let that account approve destructive actions
// on this session.
func (s *Server) finishReauthCallback(w http.ResponseWriter, r *http.Request) {
	if !s.flowStartedRecently(r.Context(), "reauth_started_at") {
		s.clearOIDCFlowState(r.Context())
		writeReauthPage(w, false, "This confirmation took too long. Please try again.")
		return
	}

	claims, stage, err := s.exchangeAndVerifyClaims(r, "reauth_nonce", "reauth_verifier")
	if stage != "" {
		s.clearOIDCFlowState(r.Context())
		slog.Error("reauth exchange failed", "stage", stage, "err", err)
		writeReauthPage(w, false, "Could not verify your identity. Please try again.")
		return
	}
	if !emailVerifiedOK(claims) {
		s.clearOIDCFlowState(r.Context())
		slog.Warn("reauth rejected: email_verified is false")
		writeReauthPage(w, false, "Could not verify your identity. Please try again.")
		return
	}

	userID, _ := s.sm.Get(r.Context(), "userID").(int64)
	var storedSub, storedEmail string
	if err := s.pool.QueryRow(r.Context(), `SELECT oidc_subject, email FROM users WHERE id=$1`, userID).Scan(&storedSub, &storedEmail); err != nil {
		s.clearOIDCFlowState(r.Context())
		slog.Error("reauth user lookup", "err", err)
		writeReauthPage(w, false, "Could not verify your identity. Please try again.")
		return
	}

	if err := verifyReauthIdentity(storedSub, storedEmail, claims.Sub, claims.Email); err != nil {
		s.clearOIDCFlowState(r.Context())
		// Only the local user id and match booleans — logs go to stdout and
		// are retained by the cluster log stack, where the raw OIDC subject
		// and email would be the most sensitive data the app holds.
		slog.Warn("reauth identity mismatch", "userID", userID,
			"subMatch", storedSub != "" && claims.Sub == storedSub,
			"emailMatch", storedEmail != "" && strings.EqualFold(claims.Email, storedEmail))
		writeReauthPage(w, false, "That wasn't the currently logged-in account. Please close this window and try again.")
		return
	}

	s.sm.Put(r.Context(), "reauthAt", time.Now())
	s.clearOIDCFlowState(r.Context())
	writeReauthPage(w, true, "")
}

// errReauthIdentityMismatch deliberately carries no identity values — the
// caller logs it, and the subjects/emails involved must not reach stdout.
var errReauthIdentityMismatch = errors.New("reauth identity mismatch")

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
	return errReauthIdentityMismatch
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
// reauth flows care about. EmailVerified is a pointer because the claim is
// optional in OIDC — absent and false must be told apart (see
// emailVerifiedOK).
type oidcClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	EmailVerified *bool  `json:"email_verified"`
}

var warnEmailVerifiedMissing sync.Once

// emailVerifiedOK reports whether the ID token's email claim may be
// trusted. Email drives both the ALLOWED_EMAILS sign-in gate and the
// INITIAL_ADMIN_EMAILS bootstrap, so an unverified address is an account
// takeover primitive at any IdP that lets a user set one freely. An IdP
// that omits the claim entirely is allowed through (warned about once)
// rather than locking every account out.
func emailVerifiedOK(claims oidcClaims) bool {
	if claims.EmailVerified == nil {
		warnEmailVerifiedMissing.Do(func() {
			slog.Warn("id token has no email_verified claim; email-based access control cannot be confirmed against the IdP")
		})
		return true
	}
	return *claims.EmailVerified
}

// exchangeAndVerifyClaims does the code exchange + ID token verification +
// nonce check shared by both the normal login callback and the step-up
// reauth callback. nonceKey/verifierKey name which session keys hold the
// nonce and the PKCE verifier to use (distinct keys for login vs. reauth so
// the two flows can never cross-contaminate each other's in-flight state).
// stage identifies which part failed, so callers can map it to the same
// HTTP status codes they used before this was factored out.
func (s *Server) exchangeAndVerifyClaims(r *http.Request, nonceKey, verifierKey string) (claims oidcClaims, stage string, err error) {
	var opts []oauth2.AuthCodeOption
	// A session that started its flow before PKCE shipped has no stored
	// verifier; sending an empty code_verifier would fail the exchange
	// outright, so those in-flight logins fall back to state+nonce only.
	if verifier, _ := s.sm.Get(r.Context(), verifierKey).(string); verifier != "" {
		opts = append(opts, oauth2.VerifierOption(verifier))
	}
	idToken, err := s.oidc.Exchange(r.Context(), r.URL.Query().Get("code"), opts...)
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
		s.clearOIDCFlowState(r.Context())
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	if !s.flowStartedRecently(r.Context(), "oidc_started_at") {
		s.clearOIDCFlowState(r.Context())
		http.Error(w, "login took too long, please try again", http.StatusBadRequest)
		return
	}
	claims, stage, err := s.exchangeAndVerifyClaims(r, "oidc_nonce", "oidc_verifier")
	if stage != "" {
		s.clearOIDCFlowState(r.Context())
	}
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

	if !emailVerifiedOK(claims) {
		s.clearOIDCFlowState(r.Context())
		slog.Warn("login rejected: email_verified is false")
		http.Error(w, "access denied", http.StatusForbidden)
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
			s.clearOIDCFlowState(r.Context())
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
	}

	userID, err := s.upsertUserCtx(r.Context(), claims.Sub, claims.Email, claims.Name)
	if err != nil {
		s.clearOIDCFlowState(r.Context())
		slog.Error("upsert user", "err", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if err := s.sm.RenewToken(r.Context()); err != nil {
		// Fail closed: putting userID into the un-rotated token would
		// defeat the session-fixation defense RenewToken exists for.
		s.clearOIDCFlowState(r.Context())
		slog.Error("session renew", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	s.sm.Put(r.Context(), "userID", userID)
	s.sm.Put(r.Context(), "userEmail", claims.Email)
	s.clearOIDCFlowState(r.Context())

	returnTo := sanitizeReturnTo(s.sm.GetString(r.Context(), "return_to"))
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sm.Destroy(r.Context()); err != nil {
		slog.Error("session destroy", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
