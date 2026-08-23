//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestCategoryAutofillActualItemizedAllowed guards the 2026-08-23 behavior
// change: autofillActual and isItemized are no longer mutually exclusive.
// For an itemized category, autofillActual now means "copy the prior
// period's transactions forward at template-copy time" (see
// copyPeriodTemplate) instead of writing a never-read default amount, so
// both flags may be set together.
func TestCategoryAutofillActualItemizedAllowed(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const name = "ZTest Autofill Itemized 2099"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, name)
	})

	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": name, "defaultAmountCents": 5000, "isItemized": true, "includeInTemplate": true, "sortOrder": 0, "autofillActual": true,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create with both flags: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID             int64 `json:"id"`
		IsItemized     bool  `json:"isItemized"`
		AutofillActual bool  `json:"autofillActual"`
	}
	c.decode(resp, &cat)
	if !cat.IsItemized || !cat.AutofillActual {
		t.Fatalf("create response: want both flags true, got isItemized=%v autofillActual=%v", cat.IsItemized, cat.AutofillActual)
	}
	id := strconv.FormatInt(cat.ID, 10)

	resp = c.do(http.MethodPut, "/api/categories/"+id, map[string]any{
		"name": name, "defaultAmountCents": 5000, "isItemized": true, "includeInTemplate": true, "autofillActual": true,
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update with both flags: want 204, got %d", resp.StatusCode)
	}
}
