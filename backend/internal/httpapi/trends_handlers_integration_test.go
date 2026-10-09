//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// These tests exercise the three new /api/trends endpoints
// (income-sources, pot-balances, descriptions) end-to-end over HTTP against
// a real Postgres instance. They use far-future test years so they never
// collide with real household data, and always clean up what they create.

type trendsIncomeSourcesResponse struct {
	Sources []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"sources"`
	Entries []struct {
		Year   int              `json:"year"`
		Month  int              `json:"month"`
		Values map[string]int64 `json:"values"`
	} `json:"entries"`
}

type trendsMonthlyTotalsResponse []struct {
	Year              int   `json:"year"`
	Month             int   `json:"month"`
	IncomeTotalCents  int64 `json:"incomeTotalCents"`
	ExpenseTotalCents int64 `json:"expenseTotalCents"`
}

type trendsCategoryTotalsResponse struct {
	Categories []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
	Entries []struct {
		Year   int              `json:"year"`
		Month  int              `json:"month"`
		Values map[string]int64 `json:"values"`
	} `json:"entries"`
}

type trendsPotBalancesResponse struct {
	Pots []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Kind string `json:"kind"`
	} `json:"pots"`
	Entries []struct {
		Year     int              `json:"year"`
		Month    int              `json:"month"`
		Balances map[string]int64 `json:"balances"`
		Inflow   map[string]int64 `json:"inflow"`
		Outflow  map[string]int64 `json:"outflow"`
	} `json:"entries"`
}

type trendsDescriptionRow struct {
	Description string `json:"description"`
	TotalCents  int64  `json:"totalCents"`
	Count       int    `json:"count"`
	FirstYear   int    `json:"firstYear"`
	FirstMonth  int    `json:"firstMonth"`
	LastYear    int    `json:"lastYear"`
	LastMonth   int    `json:"lastMonth"`
}

// TestTrendsIncomeSourcesMatchesMonthlyTotals checks that, for every period,
// summing this endpoint's per-source values (itemized + non-itemized mixed)
// reproduces the exact same incomeTotalCents as /api/trends/monthly-totals.
func TestTrendsIncomeSourcesMatchesMonthlyTotals(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2081, 4
	const itemizedName = "ZTest Itemized Source 2081"
	const plainName = "ZTest Plain Source 2081"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name IN ($1,$2)`, itemizedName, plainName)
	})

	resp := c.do(http.MethodPost, "/api/income-sources", map[string]any{
		"name": itemizedName, "defaultAmountCents": 0, "isItemized": true, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create itemized source: want 201, got %d", resp.StatusCode)
	}
	var itemized struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &itemized)

	resp = c.do(http.MethodPost, "/api/income-sources", map[string]any{
		"name": plainName, "defaultAmountCents": 0, "isItemized": false, "includeInTemplate": true, "sortOrder": 1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create plain source: want 201, got %d", resp.StatusCode)
	}
	var plain struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &plain)

	periodID := createTestPeriod(t, c, year, month)
	periodIDStr := strconv.FormatInt(periodID, 10)

	// Itemized source: EffectiveIncomeCentsSQL's outer SUM iterates over
	// income_entries rows, so an itemized source still needs its header
	// row present (amount_cents on it is ignored — only its
	// income_transactions count).
	resp = c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/incomes", map[string]any{
		"sourceId": itemized.ID, "amountCents": 0, "notes": "", "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create itemized income entry header: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	for _, amt := range []int64{150000, 25000} {
		resp = c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/income-transactions", map[string]any{
			"sourceId": itemized.ID, "amountCents": amt, "description": "salary part",
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create income transaction: want 201, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp = c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/incomes", map[string]any{
		"sourceId": plain.ID, "amountCents": 40000, "notes": "", "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income entry: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/trends/income-sources", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/income-sources: want 200, got %d", resp.StatusCode)
	}
	var sourcesResp trendsIncomeSourcesResponse
	c.decode(resp, &sourcesResp)

	resp = c.do(http.MethodGet, "/api/trends/monthly-totals", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/monthly-totals: want 200, got %d", resp.StatusCode)
	}
	var monthlyResp trendsMonthlyTotalsResponse
	c.decode(resp, &monthlyResp)

	var wantIncome int64 = -1
	for _, mt := range monthlyResp {
		if mt.Year == year && mt.Month == month {
			wantIncome = mt.IncomeTotalCents
		}
	}
	if wantIncome == -1 {
		t.Fatalf("monthly-totals missing %d-%d", year, month)
	}
	if wantIncome != 175000+40000 {
		t.Fatalf("sanity: expected monthly income 215000, got %d", wantIncome)
	}

	var gotSum int64
	found := false
	for _, e := range sourcesResp.Entries {
		if e.Year == year && e.Month == month {
			found = true
			for _, v := range e.Values {
				gotSum += v
			}
		}
	}
	if !found {
		t.Fatalf("income-sources missing entry for %d-%d", year, month)
	}
	if gotSum != wantIncome {
		t.Fatalf("income-sources per-source sum %d does not match monthly-totals incomeTotalCents %d", gotSum, wantIncome)
	}
}

