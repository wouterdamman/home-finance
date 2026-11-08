//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This file owns the years 2083-2086 and asserts only on them.

type ttPeriod struct {
	Year, Month                             int
	Status                                  string
	ClosedAt                                *string
	IsCurrentMonth                          bool
	IncomeCents, ExpenseCents, SurplusCents int64
	GeneratedAt                             string
}

type ttYear struct {
	Year                                                                int
	Locked                                                              bool
	PeriodsOpen, PeriodsClosed                                          int
	IncomeCents, ExpenseCents, SurplusCents, AverageMonthlySurplusCents int64
	GeneratedAt                                                         string
}

func ttExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func ttID(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

func ttPeriodRow(t *testing.T, pool *pgxpool.Pool, y, m int, status string) int64 {
	t.Helper()
	return ttID(t, pool, `INSERT INTO periods (year, month, status, closed_at) VALUES ($1,$2,$3, CASE WHEN $3='closed' THEN now() END) RETURNING id`, y, m, status)
}

func TestIntegrationPeriodYearTotals(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)

	cleanup := func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year BETWEEN 2083 AND 2086`)
		pool.Exec(ctx, `DELETE FROM categories WHERE name LIKE 'ZTotals %' AND parent_id IS NOT NULL`)
		pool.Exec(ctx, `DELETE FROM categories WHERE name LIKE 'ZTotals %'`)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name LIKE 'ZTotals %'`)
		pool.Exec(ctx, `DELETE FROM years WHERE year BETWEEN 2083 AND 2086`)
		pool.Exec(ctx, `DELETE FROM locked_years WHERE year BETWEEN 2083 AND 2086`)
	}
	cleanup()
	t.Cleanup(cleanup)

	both := createTestToken(t, c, pool, "itest totals", []string{"periods:read", "years:read"})
	onlyPeriods := createTestToken(t, c, pool, "itest totals p", []string{"periods:read"})
	onlyYears := createTestToken(t, c, pool, "itest totals y", []string{"years:read"})
	noScope := createTestToken(t, c, pool, "itest totals none", []string{"kids:read"})

	// A year with no row in `years` and no periods is a 404 (2086 is filled in further down).
	tkGet(t, srv.URL, "/api/integrations/v1/years/2086", both.Token, http.StatusNotFound)

	// ── Fixture 2083: one closed + one open period ──
	src := ttID(t, pool, `INSERT INTO income_sources (name) VALUES ('ZTotals plain '||$1) RETURNING id`, sfx)
	itemSrc := ttID(t, pool, `INSERT INTO income_sources (name, is_itemized) VALUES ('ZTotals itemized '||$1, true) RETURNING id`, sfx)
	parent := ttID(t, pool, `INSERT INTO categories (name) VALUES ('ZTotals parent '||$1) RETURNING id`, sfx)
	child := ttID(t, pool, `INSERT INTO categories (name, parent_id) VALUES ('ZTotals child '||$1, $2) RETURNING id`, sfx, parent)
	flat := ttID(t, pool, `INSERT INTO categories (name) VALUES ('ZTotals flat '||$1) RETURNING id`, sfx)

	p1 := ttPeriodRow(t, pool, 2083, 1, "closed")
	p2 := ttPeriodRow(t, pool, 2083, 2, "open")
	// Income: plain 300000 + itemized (entry amount 999 is ignored; transactions 120000+30000).
	ttExec(t, pool, `INSERT INTO income_entries (period_id, source_id, amount_cents) VALUES ($1,$2,300000)`, p1, src)
	ttExec(t, pool, `INSERT INTO income_entries (period_id, source_id, amount_cents) VALUES ($1,$2,999)`, p1, itemSrc)
	ttExec(t, pool, `INSERT INTO income_transactions (period_id, source_id, amount_cents) VALUES ($1,$2,120000),($1,$2,30000)`, p1, itemSrc)
	// Expense: flat 100000 + tracked parent line (amount 77777 ignored; child transactions 20000+5000 roll up).
	ttExec(t, pool, `INSERT INTO budget_lines (period_id, category_id, amount_cents) VALUES ($1,$2,100000)`, p1, flat)
	ttExec(t, pool, `INSERT INTO budget_lines (period_id, category_id, amount_cents, tracks_transactions) VALUES ($1,$2,77777,true)`, p1, parent)
	ttExec(t, pool, `INSERT INTO transactions (period_id, category_id, amount_cents) VALUES ($1,$2,20000),($1,$2,5000)`, p1, child)
	// 2083-02 (open): income 100000, expense 40000.
	ttExec(t, pool, `INSERT INTO income_entries (period_id, source_id, amount_cents) VALUES ($1,$2,100000)`, p2, src)
	ttExec(t, pool, `INSERT INTO budget_lines (period_id, category_id, amount_cents) VALUES ($1,$2,40000)`, p2, flat)

	// ── Fixture 2084: locked, two closed periods, negative surplus (-201 -> average -101) ──
	q1 := ttPeriodRow(t, pool, 2084, 1, "closed")
	q2 := ttPeriodRow(t, pool, 2084, 2, "closed")
	ttExec(t, pool, `INSERT INTO budget_lines (period_id, category_id, amount_cents) VALUES ($1,$2,100)`, q1, flat)
	ttExec(t, pool, `INSERT INTO budget_lines (period_id, category_id, amount_cents) VALUES ($1,$2,101)`, q2, flat)
	ttExec(t, pool, `INSERT INTO years (year) VALUES (2084)`)
	ttExec(t, pool, `INSERT INTO locked_years (year) VALUES (2084)`)

	// ── 2085: in `years` only, no periods ──
	ttExec(t, pool, `INSERT INTO years (year) VALUES (2085)`)

	// ── 2086: periods only, no `years` row ──
	ttPeriodRow(t, pool, 2086, 3, "open")

	getT := func(tok, path string, status int) []byte {
		b := tkGet(t, srv.URL, "/api/integrations/v1"+path, tok, status)
		if status == http.StatusOK {
			tpNoForbiddenKeys(t, path, b)
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if _, ok := m["id"]; ok {
				t.Errorf("%s: id key present", path)
			}
		}
		return b
	}
	period := func(path string) ttPeriod {
		b := getT(both.Token, path, http.StatusOK)
		var p ttPeriod
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339, p.GeneratedAt); err != nil {
			t.Errorf("generatedAt: %v", err)
		}
		return p
	}
	year := func(path string) (ttYear, []byte) {
		b := getT(both.Token, path, http.StatusOK)
		var y ttYear
		if err := json.Unmarshal(b, &y); err != nil {
			t.Fatal(err)
		}
		return y, b
	}

	// ── Months ──
	m1 := period("/periods/2083-01")
	t.Logf("GET /periods/2083-01:\n%s", getT(both.Token, "/periods/2083-01", http.StatusOK))
	if m1.Year != 2083 || m1.Month != 1 || m1.Status != "closed" || m1.ClosedAt == nil || m1.IsCurrentMonth ||
		m1.IncomeCents != 450000 || m1.ExpenseCents != 125000 || m1.SurplusCents != 325000 {
		t.Errorf("2083-01: %+v", m1)
	}
	if m1.ClosedAt != nil {
		if ts, err := time.Parse(time.RFC3339, *m1.ClosedAt); err != nil || !strings.HasSuffix(*m1.ClosedAt, "Z") || ts.IsZero() {
			t.Errorf("closedAt %q not RFC3339 UTC: %v", *m1.ClosedAt, err)
		}
	}
	m2 := period("/periods/2083-02")
	if m2.Status != "open" || m2.ClosedAt != nil || m2.IncomeCents != 100000 || m2.ExpenseCents != 40000 || m2.SurplusCents != 60000 {
		t.Errorf("2083-02: %+v", m2)
	}
	if b := getT(both.Token, "/periods/2083-02", http.StatusOK); !strings.Contains(string(b), `"closedAt":null`) {
		t.Errorf("open period should serialise closedAt:null: %s", b)
	}
	m3 := period("/periods/2086-03")
	if m3.Status != "open" || m3.IncomeCents != 0 || m3.ExpenseCents != 0 || m3.SurplusCents != 0 {
		t.Errorf("2086-03: %+v", m3)
	}
	t.Logf("GET /periods/current:\n%s", getT(both.Token, "/periods/current", http.StatusOK))
	cur := period("/periods/current") // which period depends on real "now" and other tests; shape only
	if cur.Year == 0 || cur.Month < 1 || cur.Month > 12 || (cur.Status != "open" && cur.Status != "closed") {
		t.Errorf("current: %+v", cur)
	}

	// ── Years ──
	y83, body := year("/years/2083")
	t.Logf("GET /years/2083:\n%s", body)
	want83 := ttYear{Year: 2083, PeriodsOpen: 1, PeriodsClosed: 1, IncomeCents: 550000, ExpenseCents: 165000, SurplusCents: 385000, AverageMonthlySurplusCents: 192500}
	want83.GeneratedAt = y83.GeneratedAt
	if y83 != want83 {
		t.Errorf("2083: got %+v want %+v", y83, want83)
	}
	y84, _ := year("/years/2084")
	want84 := ttYear{Year: 2084, Locked: true, PeriodsClosed: 2, ExpenseCents: 201, SurplusCents: -201, AverageMonthlySurplusCents: -101, GeneratedAt: y84.GeneratedAt}
	if y84 != want84 {
		t.Errorf("2084: got %+v want %+v", y84, want84)
	}
	y85, _ := year("/years/2085")
	if want := (ttYear{Year: 2085, GeneratedAt: y85.GeneratedAt}); y85 != want {
		t.Errorf("2085: got %+v want %+v", y85, want)
	}
	y86, _ := year("/years/2086")
	if want := (ttYear{Year: 2086, PeriodsOpen: 1, GeneratedAt: y86.GeneratedAt}); y86 != want {
		t.Errorf("2086: got %+v want %+v", y86, want)
	}

	// Totals equal the session API's year summary.
	for _, y := range []ttYear{y83, y84, y85, y86} {
		resp := c.do(http.MethodGet, "/api/years/"+strconv.Itoa(y.Year)+"/summary", nil)
		var sum struct {
			YearIncomeTotalCents  int64 `json:"yearIncomeTotalCents"`
			YearExpenseTotalCents int64 `json:"yearExpenseTotalCents"`
			Locked                bool  `json:"locked"`
		}
		c.decode(resp, &sum)
		if sum.YearIncomeTotalCents != y.IncomeCents || sum.YearExpenseTotalCents != y.ExpenseCents || sum.Locked != y.Locked {
			t.Errorf("%d: integration %+v differs from session summary %+v", y.Year, y, sum)
		}
	}

	// List contains our years, oldest first, with the same totals.
	b := getT(both.Token, "/years", http.StatusOK)
	t.Logf("GET /years (fixture years only): see entries below\n%s", b)
	var list struct {
		Years       []ttYear
		GeneratedAt string
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	got := map[int]ttYear{}
	last := 0
	for _, y := range list.Years {
		if y.Year <= last {
			t.Errorf("years not strictly ascending at %d after %d", y.Year, last)
		}
		last = y.Year
		if y.Year >= 2083 && y.Year <= 2086 {
			y.GeneratedAt = ""
			got[y.Year] = y
		}
	}
	for _, w := range []ttYear{y83, y84, y85, y86} {
		w.GeneratedAt = ""
		if got[w.Year] != w {
			t.Errorf("list %d: got %+v want %+v", w.Year, got[w.Year], w)
		}
	}
	if len(got) != 4 {
		t.Errorf("list should contain all four fixture years, got %v", got)
	}
	cy, cyBody := year("/years/current")
	t.Logf("GET /years/current:\n%s", cyBody)
	if cy.Year != time.Now().In(mustAmsterdam(t)).Year() {
		t.Errorf("years/current: %+v", cy)
	}

	// ── 404s ──
	for _, p := range []string{"/periods/2083-03", "/periods/2083-13", "/periods/2083-00", "/periods/2083-1", "/periods/abc", "/periods/2083-01x", "/periods/9999-01",
		"/years/1999", "/years/2101", "/years/abc", "/years/208", "/years/2083-01"} {
		bb := tkGet(t, srv.URL, "/api/integrations/v1"+p, both.Token, http.StatusNotFound)
		if !strings.Contains(string(bb), "not_found") {
			t.Errorf("%s: %s", p, bb)
		}
	}
	// After cleaning 2086 a valid but absent year is still 404.
	ttExec(t, pool, `DELETE FROM periods WHERE year=2086`)
	tkGet(t, srv.URL, "/api/integrations/v1/years/2086", both.Token, http.StatusNotFound)

	// ── scopes ──
	for _, p := range []string{"/periods/current", "/periods/2083-01"} {
		for _, tok := range []string{noScope.Token, onlyYears.Token} {
			if bb := tkGet(t, srv.URL, "/api/integrations/v1"+p, tok, http.StatusForbidden); !strings.Contains(string(bb), "insufficient_scope") {
				t.Errorf("%s: %s", p, bb)
			}
		}
		tkGet(t, srv.URL, "/api/integrations/v1"+p, onlyPeriods.Token, http.StatusOK)
		tkGet(t, srv.URL, "/api/integrations/v1"+p, "", http.StatusUnauthorized)
	}
	for _, p := range []string{"/years", "/years/current", "/years/2083"} {
		for _, tok := range []string{noScope.Token, onlyPeriods.Token} {
			if bb := tkGet(t, srv.URL, "/api/integrations/v1"+p, tok, http.StatusForbidden); !strings.Contains(string(bb), "insufficient_scope") {
				t.Errorf("%s: %s", p, bb)
			}
		}
		tkGet(t, srv.URL, "/api/integrations/v1"+p, onlyYears.Token, http.StatusOK)
		tkGet(t, srv.URL, "/api/integrations/v1"+p, "", http.StatusUnauthorized)
	}
}

func mustAmsterdam(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
