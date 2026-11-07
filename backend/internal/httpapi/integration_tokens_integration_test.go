//go:build integration

package httpapi_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type tokenResp struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
	LastUsedAt  *string  `json:"lastUsedAt"`
	RevokedAt   *string  `json:"revokedAt"`
	Token       string   `json:"token"`
}

// bearerGet issues a request carrying only the bearer header (no cookie jar,
// no CSRF header), like Home Assistant would.
func bearerGet(t *testing.T, srvURL, method, path, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srvURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method != http.MethodGet {
		// /api's CSRF gate still applies to non-GET methods, even though
		// bearer auth has no ambient credentials to forge.
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func createTestToken(t *testing.T, c *apiClient, pool *pgxpool.Pool, name string, scopes []string) tokenResp {
	t.Helper()
	resp := c.do(http.MethodPost, "/api/integration-tokens", map[string]any{"name": name, "scopes": scopes})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create token: want 201, got %d", resp.StatusCode)
	}
	var out tokenResp
	c.decode(resp, &out)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM integration_tokens WHERE id=$1`, out.ID)
		pool.Exec(context.Background(), `DELETE FROM audit_log WHERE entity_type='integration_token' AND entity_id=$1`, out.ID)
	})
	return out
}

func TestIntegrationKidBalances(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	// Two kids of our own; other tests/demo data may add more, so assert
	// only on these by name.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	names := []string{"itk-a-" + suffix, "itk-b-" + suffix}
	for i, n := range names {
		var kidID int64
		if err := pool.QueryRow(ctx, `INSERT INTO kids (name, sort_order) VALUES ($1, 1000) RETURNING id`, n).Scan(&kidID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Exec(ctx, `DELETE FROM kid_savings_ledger WHERE kid_id=$1`, kidID)
			pool.Exec(ctx, `DELETE FROM kids WHERE id=$1`, kidID)
		})
		pool.Exec(ctx, `INSERT INTO kid_savings_ledger (kid_id, owner, entry_type, amount_cents) VALUES ($1,'ours','deposit',$2),($1,'theirs','deposit',$3)`,
			kidID, 1000*(i+1), 250*(i+1))
	}

	tok := createTestToken(t, c, pool, "itest kids", []string{"kids:read", "kids:read"})
	if len(tok.Scopes) != 1 {
		t.Errorf("scopes not deduped: %v", tok.Scopes)
	}
	if !strings.HasPrefix(tok.Token, "hf_") || tok.TokenPrefix != tok.Token[:8] {
		t.Errorf("unexpected token/prefix: %q / %q", tok.Token, tok.TokenPrefix)
	}

	resp := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/kids/balances", tok.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("balances: want 200, got %d", resp.StatusCode)
	}
	if len(resp.Header.Values("Set-Cookie")) != 0 {
		t.Errorf("token request set a cookie: %v", resp.Header.Values("Set-Cookie"))
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	var out struct {
		Kids        []map[string]any `json:"kids"`
		GeneratedAt string           `json:"generatedAt"`
	}
	c.decode(resp, &out)
	if _, err := time.Parse(time.RFC3339, out.GeneratedAt); err != nil {
		t.Errorf("generatedAt %q: %v", out.GeneratedAt, err)
	}
	found := 0
	for _, k := range out.Kids {
		if len(k) != 4 {
			t.Errorf("kid has %d fields, want 4: %v", len(k), k)
		}
		for i, n := range names {
			if k["name"] == n {
				found++
				if k["oursCents"] != float64(1000*(i+1)) || k["theirsCents"] != float64(250*(i+1)) || k["totalCents"] != float64(1250*(i+1)) {
					t.Errorf("wrong amounts for %s: %v", n, k)
				}
			}
		}
	}
	if found != 2 {
		t.Errorf("found %d of our 2 kids", found)
	}

	// info
	resp = bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/info", tok.Token)
	var info struct {
		APIVersion int      `json:"apiVersion"`
		TokenName  string   `json:"tokenName"`
		Scopes     []string `json:"scopes"`
	}
	c.decode(resp, &info)
	if info.APIVersion != 1 || info.TokenName != "itest kids" || len(info.Scopes) != 1 {
		t.Errorf("info: %+v", info)
	}

	// last_used_at was set by the requests above.
	var lastUsed *time.Time
	pool.QueryRow(ctx, `SELECT last_used_at FROM integration_tokens WHERE id=$1`, tok.ID).Scan(&lastUsed)
	if lastUsed == nil {
		t.Error("last_used_at not set")
	}
}

func TestIntegrationAuthFailures(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	for name, tok := range map[string]string{"no header": "", "garbage": "hf_notarealtoken"} {
		resp := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/kids/balances", tok)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d", name, resp.StatusCode)
		}
		if resp.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s: missing WWW-Authenticate", name)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(b), `"code":"unauthorized"`) {
			t.Errorf("%s: body %s", name, b)
		}
	}

	// A session (DevFakeAuth) must not substitute for a token.
	resp := c.do(http.MethodGet, "/api/integrations/v1/info", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("session-only request: want 401, got %d", resp.StatusCode)
	}

	// Revoked.
	tok := createTestToken(t, c, pool, "itest revoke", []string{"kids:read"})
	if r := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/info", tok.Token); r.StatusCode != http.StatusOK {
		t.Fatalf("before revoke: want 200, got %d", r.StatusCode)
	}
	idPath := "/api/integration-tokens/" + strconv.FormatInt(tok.ID, 10)
	for i := 0; i < 2; i++ { // second revoke is idempotent
		r := c.do(http.MethodDelete, idPath, nil)
		r.Body.Close()
		if r.StatusCode != http.StatusNoContent {
			t.Errorf("revoke #%d: want 204, got %d", i+1, r.StatusCode)
		}
	}
	if r := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/info", tok.Token); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("after revoke: want 401, got %d", r.StatusCode)
	}
	r := c.do(http.MethodDelete, "/api/integration-tokens/999999999", nil)
	r.Body.Close()
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("revoke unknown id: want 404, got %d", r.StatusCode)
	}

	// Audit rows: exactly one create and one revoke.
	for action, want := range map[string]int{"integration_token.create": 1, "integration_token.revoke": 1} {
		var n int
		var details string
		pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action=$1 AND entity_type='integration_token' AND entity_id=$2`, action, tok.ID).Scan(&n)
		if n != want {
			t.Errorf("%s audit rows = %d, want %d", action, n, want)
		}
		pool.QueryRow(ctx, `SELECT COALESCE(string_agg(details::text, ''), '') FROM audit_log WHERE action=$1 AND entity_id=$2 AND entity_type='integration_token'`, action, tok.ID).Scan(&details)
		if strings.Contains(details, tok.Token) {
			t.Errorf("%s audit details contain the raw token", action)
		}
	}
}