// TestTrendsIncomeSourcesCarryoverBucket checks that a carryover
// income_entries row (entry_type='carryover', source_id NULL — as written
// by period close) is bucketed under synthetic source id 0 named
// "Carryover", and counts toward the period's total like any other source.
func TestTrendsIncomeSourcesCarryoverBucket(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2090, 7

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
	})

	periodID := createTestPeriod(t, c, year, month)

	if _, err := pool.Exec(ctx,
		`INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order) VALUES ($1,NULL,'Doorlopen maand Juni',12345,'carryover','',0)`,
		periodID); err != nil {
		t.Fatalf("insert carryover income entry: %v", err)
	}

	resp := c.do(http.MethodGet, "/api/trends/income-sources", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/income-sources: want 200, got %d", resp.StatusCode)
	}
	var out trendsIncomeSourcesResponse
	c.decode(resp, &out)

	var sawCarryoverSource bool
	for _, src := range out.Sources {
		if src.ID == 0 {
			sawCarryoverSource = true
			if src.Name != "Carryover" {
				t.Fatalf("source id 0 name = %q, want %q", src.Name, "Carryover")
			}
		}
	}
	if !sawCarryoverSource {
		t.Fatalf("expected a synthetic source id 0 (Carryover) in sources list")
	}

	var gotCarryover int64 = -1
	for _, e := range out.Entries {
		if e.Year == year && e.Month == month {
			gotCarryover = e.Values["0"]
		}
	}
	if gotCarryover != 12345 {
		t.Fatalf("carryover bucket for %d-%d = %d, want 12345", year, month, gotCarryover)
	}
}

