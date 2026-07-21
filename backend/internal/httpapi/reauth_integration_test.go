//go:build integration

package httpapi_test

import (
	"net/http"
	"testing"
)

// TestReauthCompletesWithoutSessionCommitError guards against a regression
// where scs's default GobCodec fails to encode a time.Time value stored in
// the session's map[string]interface{} unless gob.Register(time.Time{})
// has been called first (see internal/auth/session.go's init()). Every
// reauth completion puts a time.Time under "reauthAt", so without that
// registration every /auth/reauth request that gets far enough to write a
// response corrupted the session commit: scs wrote a 500 "Internal Server
// Error" body, then unconditionally appended the handler's own success
// HTML to the same response on top of it. Unit tests didn't catch this
// because they never round-trip a session through the real Postgres store
// (see newTestSessionCtx in reauth_test.go) — this test exercises the real
// HTTP handler against the real pgxStore, like production does.
func TestReauthCompletesWithoutSessionCommitError(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	resp := c.do(http.MethodGet, "/auth/reauth", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth/reauth: got status %d, want 200 (a 500 here means the session commit failed to gob-encode reauthAt)", resp.StatusCode)
	}
}
