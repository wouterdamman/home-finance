//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestUpdateHandlersWriteAuditLog guards the "any destructive or
// state-changing action" audit rule (AGENTS.md) against regressing back to
// asymmetric coverage: deletes and a couple of ledger-entry updates logged,
// but handleUpdateCategory/handleUpdatePot/handleUpdateIncomeEntry/
// handleUpdateBudgetLine/handleUpdateTransaction/
// handleUpdateIncomeTransaction/handleUpdateIncomeSource did not. Exercises
// two of those entity types end-to-end over HTTP and asserts the write
// actually landed in audit_log, rather than trusting the update's own 204.
func TestUpdateHandlersWriteAuditLog(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const catName = "ZTest Audit Category 2050"
	const potName = "ZTest Audit Pot 2050"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM audit_log WHERE entity_type='category' AND details::text LIKE '%ZTest Audit Category 2050%'`)
		pool.Exec(ctx, `DELETE FROM audit_log WHERE entity_type='pot' AND details::text LIKE '%ZTest Audit Pot 2050%'`)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, catName)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
	})

	// Category update.
	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": catName, "defaultAmountCents": 1000, "isItemized": false, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)

	resp = c.do(http.MethodPut, "/api/categories/"+strconv.FormatInt(cat.ID, 10), map[string]any{
		"name": catName, "defaultAmountCents": 2000, "isItemized": false, "includeInTemplate": true,
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update category: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	if !auditLogHasEntry(t, c, "category", cat.ID, "category.update") {
		t.Errorf("audit log: expected a category.update entry for category %d", cat.ID)
	}

	// Pot update.
	resp = c.do(http.MethodPost, "/api/pots", map[string]any{
		"name": potName, "kind": "normal", "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create pot: want 201, got %d", resp.StatusCode)
	}
	var pot struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &pot)

	resp = c.do(http.MethodPut, "/api/pots/"+strconv.FormatInt(pot.ID, 10), map[string]any{
		"name": potName, "kind": "normal", "sortOrder": 1,
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update pot: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	if !auditLogHasEntry(t, c, "pot", pot.ID, "pot.update") {
		t.Errorf("audit log: expected a pot.update entry for pot %d", pot.ID)
	}
}

// auditLogHasEntry polls GET /api/audit-log (filtered by action, which the
// endpoint supports server-side) for a row matching entityType/entityID.
// auditLog's own write runs in a context.WithoutCancel'd goroutine-free but
// detached-timeout call, so it is synchronous by the time the update
// handler's response is written — no polling/retry needed, a single fetch
// suffices.
func auditLogHasEntry(t *testing.T, c *apiClient, entityType string, entityID int64, action string) bool {
	t.Helper()
	resp := c.do(http.MethodGet, "/api/audit-log?action="+action+"&entityType="+entityType+"&limit=50", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list audit log: want 200, got %d", resp.StatusCode)
	}
	var rows []struct {
		EntityType *string `json:"entityType"`
		EntityID   *int64  `json:"entityId"`
		Action     string  `json:"action"`
	}
	c.decode(resp, &rows)
	for _, row := range rows {
		if row.Action == action && row.EntityType != nil && *row.EntityType == entityType && row.EntityID != nil && *row.EntityID == entityID {
			return true
		}
	}
	return false
}