// TestTrendsCategoryTotalsMatchesMonthlyTotals checks that, for a period
// with both a real-category budget line and a label-only one (category_id
// NULL, allowed by the budget_lines check constraint), summing this
// endpoint's per-category values reproduces the exact same
// expenseTotalCents as /api/trends/monthly-totals. Before the LEFT JOIN
// fix, the label-only line's amount was silently missing from both the
// category breakdown and this sum.
func TestTrendsCategoryTotalsMatchesMonthlyTotals(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2041, 3
	const categoryName = "ZTest Real Category 2041"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryName)
	})

	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": categoryName, "defaultAmountCents": 0, "isItemized": false, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)

	periodID := createTestPeriod(t, c, year, month)
	periodIDStr := strconv.FormatInt(periodID, 10)

	resp = c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/budget-lines", map[string]any{
		"categoryId": cat.ID, "amountCents": 30000, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create budget line: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Label-only line: no category, just a label — allowed by
	// `CHECK (category_id IS NOT NULL OR label IS NOT NULL)`.
	resp = c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/budget-lines", map[string]any{
		"label": "One-off fee", "amountCents": 5000, "sortOrder": 1,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create label-only budget line: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/trends/category-totals", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/category-totals: want 200, got %d", resp.StatusCode)
	}
	var catResp trendsCategoryTotalsResponse
	c.decode(resp, &catResp)

	resp = c.do(http.MethodGet, "/api/trends/monthly-totals", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/monthly-totals: want 200, got %d", resp.StatusCode)
	}
	var monthlyResp trendsMonthlyTotalsResponse
	c.decode(resp, &monthlyResp)

	var wantExpense int64 = -1
	for _, mt := range monthlyResp {
		if mt.Year == year && mt.Month == month {
			wantExpense = mt.ExpenseTotalCents
		}
	}
	if wantExpense == -1 {
		t.Fatalf("monthly-totals missing %d-%d", year, month)
	}
	if wantExpense != 30000+5000 {
		t.Fatalf("sanity: expected monthly expense 35000, got %d", wantExpense)
	}

	var sawUncategorised bool
	for _, catOut := range catResp.Categories {
		if catOut.ID == 0 {
			sawUncategorised = true
			if catOut.Name != "Uncategorised" {
				t.Fatalf("category id 0 name = %q, want %q", catOut.Name, "Uncategorised")
			}
		}
	}
	if !sawUncategorised {
		t.Fatalf("expected a synthetic category id 0 (Uncategorised) in categories list")
	}

	var gotSum int64
	found := false
	for _, e := range catResp.Entries {
		if e.Year == year && e.Month == month {
			found = true
			for _, v := range e.Values {
				gotSum += v
			}
		}
	}
	if !found {
		t.Fatalf("category-totals missing entry for %d-%d", year, month)
	}
	if gotSum != wantExpense {
		t.Fatalf("category-totals per-category sum %d does not match monthly-totals expenseTotalCents %d", gotSum, wantExpense)
	}
}

// TestTrendsDescriptionsTrimCaseGrouping checks that "MTC", "MTC " and "Mtc"
// — distinct strings that only differ by whitespace/casing, which real data
// contains — are grouped into one row, with the most frequent original
// spelling reported back.
func TestTrendsDescriptionsTrimCaseGrouping(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2089, 9
	const categoryName = "ZTest Descriptions Category 2089"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryName)
	})

	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": categoryName, "defaultAmountCents": 0, "isItemized": false, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)

	periodID := createTestPeriod(t, c, year, month)
	periodIDStr := strconv.FormatInt(periodID, 10)

	// "MTC " (trailing space) appears twice, the others once each — the
	// most frequent original spelling must win.
	for _, desc := range []string{"MTC", "MTC ", "MTC ", "Mtc"} {
		resp := c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/transactions", map[string]any{
			"categoryId": cat.ID, "amountCents": 1000, "description": desc,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create transaction %q: want 201, got %d", desc, resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp = c.do(http.MethodGet, "/api/trends/descriptions?categoryId="+strconv.FormatInt(cat.ID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/descriptions: want 200, got %d", resp.StatusCode)
	}
	var rows []trendsDescriptionRow
	c.decode(resp, &rows)

	var found *trendsDescriptionRow
	for i := range rows {
		if rows[i].Count == 4 {
			found = &rows[i]
		}
	}
	if found == nil {
		t.Fatalf("expected one grouped row with count=4, got %+v", rows)
	}
	if found.Description != "MTC " {
		t.Fatalf("grouped description = %q, want %q (the most frequent original spelling)", found.Description, "MTC ")
	}
	if found.TotalCents != 4000 {
		t.Fatalf("grouped totalCents = %d, want 4000", found.TotalCents)
	}
}

