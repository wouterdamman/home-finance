//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tpPeriod struct{ AllocationsCents, DepositsCents, WithdrawalsCents, CarryoverOutCents, AdjustmentsCents, NetCents int64 }

type tpFlows struct {
	OpeningBalanceCents, AllocationsCents, DepositsCents, WithdrawalsCents, CarryoverOutCents, AdjustmentsCents int64
	ThisMonth, ThisYear                                                                                         tpPeriod
}

type tpDetail struct {
	Slug, Name, Kind string
	BalanceCents     int64
	Flows            tpFlows
	Target           *struct {
		TargetCents         int64
		TargetDate          *string
		RemainingCents      int64
		ProgressPercent     *float64
		Reached             bool
		MonthsLeft          *int
		NeededPerMonthCents *int64
	}
	Split *struct {
		Percentage              float64
		PeriodYear, PeriodMonth int
		PeriodStatus            string
	}
	LastEntry *struct {
		Date, Type  string
		AmountCents int64
	}
	GeneratedAt string
}

type tpEntry struct {
	typ           string
	cents         int64 // signed as stored
	date          time.Time
	periodID      *int64
	inMonth, inYr bool // expected bucketing, independent of the SQL under test
}

// tpEnsurePeriod returns the period (year, month), creating it only when absent.
// A period created here is removed again; an existing one is never touched.
func tpEnsurePeriod(t *testing.T, pool *pgxpool.Pool, year, month int, status string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	var st string
	err := pool.QueryRow(ctx, `SELECT id, status FROM periods WHERE year=$1 AND month=$2`, year, month).Scan(&id, &st)
	if err == nil {
		if status != "" && st != status {
			t.Fatalf("existing period %d-%d is %s, fixture needs %s", year, month, st, status)
		}
		return id
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if status == "" {
		status = "open"
	}
	if err := pool.QueryRow(ctx, `INSERT INTO periods (year, month, status) VALUES ($1,$2,$3) RETURNING id`, year, month, status).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM periods WHERE id=$1`, id) })
	return id
}

func tpInsertPot(t *testing.T, pool *pgxpool.Pool, name string, archived bool, targetCents *int64, targetDate *time.Time, es []tpEntry) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO pots (name, kind, sort_order, archived_at, target_cents, target_date)
		VALUES ($1, 'normal', 1000, CASE WHEN $2 THEN now() END, $3, $4) RETURNING id`, name, archived, targetCents, targetDate).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM pot_ledger WHERE pot_id=$1`, id)
		pool.Exec(ctx, `DELETE FROM pot_splits WHERE pot_id=$1`, id)
		pool.Exec(ctx, `DELETE FROM pots WHERE id=$1`, id)
	})
	for _, e := range es {
		if _, err := pool.Exec(ctx, `INSERT INTO pot_ledger (pot_id, period_id, entry_type, amount_cents, description, entry_date)
			VALUES ($1,$2,$3,$4,'secret note',$5)`, id, e.periodID, e.typ, e.cents, e.date.Format("2006-01-02")); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func tpInsertSplit(t *testing.T, pool *pgxpool.Pool, periodID, potID int64, pct float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO pot_splits (period_id, pot_id, percentage) VALUES ($1,$2,$3)`, periodID, potID, pct); err != nil {
		t.Fatal(err)
	}
}

// tpExpected sums the fixture independently of the SQL under test.
func tpExpected(es []tpEntry) (bal int64, f tpFlows) {
	for _, e := range es {
		bal += e.cents
		var p []*tpPeriod
		if e.inMonth {
			p = append(p, &f.ThisMonth)
		}
		if e.inYr {
			p = append(p, &f.ThisYear)
		}
		add := func(total *int64, sign int64, get func(*tpPeriod) *int64) {
			*total += sign * e.cents
			for _, x := range p {
				*get(x) += sign * e.cents
			}
		}
		switch e.typ {
		case "opening_balance":
			f.OpeningBalanceCents += e.cents
		case "allocation":
			add(&f.AllocationsCents, 1, func(x *tpPeriod) *int64 { return &x.AllocationsCents })
		case "deposit":
			add(&f.DepositsCents, 1, func(x *tpPeriod) *int64 { return &x.DepositsCents })
		case "withdrawal":
			add(&f.WithdrawalsCents, -1, func(x *tpPeriod) *int64 { return &x.WithdrawalsCents })
		case "carryover_out":
			add(&f.CarryoverOutCents, -1, func(x *tpPeriod) *int64 { return &x.CarryoverOutCents })
		case "adjustment":
			add(&f.AdjustmentsCents, 1, func(x *tpPeriod) *int64 { return &x.AdjustmentsCents })
		}
	}
	for _, x := range []*tpPeriod{&f.ThisMonth, &f.ThisYear} {
		x.NetCents = x.AllocationsCents + x.DepositsCents - x.WithdrawalsCents - x.CarryoverOutCents + x.AdjustmentsCents
	}
	return
}

