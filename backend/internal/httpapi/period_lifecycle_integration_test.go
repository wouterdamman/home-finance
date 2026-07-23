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
	resp := c.do(http.MethodDelete, "/api/periods/"+strconv.FormatInt(id, 10), nil)
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
		c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/unlock", nil).Body.Close()
		c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil).Body.Close()
		deleteTestPeriod(t, c, id)
	}()

	resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/lock", nil)
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
	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/lock", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("double lock: want 409, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Unlock, then reopen should succeed again.
	resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/unlock", nil)
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

// TestClosePeriodGuards covers the lifecycle guards handleClosePeriod is
// missing relative to handleReopenPeriod: closing a period whose year is
// locked, and closing a period whose successor is already closed (which
// would otherwise insert a carryover into a next period whose frozen totals
// never included it).
func TestClosePeriodGuards(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	t.Run("blocked when year is locked", func(t *testing.T) {
		const year, month = 2096, 4
		id := createTestPeriod(t, c, year, month)
		defer func() {
			c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/unlock", nil).Body.Close()
			c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/reopen", nil).Body.Close()
			deleteTestPeriod(t, c, id)
		}()

		// Lock requires all periods in the year to be closed first.
		resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close period: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		resp = c.do(http.MethodPost, "/api/years/"+strconv.FormatInt(year, 10)+"/lock", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("lock year: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		// Simulate a leftover open period in an otherwise-locked year (the
		// state the guard defends against — e.g. a lock/close race, or data
		// predating the lock feature) by flipping status directly in the DB,
		// bypassing the app-level guards that would normally prevent it.
		if _, err := pool.Exec(context.Background(), `UPDATE periods SET status='open' WHERE id=$1`, id); err != nil {
			t.Fatalf("simulate reopen: %v", err)
		}

		resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("close while year locked: want 409, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("blocked when next period is already closed", func(t *testing.T) {
		const year, month = 2096, 8
		id := createTestPeriod(t, c, year, month)
		nextID := createTestPeriod(t, c, year, month+1)
		defer func() {
			c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(nextID, 10)+"/reopen", nil).Body.Close()
			deleteTestPeriod(t, c, nextID)
			deleteTestPeriod(t, c, id)
		}()

		resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(nextID, 10)+"/close", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close next period: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(id, 10)+"/close", nil)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("close with next period closed: want 409, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}

func TestCreatePeriodCopiesItemizedIncomeTransactions(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	// Create an itemized income source with includeInTemplate: true.
	resp := c.do(http.MethodPost, "/api/income-sources", map[string]any{
		"name":              "Test Itemized Source XYZ",
		"defaultAmountCents": 0,
		"isItemized":        true,
		"includeInTemplate": true,
		"sortOrder":         999,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income source: want 201, got %d", resp.StatusCode)
	}
	var sourceResp struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &sourceResp)
	sourceID := sourceResp.ID
	t.Cleanup(func() {
		c.do(http.MethodDelete, "/api/income-sources/"+strconv.FormatInt(sourceID, 10), nil).Body.Close()
	})

	// Create a second income source with includeInTemplate: false for negative testing.
	resp = c.do(http.MethodPost, "/api/income-sources", map[string]any{
		"name":              "Non-Template Itemized Source",
		"defaultAmountCents": 0,
		"isItemized":        true,
		"includeInTemplate": false,
		"sortOrder":         998,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create non-template source: want 201, got %d", resp.StatusCode)
	}
	var sourceResp2 struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &sourceResp2)
	nonTemplateSourceID := sourceResp2.ID
	t.Cleanup(func() {
		c.do(http.MethodDelete, "/api/income-sources/"+strconv.FormatInt(nonTemplateSourceID, 10), nil).Body.Close()
	})

	// Create period A (year 2097, month 1) with copyFromPeriodId=0 (empty start).
	periodAID := createTestPeriod(t, c, 2097, 1)
	t.Cleanup(func() {
		deleteTestPeriod(t, c, periodAID)
	})

	// Add two income transactions to period A.
	tx1Date := "2097-01-01"
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodAID, 10)+"/income-transactions", map[string]any{
		"sourceId":    sourceID,
		"amountCents": 10000,
		"description": "Line One",
		"txDate":      tx1Date,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income transaction 1: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	tx2Date := "2097-01-01"
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodAID, 10)+"/income-transactions", map[string]any{
		"sourceId":    sourceID,
		"amountCents": 20000,
		"description": "Line Two",
		"txDate":      tx2Date,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income transaction 2: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Also add a transaction for the non-template source to period A.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodAID, 10)+"/income-transactions", map[string]any{
		"sourceId":    nonTemplateSourceID,
		"amountCents": 5000,
		"description": "Should Not Copy",
		"txDate":      tx1Date,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income transaction (non-template): want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create period B (year 2097, month 2) copying from period A.
	resp = c.do(http.MethodPost, "/api/periods", map[string]any{
		"year":             2097,
		"month":            2,
		"copyFromPeriodId": periodAID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create period B: want 201, got %d", resp.StatusCode)
	}
	var periodBResp struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &periodBResp)
	periodBID := periodBResp.ID
	t.Cleanup(func() {
		deleteTestPeriod(t, c, periodBID)
	})

	// Fetch income transactions for period B, filtered by the template source.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodBID, 10)+"/income-transactions?sourceId="+strconv.FormatInt(sourceID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list income transactions: want 200, got %d", resp.StatusCode)
	}
	type txRow struct {
		ID          int64   `json:"id"`
		PeriodID    int64   `json:"periodId"`
		SourceID    int64   `json:"sourceId"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate"`
	}
	var txRows []txRow
	c.decode(resp, &txRows)

	// Verify: exactly 2 transactions from the template source in period B.
	if len(txRows) != 2 {
		t.Errorf("transaction count: want 2, got %d", len(txRows))
	}

	// Build a map of descriptions to amounts to verify both transactions copied.
	txMap := make(map[string]int64)
	for _, tx := range txRows {
		if tx.PeriodID != periodBID {
			t.Errorf("periodId: want %d, got %d", periodBID, tx.PeriodID)
		}
		if tx.SourceID != sourceID {
			t.Errorf("sourceId: want %d, got %d", sourceID, tx.SourceID)
		}
		// Verify txDate was updated to period B's month (2097-02-01), not kept as 2097-01-01.
		expectedDate := "2097-02-01"
		if tx.TxDate == nil || *tx.TxDate != expectedDate {
			t.Errorf("txDate: want %s, got %v", expectedDate, tx.TxDate)
		}
		txMap[tx.Description] = tx.AmountCents
	}

	if amt, ok := txMap["Line One"]; !ok || amt != 10000 {
		t.Errorf("Line One: want 10000, got %v", amt)
	}
	if amt, ok := txMap["Line Two"]; !ok || amt != 20000 {
		t.Errorf("Line Two: want 20000, got %v", amt)
	}

	// Negative test: verify no transactions from non-template source appear in period B.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodBID, 10)+"/income-transactions?sourceId="+strconv.FormatInt(nonTemplateSourceID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list non-template transactions: want 200, got %d", resp.StatusCode)
	}
	var nonTemplateTxRows []txRow
	c.decode(resp, &nonTemplateTxRows)
	if len(nonTemplateTxRows) != 0 {
		t.Errorf("non-template transaction count: want 0, got %d", len(nonTemplateTxRows))
	}
}