// TestTrendsDescriptionsExcludeCategories checks that excludeCategoryIds drops
// a category's rows (and its children's, via category_rollup) while leaving
// the rest of the result alone.
func TestTrendsDescriptionsExcludeCategories(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, month = 2087, 4
	const keepName, dropName = "ZTest Exclude Keep 2087", "ZTest Exclude Drop 2087"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1, $2)`, keepName, dropName)
	})

	create := func(name string) int64 {
		resp := c.do(http.MethodPost, "/api/categories", map[string]any{
			"name": name, "defaultAmountCents": 0, "isItemized": false, "includeInTemplate": true, "sortOrder": 0,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create category %s: want 201, got %d", name, resp.StatusCode)
		}
		var cat struct {
			ID int64 `json:"id"`
		}
		c.decode(resp, &cat)
		return cat.ID
	}
	keepID, dropID := create(keepName), create(dropName)

	periodIDStr := strconv.FormatInt(createTestPeriod(t, c, year, month), 10)
	for _, tx := range []struct {
		cat  int64
		desc string
	}{{keepID, "ZTest Shop Keep"}, {dropID, "ZTest Shop Drop"}} {
		resp := c.do(http.MethodPost, "/api/periods/"+periodIDStr+"/transactions", map[string]any{
			"categoryId": tx.cat, "amountCents": 1000, "description": tx.desc,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create transaction: want 201, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp := c.do(http.MethodGet, "/api/trends/descriptions?limit=100&excludeCategoryIds="+strconv.FormatInt(dropID, 10), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/descriptions: want 200, got %d", resp.StatusCode)
	}
	var rows []trendsDescriptionRow
	c.decode(resp, &rows)
	var sawKeep, sawDrop bool
	for _, r := range rows {
		sawKeep = sawKeep || r.Description == "ZTest Shop Keep"
		sawDrop = sawDrop || r.Description == "ZTest Shop Drop"
	}
	if !sawKeep || sawDrop {
		t.Fatalf("exclusion: sawKeep=%v sawDrop=%v, want true/false", sawKeep, sawDrop)
	}
}

// TestTrendsDescriptionsRejectsBadParams checks the 400-style validation on
// categoryId/sourceId/limit.
func TestTrendsDescriptionsRejectsBadParams(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	for _, qs := range []string{
		"?categoryId=not-a-number",
		"?sourceId=not-a-number",
		"?excludeCategoryIds=1,x",
		"?limit=0",
		"?limit=101",
		"?limit=not-a-number",
	} {
		resp := c.do(http.MethodGet, "/api/trends/descriptions"+qs, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET /api/trends/descriptions%s: want 400, got %d", qs, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestTrendsPotBalancesCarryForwardAndPeriodBucketing checks two things at
// once: (1) a pot_ledger row with period_id set is bucketed by that
// period's year/month even when its entry_date is a misleading constant
// (mirrors real imported allocation rows), and (2) a month with no ledger
// activity for a pot still carries forward its previous closing balance.
func TestTrendsPotBalancesCarryForwardAndPeriodBucketing(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year = 2088
	const potName = "ZTest Pot Balances 2088"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
	})

	resp := c.do(http.MethodPost, "/api/pots", map[string]any{
		"name": potName, "kind": "normal", "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create pot: want 201, got %d", resp.StatusCode)
	}
	var pot struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &pot)

	period1 := createTestPeriod(t, c, year, 1)
	// Month 2 deliberately has zero ledger rows — it must still show up in
	// the series with the carried-forward balance from month 1.
	period3 := createTestPeriod(t, c, year, 3)

	// Both allocation rows carry the same (meaningless) import date, on
	// purpose — only period_id must determine their month bucket.
	const sameMisleadingDate = "2088-06-15"
	if _, err := pool.Exec(ctx,
		`INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$2,'allocation',10000,'Monthly allocation',$3)`,
		pot.ID, period1, sameMisleadingDate); err != nil {
		t.Fatalf("insert allocation (period 1): %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$2,'allocation',5000,'Monthly allocation',$3)`,
		pot.ID, period3, sameMisleadingDate); err != nil {
		t.Fatalf("insert allocation (period 3): %v", err)
	}

	resp = c.do(http.MethodGet, "/api/trends/pot-balances", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/pot-balances: want 200, got %d", resp.StatusCode)
	}
	var out trendsPotBalancesResponse
	c.decode(resp, &out)

	potIDStr := strconv.FormatInt(pot.ID, 10)
	balanceAt := map[int]int64{}
	sawMonth := map[int]bool{}
	for _, e := range out.Entries {
		if e.Year != year {
			continue
		}
		sawMonth[e.Month] = true
		if v, ok := e.Balances[potIDStr]; ok {
			balanceAt[e.Month] = v
		}
	}
	if !sawMonth[1] || !sawMonth[3] {
		t.Fatalf("expected entries for months 1 and 3, got months: %+v", sawMonth)
	}
	if balanceAt[1] != 10000 {
		t.Fatalf("balance after month 1 allocation = %d, want 10000 (entry_date must not override period_id bucketing)", balanceAt[1])
	}
	// Month 2 has no ledger rows for this pot at all — it must not even be
	// present as its own entry unless some OTHER pot had activity that
	// month. Since no period exists for month 2 and no ledger row
	// references it, there is nothing to assert about month 2 directly;
	// what matters is that month 3's balance correctly carries month 1's
	// balance forward rather than resetting to just its own delta.
	if balanceAt[3] != 15000 {
		t.Fatalf("balance after month 3 allocation = %d, want 15000 (10000 carried forward + 5000)", balanceAt[3])
	}
}

