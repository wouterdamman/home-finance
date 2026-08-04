//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestCategoryAutofillActualItemizedMutuallyExclusive guards against the
// #23 bug: autofill_actual writes budget_lines.amount_cents, but every
// read path (EffectiveExpenseCentsSQL, handleGetPeriodOverview) ignores
// that column once tracks_transactions is true (i.e. is_itemized), so a
// category with both flags set silently shows 0 instead of its default.
func TestCategoryAutofillActualItemizedMutuallyExclusive(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const name = "ZTest Autofill Conflict 2099"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, name)
	})

	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": name, "defaultAmountCents": 5000, "isItemized": true, "includeInTemplate": true, "sortOrder": 0, "autofillActual": true,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create with both flags: want 400, got %d", resp.StatusCode)
	}

	resp = c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": name, "defaultAmountCents": 5000, "isItemized": false, "includeInTemplate": true, "sortOrder": 0, "autofillActual": false,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create without conflict: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)
	id := strconv.FormatInt(cat.ID, 10)

	resp = c.do(http.MethodPut, "/api/categories/"+id, map[string]any{
		"name": name, "defaultAmountCents": 5000, "isItemized": true, "includeInTemplate": true, "autofillActual": true,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("update with both flags: want 400, got %d", resp.StatusCode)
	}
}
