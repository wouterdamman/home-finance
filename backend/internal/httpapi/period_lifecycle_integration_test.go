//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wouterdamman/home-finance/internal/auth"
	"github.com/wouterdamman/home-finance/internal/config"
	"github.com/wouterdamman/home-finance/internal/httpapi"
)

// These tests exercise the period close/reopen and year lock/unlock flows
// end-to-end over HTTP against a real Postgres instance (DevFakeAuth mode,
// no OIDC round-trip needed). They use far-future test years (2097/2098 —
// the periods.year column is CHECK-constrained to 2000-2100) so they never
// collide with real household data, and always clean up the periods they
// create.

func newIntegrationServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	cfg := &config.Config{DevFakeAuth: true, Env: "development", SessionSecure: false}
	sm := auth.NewSessionManager(pool, false)
	handler := httpapi.NewServer(cfg, pool, sm, nil)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, pool
}

type periodOverviewResponse struct {
	Period struct {
		Status   string  `json:"status"`
		ClosedAt *string `json:"closedAt"`
	} `json:"period"`
	IncomeTotalCents  int64 `json:"incomeTotalCents"`
	ExpenseTotalCents int64 `json:"expenseTotalCents"`
	SurplusCents      int64 `json:"surplusCents"`
}

type apiClient struct {
	t   *testing.T
	srv *httptest.Server
	hc  *http.Client
}

func newAPIClient(t *testing.T, srv *httptest.Server) *apiClient {
	return &apiClient{t: t, srv: srv, hc: srv.Client()}
}

func (c *apiClient) do(method, path string, body any) *http.Response {
	c.t.Helper()
	var reader *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = strings.NewReader(string(b))
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, c.srv.URL+path, reader)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (c *apiClient) decode(resp *http.Response, v any) {
	c.t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		c.t.Fatalf("decode response: %v", err)
	}
}

// createTestPeriod creates a period with no copied budget/income template
// (copyFromPeriodId: 0 matches no real period, so nothing is inherited).
func createTestPeriod(t *testing.T, c *apiClient, year, month int) int64 {
	t.Helper()
	resp := c.do(http.MethodPost, "/api/periods", map[string]any{
		"year": year, "month": month, "copyFromPeriodId": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create period: want 201, got %d", resp.StatusCode)
	}
	var out struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &out)
	return out.ID
}

func deleteTestPeriod(t *testing.T, c *apiClient, id int64) {
	t.Helper()
	resp := c.do(http.MethodDelete, "/api/periods/"+strconv.FormatInt(id, 10), map[string]any{"password": "test"})
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("cleanup: delete period %d: want 204, got %d", id, resp.StatusCode)
	}
}

func TestPeriodCloseReopenLifecycle(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	const year, month = 2098, 6
	id := createTestPeriod(t, c, year, month)
	defer func() {
		// Ensure period is open before cleanup delete (closed periods can't be deleted).
		c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil)
		deleteTestPeriod(t, c, id)
	}()

	// Add income and a budget line so close computes a non-trivial surplus.
	resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/incomes", map[string]any{
		"label": "Test salary", "amountCents": 500000, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/budget-lines", map[string]any{
		"label": "Test rent", "amountCents": 120000, "tracksTransactions": false, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Close.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(id, 10)+"/overview", nil)
	var overview periodOverviewResponse
	c.decode(resp, &overview)
	if overview.Period.Status != "closed" {
		t.Errorf("status: want closed, got %s", overview.Period.Status)
	}
	if overview.Period.ClosedAt == nil {
		t.Error("closedAt: want set, got nil")
	}
	if overview.IncomeTotalCents != 500000 {
		t.Errorf("incomeTotalCents: want 500000, got %d", overview.IncomeTotalCents)
	}
	if overview.ExpenseTotalCents != 120000 {
		t.Errorf("expenseTotalCents: want 120000, got %d", overview.ExpenseTotalCents)
	}
	if overview.SurplusCents != 380000 {
		t.Errorf("surplusCents: want 380000, got %d", overview.SurplusCents)
	}

	// Closing again must be rejected, not silently re-applied.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("double close: want 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Reopen.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reopen period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(id, 10)+"/overview", nil)
	overview = periodOverviewResponse{}
	c.decode(resp, &overview)
	if overview.Period.Status != "open" {
		t.Errorf("status after reopen: want open, got %s", overview.Period.Status)
	}
	if overview.Period.ClosedAt != nil {
		t.Errorf("closedAt after reopen: want nil, got %v", *overview.Period.ClosedAt)
	}
}

func TestYearLockBlocksWritesAndUnlockRestores(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	const year, month = 2097, 3
	id := createTestPeriod(t, c, year, month)
	defer func() {
		// Best-effort cleanup: unlock, reopen, delete.
		c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/unlock", map[string]any{"password": "test"}).Body.Close()
		c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil).Body.Close()
		deleteTestPeriod(t, c, id)
	}()

	resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/lock", map[string]any{"password": "test"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("lock year: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/years/"+strconv.FormatInt(year, 10)+"/summary", nil)
	var summary struct {
		Locked bool `json:"locked"`
	}
	c.decode(resp, &summary)
	if !summary.Locked {
		t.Error("year summary: want locked=true after lock")
	}

	// Reopen must be blocked while the year is locked.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("reopen while locked: want 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Locking an already-locked year must be rejected, not silently accepted.
	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/lock", map[string]any{"password": "test"})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("double lock: want 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Unlock, then reopen should succeed again.
	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/unlock", map[string]any{"password": "test"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unlock year: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/years/"+strconv.FormatInt(year, 10)+"/summary", nil)
	c.decode(resp, &summary)
	if summary.Locked {
		t.Error("year summary: want locked=false after unlock")
	}

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("reopen after unlock: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