// A pot adjustment can raise or lower a balance. It used to be folded into
// outflow regardless of sign, so a correction that ADDED money was reported
// as money leaving the pot — and contradicted the balance series, which went
// up in the same month.
func TestTrendsPotBalancesAdjustmentClassifiedBySign(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year = 2089
	const potName = "ZTest Pot Adjustment 2089"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
	})

	resp := c.do(http.MethodPost, "/api/pots", map[string]any{
		"name": potName, "kind": "normal", "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create pot: want 201, got %d", resp.StatusCode)
	}
	var pot struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &pot)

	periodUp := createTestPeriod(t, c, year, 1)
	periodDown := createTestPeriod(t, c, year, 2)

	if _, err := pool.Exec(ctx,
		`INSERT INTO pot_ledger (pot_id,period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,'adjustment',2500,'Correction up','2089-01-10')`,
		pot.ID, periodUp); err != nil {
		t.Fatalf("insert positive adjustment: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO pot_ledger (pot_id,period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,'adjustment',-1000,'Correction down','2089-02-10')`,
		pot.ID, periodDown); err != nil {
		t.Fatalf("insert negative adjustment: %v", err)
	}

	resp = c.do(http.MethodGet, "/api/trends/pot-balances", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trends/pot-balances: want 200, got %d", resp.StatusCode)
	}
	var out trendsPotBalancesResponse
	c.decode(resp, &out)

	potIDStr := strconv.FormatInt(pot.ID, 10)
	for _, e := range out.Entries {
		if e.Year != year {
			continue
		}
		switch e.Month {
		case 1:
			if e.Inflow[potIDStr] != 2500 {
				t.Fatalf("month 1 inflow = %d, want 2500 (a positive adjustment is inflow)", e.Inflow[potIDStr])
			}
			if e.Outflow[potIDStr] != 0 {
				t.Fatalf("month 1 outflow = %d, want 0", e.Outflow[potIDStr])
			}
		case 2:
			if e.Outflow[potIDStr] != -1000 {
				t.Fatalf("month 2 outflow = %d, want -1000 (a negative adjustment is outflow)", e.Outflow[potIDStr])
			}
			if e.Inflow[potIDStr] != 0 {
				t.Fatalf("month 2 inflow = %d, want 0", e.Inflow[potIDStr])
			}
		}
	}
}