func tpInvariant(f tpFlows) int64 {
	return f.OpeningBalanceCents + f.AllocationsCents + f.DepositsCents - f.WithdrawalsCents - f.CarryoverOutCents + f.AdjustmentsCents
}

func tpNoForbiddenKeys(t *testing.T, path string, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var walk func(any)
	walk = func(x any) {
		switch x := x.(type) {
		case map[string]any:
			for k, c := range x {
				switch strings.ToLower(k) {
				case "id", "potid", "periodid", "sourceperiodid", "description":
					t.Errorf("%s: forbidden key %q", path, k)
				}
				walk(c)
			}
		case []any:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(v)
	if strings.Contains(string(b), "secret note") {
		t.Errorf("%s leaks a ledger description", path)
	}
}

func TestIntegrationPots(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)

	ams, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	n := time.Now().In(ams)
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
	today := d(n.Year(), n.Month(), n.Day())
	earlyYear := d(n.Year(), time.January, 1)
	inJan := n.Month() == time.January
	lastYear := d(n.Year()-1, time.July, 1)

	// Periods: year 2100 is ours (open 2100-01 and 2100-04, closed 02/03), plus the real
	// current month, created only when absent.
	pJan := tpEnsurePeriod(t, pool, 2100, 1, "open")
	pFeb := tpEnsurePeriod(t, pool, 2100, 2, "closed")
	pMar := tpEnsurePeriod(t, pool, 2100, 3, "closed")
	pApr := tpEnsurePeriod(t, pool, 2100, 4, "open")
	pNow := tpEnsurePeriod(t, pool, n.Year(), int(n.Month()), "")

	// "Main" has every entry type. The last "today" row inserted is the last entry.
	main := []tpEntry{
		{"opening_balance", 10000, lastYear, nil, false, false},
		// period_id (2100-02) overrides the entry date: neither this month nor this year.
		{"allocation", 20000, today, &pFeb, false, false},
		// period_id = this month overrides an old entry date.
		{"allocation", 30000, lastYear, &pNow, true, true},
		{"deposit", 5000, today, nil, true, true},
		{"deposit", 1000, earlyYear, nil, inJan, true},
		{"withdrawal", -4000, today, nil, true, true},
		{"adjustment", -300, lastYear, nil, false, false},
		{"carryover_out", -2500, today, nil, true, true},
		{"adjustment", 100, today, nil, true, true},
	}
	mainName := "ZTest Main " + sfx
	mainSlug := "ztest-main-" + sfx
	tpInsertPot(t, pool, mainName, false, nil, nil, main)

	i64 := func(v int64) *int64 { return &v }
	in5 := d(n.Year(), n.Month()+5, 1)
	past := d(n.Year()-1, time.March, 1)
	goalName, goalSlug := "ZTest Goal "+sfx, "ztest-goal-"+sfx
	goalID := tpInsertPot(t, pool, goalName, false, i64(100000), &in5, []tpEntry{{"deposit", 40000, today, nil, true, true}})
	tpInsertSplit(t, pool, pApr, goalID, 25.5)
	tpInsertSplit(t, pool, pMar, goalID, 30)

	// Open period 2100-04 has no row for this pot: falls back to the latest period with a row.
	fallID := tpInsertPot(t, pool, "ZTest Fallback "+sfx, false, nil, nil, nil)
	tpInsertSplit(t, pool, pJan, fallID, 10)
	tpInsertSplit(t, pool, pMar, fallID, 12.5)

	tpInsertPot(t, pool, "ZTest Reached "+sfx, false, i64(5000), &past, []tpEntry{{"deposit", 6000, today, nil, true, true}})
	tpInsertPot(t, pool, "ZTest Past "+sfx, false, i64(10000), &past, []tpEntry{{"deposit", 2500, today, nil, true, true}})
	tpInsertPot(t, pool, "ZTest NoDate "+sfx, false, i64(3000), nil, []tpEntry{{"deposit", 1000, today, nil, true, true}})
	tpInsertPot(t, pool, "ZTest Empty "+sfx, false, nil, nil, nil)
	tpInsertPot(t, pool, "ZTest Arch "+sfx, true, nil, nil, []tpEntry{{"deposit", 99, today, nil, true, true}})

	tok := createTestToken(t, c, pool, "itest pots", []string{"pots:read"})
	get := func(path string, status int) []byte {
		return tkGet(t, srv.URL, "/api/integrations/v1/pots"+path, tok.Token, status)
	}
	detail := func(slug string) (tpDetail, []byte) {
		b := get("/"+slug, http.StatusOK)
		tpNoForbiddenKeys(t, slug, b)
		var det tpDetail
		if err := json.Unmarshal(b, &det); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339, det.GeneratedAt); err != nil {
			t.Errorf("generatedAt: %v", err)
		}
		return det, b
	}

	// ── Main: every entry type, bucketing, invariant ──
	det, _ := detail(mainSlug)
	wantBal, wantF := tpExpected(main)
	if det.Slug != mainSlug || det.Name != mainName || det.Kind != "normal" {
		t.Errorf("identity: %+v", det)
	}
	if det.BalanceCents != wantBal || det.Flows != wantF {
		t.Errorf("flows mismatch:\n got %d %+v\nwant %d %+v", det.BalanceCents, det.Flows, wantBal, wantF)
	}
	if wantBal != 59300 || tpInvariant(det.Flows) != det.BalanceCents {
		t.Errorf("fixture/invariant: want 59300, expected %d, invariant %d, got %d", wantBal, tpInvariant(det.Flows), det.BalanceCents)
	}
	if f := det.Flows; f.OpeningBalanceCents != 10000 || f.AllocationsCents != 50000 || f.DepositsCents != 6000 ||
		f.WithdrawalsCents != 4000 || f.CarryoverOutCents != 2500 || f.AdjustmentsCents != -200 {
		t.Errorf("lifetime flows: %+v", f)
	}
	// The 2100-02 allocation never shows in this month/year; the 'now' period one always does.
	if det.Flows.ThisYear.AllocationsCents != 30000 || det.Flows.ThisMonth.AllocationsCents != 30000 {
		t.Errorf("allocation bucketing: %+v", det.Flows)
	}
	if det.Target != nil || det.Split != nil {
		t.Errorf("Main should have no target or split: %+v %+v", det.Target, det.Split)
	}
	if l := det.LastEntry; l == nil || l.Date != today.Format("2006-01-02") || l.Type != "adjustment" || l.AmountCents != 100 {
		t.Errorf("lastEntry: %+v", l)
	}

	// ── Goal: target, progress, months, split in the open period ──
	gb, gbody := detail(goalSlug)
	t.Logf("GET /pots/%s:\n%s", goalSlug, gbody)
	wantMonths := 5
	if gb.Target == nil || gb.Target.TargetCents != 100000 || gb.Target.RemainingCents != 60000 || gb.Target.Reached ||
		gb.Target.ProgressPercent == nil || *gb.Target.ProgressPercent != 40 ||
		gb.Target.TargetDate == nil || *gb.Target.TargetDate != in5.Format("2006-01-02") ||
		gb.Target.MonthsLeft == nil || *gb.Target.MonthsLeft != wantMonths ||
		gb.Target.NeededPerMonthCents == nil || *gb.Target.NeededPerMonthCents != 12000 {
		t.Errorf("goal target: %+v", gb.Target)
	}
	if s := gb.Split; s == nil || s.Percentage != 25.5 || s.PeriodYear != 2100 || s.PeriodMonth != 4 || s.PeriodStatus != "open" {
		t.Errorf("goal split: %+v", s)
	}

	// ── split fallback ──
	fb, _ := detail("ztest-fallback-" + sfx)
	if s := fb.Split; s == nil || s.Percentage != 12.5 || s.PeriodYear != 2100 || s.PeriodMonth != 3 || s.PeriodStatus != "closed" {
		t.Errorf("fallback split: %+v", s)
	}
	if fb.Target != nil || fb.LastEntry != nil || fb.BalanceCents != 0 || fb.Flows != (tpFlows{}) {
		t.Errorf("fallback pot should be empty: %+v", fb)
	}

	// ── target edge cases ──
	re, _ := detail("ztest-reached-" + sfx)
	if tg := re.Target; tg == nil || !tg.Reached || tg.RemainingCents != 0 || *tg.ProgressPercent != 120 || *tg.MonthsLeft != 0 || *tg.NeededPerMonthCents != 0 {
		t.Errorf("reached: %+v", tg)
	}
	pa, _ := detail("ztest-past-" + sfx)
	if tg := pa.Target; tg == nil || tg.Reached || tg.RemainingCents != 7500 || *tg.ProgressPercent != 25 || *tg.MonthsLeft != 0 || *tg.NeededPerMonthCents != 7500 {
		t.Errorf("past date: %+v", tg)
	}
	nd, _ := detail("ztest-nodate-" + sfx)
	if tg := nd.Target; tg == nil || tg.TargetDate != nil || tg.MonthsLeft != nil || tg.NeededPerMonthCents != nil ||
		tg.RemainingCents != 2000 || *tg.ProgressPercent != 33.3 {
		t.Errorf("no date: %+v", tg)
	}
	em, _ := detail("ztest-empty-" + sfx)
	if em.BalanceCents != 0 || em.LastEntry != nil || em.Split != nil || em.Target != nil {
		t.Errorf("empty: %+v", em)
	}

	// ── carryover pot (shared with the rest of the suite; assert only relative facts) ──
	carryoverPotID(t, pool)
	lb := get("", http.StatusOK)
	tpNoForbiddenKeys(t, "list", lb)
	var list struct {
		Pots []struct {
			Slug, Name, Kind string
			BalanceCents     int64
			TargetCents      *int64
			ProgressPercent  *float64
		}
		Totals      struct{ BalanceCents, TargetCents int64 }
		GeneratedAt string
	}
	if err := json.Unmarshal(lb, &list); err != nil {
		t.Fatal(err)
	}
	var sumBal, sumTarget int64
	seen := map[string]bool{}
	found := 0
	carryoverSlug := ""
	for _, p := range list.Pots {
		sumBal += p.BalanceCents
		if p.TargetCents != nil {
			sumTarget += *p.TargetCents
		} else if p.ProgressPercent != nil {
			t.Errorf("%s: progress without target", p.Slug)
		}
		if seen[p.Slug] {
			t.Errorf("duplicate slug %q", p.Slug)
		}
		seen[p.Slug] = true
		if p.Kind == "carryover" {
			carryoverSlug = p.Slug
		}
		switch p.Slug {
		case mainSlug:
			found++
			if p.BalanceCents != wantBal || p.TargetCents != nil {
				t.Errorf("list main: %+v", p)
			}
		case goalSlug:
			found++
			if p.BalanceCents != 40000 || p.TargetCents == nil || *p.TargetCents != 100000 || p.ProgressPercent == nil || *p.ProgressPercent != 40 {
				t.Errorf("list goal: %+v", p)
			}
		case "ztest-fallback-" + sfx, "ztest-reached-" + sfx, "ztest-past-" + sfx, "ztest-nodate-" + sfx, "ztest-empty-" + sfx:
			found++
		case "ztest-arch-" + sfx:
			t.Error("archived pot listed")
		}
	}
	if found != 7 {
		t.Errorf("found %d of 7 fixture pots", found)
	}
	if list.Totals.BalanceCents != sumBal || list.Totals.TargetCents != sumTarget {
		t.Errorf("totals %+v != sums %d / %d", list.Totals, sumBal, sumTarget)
	}
	if carryoverSlug == "" {
		t.Fatal("no carryover pot in list")
	}
	cd, _ := detail(carryoverSlug)
	if cd.Kind != "carryover" || tpInvariant(cd.Flows) != cd.BalanceCents {
		t.Errorf("carryover pot: kind %s, invariant %d vs %d", cd.Kind, tpInvariant(cd.Flows), cd.BalanceCents)
	}

	// ── 404s ──
	for _, p := range []string{"nope-" + sfx, "ztest-arch-" + sfx, "ZTest", "bad_slug", mainSlug + "/other", strings.Repeat("a", 65)} {
		b := get("/"+p, http.StatusNotFound)
		if !strings.Contains(string(b), `"code":"not_found"`) {
			t.Errorf("%s: body %s", p, b)
		}
	}

	// ── scope and auth ──
	raw := "hf_potsscope" + sfx
	var tid int64
	if err := pool.QueryRow(ctx, `INSERT INTO integration_tokens (name, token_hash, token_prefix, scopes, expires_at) VALUES ('itest pots noscope', sha256(convert_to($1,'UTF8')), 'hf_potss', '{kids:read}', now() + interval '1 day') RETURNING id`, raw).Scan(&tid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM integration_tokens WHERE id=$1`, tid) })
	for _, p := range []string{"", "/" + mainSlug} {
		b := tkGet(t, srv.URL, "/api/integrations/v1/pots"+p, raw, http.StatusForbidden)
		if !strings.Contains(string(b), "insufficient_scope") {
			t.Errorf("%s: %s", p, b)
		}
		tkGet(t, srv.URL, "/api/integrations/v1/pots"+p, "", http.StatusUnauthorized)
	}
	// A kids-only token cannot read pots, and a pots token cannot read kids.
	tkGet(t, srv.URL, "/api/integrations/v1/kids", tok.Token, http.StatusForbidden)
}
