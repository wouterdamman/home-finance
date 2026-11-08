//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type tkPeriod struct{ DepositsCents, WithdrawalsCents, AdjustmentsCents, NetCents int64 }

type tkFlows struct {
	OpeningBalanceCents, DepositsCents, WithdrawalsCents, AdjustmentsCents int64
	ThisMonth, ThisYear                                                    tkPeriod
}

type tkLast struct {
	Date, Type, Owner string
	AmountCents       int64
}

type tkPot struct {
	Owner        string
	BalanceCents int64
	tkFlows
	LastEntry *tkLast
}

type tkDetail struct {
	Slug, Name string
	TotalCents int64
	Combined   tkFlows
	Pots       map[string]tkPot
	Bank       struct {
		ReportedBalanceCents *int64
		ReportedBalanceDate  *string
		DifferenceCents      *int64
	}
	LastEntry   *tkLast
	GeneratedAt string
}

type tkEntry struct {
	owner, typ    string
	cents         int64 // signed as stored
	date          time.Time
	inMonth, inYr bool
}

func tkInsertKid(t *testing.T, pool *pgxpool.Pool, name string, archived bool) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO kids (name, sort_order, archived_at) VALUES ($1, 1000, CASE WHEN $2 THEN now() END) RETURNING id`, name, archived).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM kid_savings_ledger WHERE kid_id=$1`, id)
		pool.Exec(ctx, `DELETE FROM kids WHERE id=$1`, id)
	})
	return id
}

