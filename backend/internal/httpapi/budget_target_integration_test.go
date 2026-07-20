//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestCarryForwardRespectsAutofillFlag verifies that when copying a period
// (via copyFromPeriodId), budget lines for categories with autofillActual=true
// get their amountCents set to the category's defaultAmountCents, while
// categories with autofillActual=false get amountCents=0 regardless of any
// prior budget target.
func TestCarryForwardRespectsAutofillFlag(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year = 2093
	const autofillName = "ZTest Autofill Budget 2093"
	const noAutofillName = "ZTest NoAutofill Budget 2093"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1,$2)`, autofillName, noAutofillName)
	})

	// Create category with autofill enabled and defaultAmountCents=50000.
	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name":               autofillName,
		"defaultAmountCents": 50000,
		"autofillActual":     true,
		"isItemized":         false,
		"includeInTemplate":  true,
		"sortOrder":          0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create autofill category: want 201, got %d", resp.StatusCode)
	}
	var autofill struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &autofill)

	// Create category with autofill disabled and defaultAmountCents=30000.
	resp = c.do(http.MethodPost, "/api/categories", map[string]any{
		"name":               noAutofillName,
		"defaultAmountCents": 30000,
		"autofillActual":     false,
		"isItemized":         false,
		"includeInTemplate":  true,
		"sortOrder":          1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create no-autofill category: want 201, got %d", resp.StatusCode)
	}
	var noAutofill struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &noAutofill)

	// Create first period (month 1).
	period1ID := createTestPeriod(t, c, year, 1)

	// Add budget lines for both categories with a placeholder amount that
	// will be overwritten by carry-forward.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(period1ID, 10)+"/budget-lines", map[string]any{
		"categoryId":  autofill.ID,
		"amountCents": 99999,
		"sortOrder":   0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line for autofill category: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(period1ID, 10)+"/budget-lines", map[string]any{
		"categoryId":  noAutofill.ID,
		"amountCents": 99999,
		"sortOrder":   1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line for no-autofill category: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create second period (month 2) by copying from period 1.
	resp = c.do(http.MethodPost, "/api/periods", map[string]any{
		"year":             year,
		"month":            2,
		"copyFromPeriodId": period1ID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create period 2 via copy: want 201, got %d", resp.StatusCode)
	}
	var period2 struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &period2)

	// Get overview of period 2 and check budget line amounts.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(period2.ID, 10)+"/overview", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get period 2 overview: want 200, got %d", resp.StatusCode)
	}
	var overview struct {
		BudgetLines []struct {
			CategoryID  *int64 `json:"categoryId"`
			AmountCents int64  `json:"amountCents"`
		} `json:"budgetLines"`
	}
	c.decode(resp, &overview)

	// Find and verify budget lines.
	var autofillFound, noAutofillFound bool
	for _, line := range overview.BudgetLines {
		if line.CategoryID != nil {
			if *line.CategoryID == autofill.ID {
				autofillFound = true
				if line.AmountCents != 50000 {
					t.Errorf("autofill category budget line: want amountCents=50000, got %d", line.AmountCents)
				}
			} else if *line.CategoryID == noAutofill.ID {
				noAutofillFound = true
				if line.AmountCents != 0 {
					t.Errorf("no-autofill category budget line: want amountCents=0, got %d", line.AmountCents)
				}
			}
		}
	}

	if !autofillFound {
		t.Fatal("autofill category budget line not found in period 2 overview")
	}
	if !noAutofillFound {
		t.Fatal("no-autofill category budget line not found in period 2 overview")
	}
}

// TestClosedPeriodFreezesTarget verifies that closing a period freezes the
// targetCents value of its budget lines, so that subsequent category updates
// do not affect already-closed periods. When the period is reopened,
// targetCents should be recalculated from the updated category defaults.
func TestClosedPeriodFreezesTarget(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2093, 6
	const categoryName = "ZTest Freeze Target 2093"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryName)
	})

	// Create category with defaultAmountCents=10000.
	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name":               categoryName,
		"defaultAmountCents": 10000,
		"isItemized":         false,
		"includeInTemplate":  true,
		"sortOrder":          0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)

	// Create period.
	periodID := createTestPeriod(t, c, year, month)

	// Add budget line for the category.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/budget-lines", map[string]any{
		"categoryId":  cat.ID,
		"amountCents": 0,
		"sortOrder":   0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Close the period (requires income >= expenses, so add income).
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/incomes", map[string]any{
		"label": "Test income", "amountCents": 500000, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/close", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fetch period overview and capture targetCents before category update.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/overview", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get overview after close: want 200, got %d", resp.StatusCode)
	}
	var overviewBefore struct {
		BudgetLines []struct {
			CategoryID  *int64 `json:"categoryId"`
			TargetCents int64  `json:"targetCents"`
		} `json:"budgetLines"`
	}
	c.decode(resp, &overviewBefore)

	var targetBeforeUpdate int64
	for _, line := range overviewBefore.BudgetLines {
		if line.CategoryID != nil && *line.CategoryID == cat.ID {
			targetBeforeUpdate = line.TargetCents
			break
		}
	}

	// Update category's defaultAmountCents to a different value.
	resp = c.do(http.MethodPut, "/api/categories/"+strconv.FormatInt(cat.ID, 10), map[string]any{
		"name":               categoryName,
		"defaultAmountCents": 20000,
		"isItemized":         false,
		"includeInTemplate":  true,
		"sortOrder":          0,
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("update category: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fetch period overview again; targetCents should still be the old value.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/overview", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get overview after category update: want 200, got %d", resp.StatusCode)
	}
	var overviewAfterUpdate struct {
		BudgetLines []struct {
			CategoryID  *int64 `json:"categoryId"`
			TargetCents int64  `json:"targetCents"`
		} `json:"budgetLines"`
	}
	c.decode(resp, &overviewAfterUpdate)

	var targetAfterUpdate int64
	for _, line := range overviewAfterUpdate.BudgetLines {
		if line.CategoryID != nil && *line.CategoryID == cat.ID {
			targetAfterUpdate = line.TargetCents
			break
		}
	}

	if targetAfterUpdate != targetBeforeUpdate {
		t.Errorf("targetCents after category update on closed period: want %d (unchanged), got %d", targetBeforeUpdate, targetAfterUpdate)
	}

	// Reopen the period.
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reopen period: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Fetch period overview after reopen; targetCents should now be the updated value.
	resp = c.do(http.MethodGet, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/overview", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get overview after reopen: want 200, got %d", resp.StatusCode)
	}
	var overviewAfterReopen struct {
		BudgetLines []struct {
			CategoryID  *int64 `json:"categoryId"`
			TargetCents int64  `json:"targetCents"`
		} `json:"budgetLines"`
	}
	c.decode(resp, &overviewAfterReopen)

	var targetAfterReopen int64
	for _, line := range overviewAfterReopen.BudgetLines {
		if line.CategoryID != nil && *line.CategoryID == cat.ID {
			targetAfterReopen = line.TargetCents
			break
		}
	}

	if targetAfterReopen != 20000 {
		t.Errorf("targetCents after reopen: want 20000 (updated default), got %d", targetAfterReopen)
	}
}
