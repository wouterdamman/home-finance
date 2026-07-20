//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestCategoryRollupFoldsChildTransactionsIntoParent exercises the
// category_rollup join (migration 0011) end-to-end over HTTP: a parent
// category's tracked budget line should sum both its own transactions and
// its child category's, and the transactions drill-down should include
// child rows when filtered by the parent while staying scoped to just its
// own rows when filtered by the child.
func TestCategoryRollupFoldsChildTransactionsIntoParent(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2099, 5
	const parentName = "ZTest Rollup Parent 2099"
	const childName = "ZTest Rollup Child 2099"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1,$2)`, parentName, childName)
	})

	// Parent must be is_itemized=true so its budget line tracks
	// transactions (tracksTransactions mirrors the category flag at
	// budget-line creation time — see handleCreateBudgetLine).
	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": parentName, "defaultAmountCents": 0, "isItemized": true, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create parent category: want 201, got %d", resp.StatusCode)
	}
	var parent struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &parent)

	resp = c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": childName, "defaultAmountCents": 0, "isItemized": true, "includeInTemplate": true, "sortOrder": 1, "parentId": parent.ID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create child category: want 201, got %d", resp.StatusCode)
	}
	var child struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &child)

	periodID := createTestPeriod(t, c, year, month)

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/budget-lines", map[string]any{
		"categoryId": parent.ID, "amountCents": 0, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line: want 201, got %d", resp.StatusCode)
	}
	var bl struct {
		TracksTransactions bool `json:"tracksTransactions"`
	}
	c.decode(resp, &bl)
	if !bl.TracksTransactions {
		t.Fatalf("expected budget line to track transactions (parent category is_itemized=true)")
	}

	createTx := func(categoryID, amountCents int64, desc string) {
		resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/transactions", map[string]any{
			"categoryId": categoryID, "amountCents": amountCents, "description": desc,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create transaction %q: want 201, got %d", desc, resp.StatusCode)
		}
		resp.Body.Close()
	}
	createTx(parent.ID, 500, "parent's own transaction")
	createTx(child.ID, 700, "child transaction 1")
	createTx(child.ID, 800, "child transaction 2")

	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/overview", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get overview: want 200, got %d", resp.StatusCode)
	}
	var overview struct {
		BudgetLines []struct {
			CategoryID     *int64 `json:"categoryId"`
			EffectiveCents int64  `json:"effectiveCents"`
		} `json:"budgetLines"`
	}
	c.decode(resp, &overview)
	var found bool
	for _, l := range overview.BudgetLines {
		if l.CategoryID != nil && *l.CategoryID == parent.ID {
			found = true
			if l.EffectiveCents != 2000 {
				t.Errorf("parent effectiveCents: want 2000 (500+700+800), got %d", l.EffectiveCents)
			}
		}
	}
	if !found {
		t.Fatal("parent budget line not found in overview response")
	}

	type txRow struct {
		ID         int64 `json:"id"`
		CategoryID int64 `json:"categoryId"`
	}

	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/transactions?categoryId="+strconv.FormatInt(parent.ID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list transactions by parent: want 200, got %d", resp.StatusCode)
	}
	var byParent []txRow
	c.decode(resp, &byParent)
	if len(byParent) != 3 {
		t.Errorf("transactions filtered by parent categoryId: want 3 (own + 2 child), got %d", len(byParent))
	}

	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/transactions?categoryId="+strconv.FormatInt(child.ID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list transactions by child: want 200, got %d", resp.StatusCode)
	}
	var byChild []txRow
	c.decode(resp, &byChild)
	if len(byChild) != 2 {
		t.Errorf("transactions filtered by child categoryId: want 2, got %d", len(byChild))
	}
	for _, tx := range byChild {
		if tx.CategoryID != child.ID {
			t.Errorf("transaction %d filtered by child categoryId has categoryId=%d, want %d", tx.ID, tx.CategoryID, child.ID)
		}
	}
}