func tkInsertEntries(t *testing.T, pool *pgxpool.Pool, kid int64, es []tkEntry) {
	t.Helper()
	for _, e := range es {
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO kid_savings_ledger (kid_id, owner, entry_type, amount_cents, description, entry_date) VALUES ($1,$2,$3,$4,'secret note',$5)`,
			kid, e.owner, e.typ, e.cents, e.date.Format("2006-01-02")); err != nil {
			t.Fatal(err)
		}
	}
}

// tkExpected sums the fixture independently of the SQL under test.
func tkExpected(es []tkEntry, owner string) (bal int64, f tkFlows) {
	for _, e := range es {
		if owner != "" && e.owner != owner {
			continue
		}
		bal += e.cents
		var p []*tkPeriod
		if e.inMonth {
			p = append(p, &f.ThisMonth)
		}
		if e.inYr {
			p = append(p, &f.ThisYear)
		}
		switch e.typ {
		case "opening_balance":
			f.OpeningBalanceCents += e.cents
		case "deposit":
			f.DepositsCents += e.cents
			for _, x := range p {
				x.DepositsCents += e.cents
			}
		case "withdrawal":
			f.WithdrawalsCents -= e.cents
			for _, x := range p {
				x.WithdrawalsCents -= e.cents
			}
		case "adjustment":
			f.AdjustmentsCents += e.cents
			for _, x := range p {
				x.AdjustmentsCents += e.cents
			}
		}
	}
	for _, x := range []*tkPeriod{&f.ThisMonth, &f.ThisYear} {
		x.NetCents = x.DepositsCents - x.WithdrawalsCents + x.AdjustmentsCents
	}
	return
}

func tkGet(t *testing.T, srv, path, tok string, wantStatus int) []byte {
	t.Helper()
	resp := bearerGet(t, srv, http.MethodGet, path, tok)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: want %d, got %d: %s", path, wantStatus, resp.StatusCode, b)
	}
	return b
}

func tkNoForbiddenKeys(t *testing.T, path string, b []byte) {
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
				case "id", "kidid", "description":
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

func TestIntegrationKidsDetail(t *testing.T) {
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
	earlyYear := d(n.Year(), time.January, 1) // same month as today when it is January
	inJan := n.Month() == time.January
	lastYear := d(n.Year()-1, time.July, 1)

	// Insert order matters for "last entry": same-date ties go to the higher id.
	zoe := []tkEntry{
		{"ours", "opening_balance", 10000, lastYear, false, false},
		{"ours", "adjustment", -300, lastYear, false, false},
		{"ours", "deposit", 2000, earlyYear, inJan, true},
		{"ours", "deposit", 5000, today, true, true},
		{"ours", "withdrawal", -1500, today, true, true},
		{"ours", "adjustment", 100, today, true, true},
		{"theirs", "opening_balance", 2000, lastYear, false, false},
		{"theirs", "withdrawal", -200, lastYear, false, false},
		{"theirs", "deposit", 700, today, true, true},
	}
	zoeName := "Zoë " + sfx
	zoeID := tkInsertKid(t, pool, zoeName, false)
	tkInsertEntries(t, pool, zoeID, zoe)
	if _, err := pool.Exec(ctx, `UPDATE kids SET reported_balance_cents=17300, reported_balance_date='2026-01-15' WHERE id=$1`, zoeID); err != nil {
		t.Fatal(err)
	}
	zoeSlug := "zoe-" + sfx

	// Two kids with the same name; one only has an "ours" pot.
	twinName := "Twin " + sfx
	twinA := tkInsertKid(t, pool, twinName, false)
	twinB := tkInsertKid(t, pool, twinName, false)
	tkInsertEntries(t, pool, twinA, []tkEntry{{"ours", "deposit", 1234, today, true, true}})
	tkInsertEntries(t, pool, twinB, []tkEntry{{"theirs", "deposit", 4321, today, true, true}})
	// Reserved and symbol-only names, an archived kid, and an empty kid.
	balID := tkInsertKid(t, pool, "Balances", false)
	symID := tkInsertKid(t, pool, "!?! "+sfx[:0], false)
	archID := tkInsertKid(t, pool, "Arch "+sfx, true)
	tkInsertEntries(t, pool, archID, []tkEntry{{"ours", "deposit", 99, today, true, true}})

	tok := createTestToken(t, c, pool, "itest kids detail", []string{"kids:read"})

	// ── detail ──
	body := tkGet(t, srv.URL, "/api/integrations/v1/kids/"+zoeSlug, tok.Token, http.StatusOK)
	tkNoForbiddenKeys(t, "detail", body)
	t.Logf("GET /kids/%s:\n%s", zoeSlug, body)
	var det tkDetail
	if err := json.Unmarshal(body, &det); err != nil {
		t.Fatal(err)
	}
	if det.Slug != zoeSlug || det.Name != zoeName {
		t.Errorf("slug/name: %q %q", det.Slug, det.Name)
	}
	oursBal, oursF := tkExpected(zoe, "ours")
	theirsBal, theirsF := tkExpected(zoe, "theirs")
	allBal, allF := tkExpected(zoe, "")
	if oursBal != 15300 || theirsBal != 2500 || allBal != 17800 {
		t.Fatalf("fixture sanity: %d %d %d", oursBal, theirsBal, allBal)
	}
	if det.TotalCents != 17800 || det.Pots["ours"].BalanceCents != 15300 || det.Pots["theirs"].BalanceCents != 2500 {
		t.Errorf("balances: %+v", det)
	}
	if det.Pots["ours"].tkFlows != oursF || det.Pots["theirs"].tkFlows != theirsF || det.Combined != allF {
		t.Errorf("flows mismatch:\n got ours   %+v\nwant ours   %+v\n got theirs %+v\nwant theirs %+v\n got comb   %+v\nwant comb   %+v",
			det.Pots["ours"].tkFlows, oursF, det.Pots["theirs"].tkFlows, theirsF, det.Combined, allF)
	}
	// Hard-coded lifetime numbers.
	if f := det.Pots["ours"]; f.OpeningBalanceCents != 10000 || f.DepositsCents != 7000 || f.WithdrawalsCents != 1500 || f.AdjustmentsCents != -200 {
		t.Errorf("ours lifetime flows: %+v", f.tkFlows)
	}
	if f := det.Pots["theirs"]; f.OpeningBalanceCents != 2000 || f.DepositsCents != 700 || f.WithdrawalsCents != 200 || f.AdjustmentsCents != 0 {
		t.Errorf("theirs lifetime flows: %+v", f.tkFlows)
	}
	// Opening balance never shows in periods; last year never shows in them either.
	if det.Pots["ours"].ThisYear.DepositsCents != 7000 || det.Pots["ours"].ThisYear.AdjustmentsCents != 100 {
		t.Errorf("ours thisYear: %+v", det.Pots["ours"].ThisYear)
	}
	if wantM := int64(5000); !inJan && det.Pots["ours"].ThisMonth.DepositsCents != wantM {
		t.Errorf("ours thisMonth deposits: %+v", det.Pots["ours"].ThisMonth)
	}
	for owner, p := range det.Pots {
		if p.Owner != owner {
			t.Errorf("owner %q in pot %q", p.Owner, owner)
		}
		if got := p.OpeningBalanceCents + p.DepositsCents - p.WithdrawalsCents + p.AdjustmentsCents; got != p.BalanceCents {
			t.Errorf("%s invariant: %d != %d", owner, got, p.BalanceCents)
		}
	}
	if got := det.Combined.OpeningBalanceCents + det.Combined.DepositsCents - det.Combined.WithdrawalsCents + det.Combined.AdjustmentsCents; got != det.TotalCents {
		t.Errorf("combined invariant: %d != %d", got, det.TotalCents)
	}
	if det.Bank.ReportedBalanceCents == nil || *det.Bank.ReportedBalanceCents != 17300 ||
		det.Bank.ReportedBalanceDate == nil || *det.Bank.ReportedBalanceDate != "2026-01-15" ||
		det.Bank.DifferenceCents == nil || *det.Bank.DifferenceCents != 500 {
		t.Errorf("bank: %+v", det.Bank)
	}
	if l := det.Pots["ours"].LastEntry; l == nil || *l != (tkLast{today.Format("2006-01-02"), "adjustment", "ours", 100}) {
		t.Errorf("ours lastEntry: %+v", l)
	}
	if l := det.Pots["theirs"].LastEntry; l == nil || *l != (tkLast{today.Format("2006-01-02"), "deposit", "theirs", 700}) {
		t.Errorf("theirs lastEntry: %+v", l)
	}
	if l := det.LastEntry; l == nil || *l != (tkLast{today.Format("2006-01-02"), "deposit", "theirs", 700}) {
		t.Errorf("combined lastEntry: %+v", l)
	}
	if _, err := time.Parse(time.RFC3339, det.GeneratedAt); err != nil {
		t.Errorf("generatedAt: %v", err)
	}

	// ── sub-pot endpoints ──
	for owner, want := range map[string]struct {
		bal int64
		f   tkFlows
	}{"ours": {oursBal, oursF}, "theirs": {theirsBal, theirsF}} {
		path := "/api/integrations/v1/kids/" + zoeSlug + "/" + owner
		b := tkGet(t, srv.URL, path, tok.Token, http.StatusOK)
		tkNoForbiddenKeys(t, path, b)
		if owner == "ours" {
			t.Logf("GET %s:\n%s", path, b)
		}
		var sp struct {
			Slug, Name  string
			GeneratedAt string
			tkPot
		}
		if err := json.Unmarshal(b, &sp); err != nil {
			t.Fatal(err)
		}
		if sp.Slug != zoeSlug || sp.Name != zoeName || sp.Owner != owner || sp.BalanceCents != want.bal || sp.tkFlows != want.f || sp.LastEntry == nil || sp.GeneratedAt == "" {
			t.Errorf("%s: %+v", path, sp)
		}
	}

	// ── twins: both get the id suffix; sub-pot without entries is zeros ──
	twinSlugA := "twin-" + sfx + "-" + strconv.FormatInt(twinA, 10)
	twinSlugB := "twin-" + sfx + "-" + strconv.FormatInt(twinB, 10)
	tkGet(t, srv.URL, "/api/integrations/v1/kids/twin-"+sfx, tok.Token, http.StatusNotFound)
	var ta tkDetail
	if err := json.Unmarshal(tkGet(t, srv.URL, "/api/integrations/v1/kids/"+twinSlugA, tok.Token, http.StatusOK), &ta); err != nil {
		t.Fatal(err)
	}
	if ta.TotalCents != 1234 || ta.Pots["theirs"].BalanceCents != 0 || ta.Pots["theirs"].LastEntry != nil ||
		ta.Pots["theirs"].tkFlows != (tkFlows{}) || ta.Bank.DifferenceCents != nil || ta.Bank.ReportedBalanceCents != nil {
		t.Errorf("twin A: %+v", ta)
	}
	if b := tkGet(t, srv.URL, "/api/integrations/v1/kids/"+twinSlugB+"/theirs", tok.Token, http.StatusOK); !strings.Contains(string(b), `"balanceCents":4321`) {
		t.Errorf("twin B theirs: %s", b)
	}

	// ── reserved / symbol-only / empty ──
	balSlug := "balances-" + strconv.FormatInt(balID, 10)
	tkGet(t, srv.URL, "/api/integrations/v1/kids/"+balSlug, tok.Token, http.StatusOK)
	symSlug := "kid-" + strconv.FormatInt(symID, 10)
	var sd tkDetail
	if err := json.Unmarshal(tkGet(t, srv.URL, "/api/integrations/v1/kids/"+symSlug, tok.Token, http.StatusOK), &sd); err != nil {
		t.Fatal(err)
	}
	if sd.TotalCents != 0 || sd.LastEntry != nil || sd.Pots["ours"].LastEntry != nil {
		t.Errorf("empty kid: %+v", sd)
	}
	// "balances" itself is still the old endpoint.
	if b := tkGet(t, srv.URL, "/api/integrations/v1/kids/balances", tok.Token, http.StatusOK); strings.Contains(string(b), `"slug"`) || strings.Contains(string(b), `"totals"`) {
		t.Errorf("/kids/balances changed shape: %s", b)
	}

	// ── 404s ──
	for _, p := range []string{"nope-" + sfx, "arch-" + sfx, "Zoe", "bad_slug", zoeSlug + "/other", strings.Repeat("a", 65)} {
		b := tkGet(t, srv.URL, "/api/integrations/v1/kids/"+p, tok.Token, http.StatusNotFound)
		if !strings.Contains(string(b), `"code":"not_found"`) {
			t.Errorf("%s: body %s", p, b)
		}
	}
	tkGet(t, srv.URL, "/api/integrations/v1/kids/arch-"+sfx+"/ours", tok.Token, http.StatusNotFound)

	// ── list ──
	lb := tkGet(t, srv.URL, "/api/integrations/v1/kids", tok.Token, http.StatusOK)
	tkNoForbiddenKeys(t, "list", lb)
	var list struct {
		Kids []struct {
			Slug, Name                         string
			TotalCents, OursCents, TheirsCents int64
		}
		Totals struct{ TotalCents, OursCents, TheirsCents int64 }
	}
	if err := json.Unmarshal(lb, &list); err != nil {
		t.Fatal(err)
	}
	var sum [3]int64
	seen := map[string]bool{}
	found := 0
	for _, k := range list.Kids {
		sum[0] += k.TotalCents
		sum[1] += k.OursCents
		sum[2] += k.TheirsCents
		if seen[k.Slug] {
			t.Errorf("duplicate slug %q", k.Slug)
		}
		seen[k.Slug] = true
		if k.TotalCents != k.OursCents+k.TheirsCents {
			t.Errorf("kid %s total != ours+theirs", k.Slug)
		}
		switch k.Slug {
		case zoeSlug:
			found++
			if k.TotalCents != 17800 || k.OursCents != 15300 || k.TheirsCents != 2500 {
				t.Errorf("list zoe: %+v", k)
			}
		case twinSlugA, twinSlugB, balSlug, symSlug:
			found++
		case "arch-" + sfx:
			t.Error("archived kid listed")
		}
	}
	if found != 5 {
		t.Errorf("found %d of 5 fixture kids in list", found)
	}
	if sum != [3]int64{list.Totals.TotalCents, list.Totals.OursCents, list.Totals.TheirsCents} {
		t.Errorf("totals %+v != sum %v", list.Totals, sum)
	}

	// ── scope ──
	raw := "hf_kidsscope" + sfx
	var tid int64
	if err := pool.QueryRow(ctx, `INSERT INTO integration_tokens (name, token_hash, token_prefix, scopes, expires_at) VALUES ('itest kids noscope', sha256(convert_to($1,'UTF8')), 'hf_kidss', '{pots:read}', now() + interval '1 day') RETURNING id`, raw).Scan(&tid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM integration_tokens WHERE id=$1`, tid) })
	for _, p := range []string{"", "/" + zoeSlug, "/" + zoeSlug + "/ours", "/" + zoeSlug + "/theirs"} {
		b := tkGet(t, srv.URL, "/api/integrations/v1/kids"+p, raw, http.StatusForbidden)
		if !strings.Contains(string(b), "insufficient_scope") {
			t.Errorf("%s: %s", p, b)
		}
		tkGet(t, srv.URL, "/api/integrations/v1/kids"+p, "", http.StatusUnauthorized)
	}
}