func TestIntegrationScopeAndRouting(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	// The API only mints known scopes, so a scope-less token (or one with a
	// future scope) has to be inserted directly.
	raw := "hf_scopetest" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO integration_tokens (name, token_hash, token_prefix, scopes) VALUES ('itest noscope', sha256(convert_to($1,'UTF8')), 'hf_scope', '{pots:read}') RETURNING id`, raw).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM integration_tokens WHERE id=$1`, id) })

	if r := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/info", raw); r.StatusCode != http.StatusOK {
		t.Errorf("info with any scope: want 200, got %d", r.StatusCode)
	}
	resp := bearerGet(t, srv.URL, http.MethodGet, "/api/integrations/v1/kids/balances", raw)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(b), `"code":"insufficient_scope"`) {
		t.Errorf("want 403 insufficient_scope, got %d %s", resp.StatusCode, b)
	}

	// JSON 405 / 404 under the integration prefix.
	for _, tc := range []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodPost, "/api/integrations/v1/kids/balances", http.StatusMethodNotAllowed, "method_not_allowed"},
		{http.MethodGet, "/api/integrations/v1/nope", http.StatusNotFound, "not_found"},
	} {
		resp := bearerGet(t, srv.URL, tc.method, tc.path, raw)
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status || !strings.Contains(string(b), `"code":"`+tc.code+`"`) {
			t.Errorf("%s %s: want %d %s, got %d %s", tc.method, tc.path, tc.status, tc.code, resp.StatusCode, b)
		}
	}
	_ = c
}

func TestIntegrationTokenAdmin(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	for name, body := range map[string]map[string]any{
		"empty name":    {"name": "  ", "scopes": []string{"kids:read"}},
		"long name":     {"name": strings.Repeat("x", 101), "scopes": []string{"kids:read"}},
		"no scopes":     {"name": "x", "scopes": []string{}},
		"unknown scope": {"name": "x", "scopes": []string{"kids:write"}},
	} {
		resp := c.do(http.MethodPost, "/api/integration-tokens", body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}

	tok := createTestToken(t, c, pool, fmt.Sprintf("  itest list %d  ", time.Now().UnixNano()), []string{"kids:read"})
	if strings.HasPrefix(tok.Name, " ") {
		t.Errorf("name not trimmed: %q", tok.Name)
	}

	resp := c.do(http.MethodGet, "/api/integration-tokens", nil)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(raw)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	if strings.Contains(body, tok.Token) || strings.Contains(body, `"token"`) || strings.Contains(strings.ToLower(body), "hash") {
		t.Error("list leaks token or hash")
	}
	if !strings.Contains(body, `"tokenPrefix":"`+tok.TokenPrefix+`"`) {
		t.Error("list missing our token")
	}
}
