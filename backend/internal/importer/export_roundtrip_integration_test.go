//go:build integration

// This test lives in the external test package so it can import httpapi (which
// imports importer) and drive the *real* export writer, rather than a fixture
// that only claims to match it.
package importer_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wouterdamman/home-finance/internal/auth"
	"github.com/wouterdamman/home-finance/internal/config"
	"github.com/wouterdamman/home-finance/internal/httpapi"
	"github.com/wouterdamman/home-finance/internal/importer"
)

// TestExportImportRoundTrip writes a period through the Excel export, feeds the
// bytes straight back through DetectAndParse + Run, and asserts nothing moved:
// same income/expense totals, same row counts, no duplicated rows on a second
// import of the same file.
func TestExportImportRoundTrip(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const (
		year        = 2093
		month       = 7
		plainSource = "ZTest Salaris 2093"
		itemSource  = "ZTest Freelance 2093"
		trackedCat  = "ZTest Boodschappen 2093"
		fixedCat    = "ZTest Huur 2093"
	)

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name IN ($1,$2)`, plainSource, itemSource)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1,$2)`, trackedCat, fixedCat)
	})
	pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}
	scanID := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
		return id
	}

	exec(`INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, year)
	periodID := scanID(`INSERT INTO periods (year, month) VALUES ($1,$2) RETURNING id`, year, month)
	plainID := scanID(`INSERT INTO income_sources (name, default_amount_cents, sort_order) VALUES ($1,0,0) RETURNING id`, plainSource)
	itemID := scanID(`INSERT INTO income_sources (name, default_amount_cents, is_itemized, sort_order) VALUES ($1,0,true,1) RETURNING id`, itemSource)
	trackedID := scanID(`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1,0,true,0) RETURNING id`, trackedCat)
	fixedID := scanID(`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1,0,false,1) RETURNING id`, fixedCat)

	exec(`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, sort_order) VALUES ($1,$2,$3,250000,'normal',0)`, periodID, plainID, plainSource)
	exec(`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, sort_order) VALUES ($1,$2,$3,0,'normal',1)`, periodID, itemID, itemSource)
	exec(`INSERT INTO income_transactions (period_id, source_id, amount_cents, description, tx_date) VALUES ($1,$2,30000,'Klus A','2093-07-05')`, periodID, itemID)
	exec(`INSERT INTO income_transactions (period_id, source_id, amount_cents, description, tx_date) VALUES ($1,$2,15000,'Klus B',NULL)`, periodID, itemID)

	exec(`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order) VALUES ($1,$2,$3,40000,true,0)`, periodID, trackedID, trackedCat)
	exec(`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order) VALUES ($1,$2,$3,100000,false,1)`, periodID, fixedID, fixedCat)
	exec(`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date) VALUES ($1,$2,25000,'AH','2093-07-03')`, periodID, trackedID)
	exec(`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date) VALUES ($1,$2,17000,'Zonder datum',NULL)`, periodID, trackedID)
	exec(`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date) VALUES ($1,$2,-5000,'Retour','2093-07-10')`, periodID, trackedID)

	const wantIncome int64 = 295000  // 250000 + (30000 + 15000) itemized
	const wantExpense int64 = 137000 // (25000 + 17000 - 5000) tracked + 100000 fixed

	// ── Export through the real handler ─────────────────────────────
	cfg := &config.Config{DevFakeAuth: true, Env: "development", SessionSecure: false}
	sm := auth.NewSessionManager(pool, false, auth.DefaultSessionLifetime, auth.DefaultSessionIdleTimeout)
	srv := httptest.NewServer(httpapi.NewServer(cfg, pool, sm, nil))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/export/years/2093?months=7", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("export: status %d, body %s", resp.StatusCode, body)
	}
	path := t.TempDir() + "/export.xlsx"
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp export: %v", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		t.Fatalf("save export: %v", err)
	}
	out.Close()

	// ── Feed it straight back in ────────────────────────────────────
	countRows := func() (incomeEntries, incomeTxs, budgetLines, txs int) {
		t.Helper()
		q := func(sql string) int {
			var n int
			if err := pool.QueryRow(ctx, sql, periodID).Scan(&n); err != nil {
				t.Fatalf("count: %v", err)
			}
			return n
		}
		return q(`SELECT count(*) FROM income_entries WHERE period_id=$1`),
			q(`SELECT count(*) FROM income_transactions WHERE period_id=$1`),
			q(`SELECT count(*) FROM budget_lines WHERE period_id=$1`),
			q(`SELECT count(*) FROM transactions WHERE period_id=$1`)
	}
	wantIE, wantITx, wantBL, wantTx := countRows()

	for pass := 1; pass <= 2; pass++ {
		sheets, skipped, err := importer.DetectAndParse(path)
		if err != nil {
			t.Fatalf("pass %d: DetectAndParse: %v", pass, err)
		}
		if len(sheets) == 0 {
			t.Fatalf("pass %d: no sheets parsed (skipped: %v)", pass, skipped)
		}
		rep, err := importer.Run(ctx, pool, sheets, importer.ImportOptions{Year: year})
		if err != nil {
			t.Fatalf("pass %d: Run: %v", pass, err)
		}
		if len(rep.Problems) != 0 {
			t.Errorf("pass %d: problems: want none, got %v", pass, rep.Problems)
		}
		if len(rep.Months) != 1 {
			t.Fatalf("pass %d: months: want 1, got %d", pass, len(rep.Months))
		}
		mr := rep.Months[0]
		if mr.IncomeTotalCents != wantIncome {
			t.Errorf("pass %d: income total: want %d, got %d", pass, wantIncome, mr.IncomeTotalCents)
		}
		if mr.ExpenseTotalCents != wantExpense {
			t.Errorf("pass %d: expense total: want %d, got %d", pass, wantExpense, mr.ExpenseTotalCents)
		}

		gotIE, gotITx, gotBL, gotTx := countRows()
		if gotIE != wantIE || gotITx != wantITx || gotBL != wantBL || gotTx != wantTx {
			t.Fatalf("pass %d: row counts changed: income_entries %d→%d, income_transactions %d→%d, budget_lines %d→%d, transactions %d→%d",
				pass, wantIE, gotIE, wantITx, gotITx, wantBL, gotBL, wantTx, gotTx)
		}
	}

	// The reimported itemized source must still be itemized and still carry its
	// line items — collapsing it to a summed header row valued it at 0.
	var itemized bool
	if err := pool.QueryRow(ctx, `SELECT is_itemized FROM income_sources WHERE id=$1`, itemID).Scan(&itemized); err != nil {
		t.Fatalf("read is_itemized: %v", err)
	}
	if !itemized {
		t.Error("itemized income source lost its is_itemized flag on reimport")
	}
}

// exportPeriodToFile drives the real export handler for one month and returns
// the path of the written workbook.
func exportPeriodToFile(t *testing.T, pool *pgxpool.Pool, year, month int) string {
	t.Helper()
	cfg := &config.Config{DevFakeAuth: true, Env: "development", SessionSecure: false}
	sm := auth.NewSessionManager(pool, false, auth.DefaultSessionLifetime, auth.DefaultSessionIdleTimeout)
	srv := httptest.NewServer(httpapi.NewServer(cfg, pool, sm, nil))
	t.Cleanup(srv.Close)

	url := fmt.Sprintf("%s/api/export/years/%d?months=%d", srv.URL, year, month)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("export request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("export: status %d, body %s", resp.StatusCode, body)
	}
	path := t.TempDir() + "/export.xlsx"
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp export: %v", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		t.Fatalf("save export: %v", err)
	}
	out.Close()
	return path
}

// TestRoundTripKeepsUntrackedLineUntracked: the exported Transacties section
// carries every transaction in the period, including ones sitting under a line
// whose tracks_transactions is false (legitimate — the toggle is deliberately
// per-period). The importer used to set tracks_transactions from a label match
// against that section, so such a line flipped to tracked on reimport and its
// effective amount switched from amount_cents to the transaction sum: 350 euro
// became 310 while the workbook's own total cell still said 350.
func TestRoundTripKeepsUntrackedLineUntracked(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const (
		year       = 2062
		month      = 5
		trackedCat = "ZTest Boodschappen 2062"
		staleCat   = "ZTest Onderhoud 2062"
	)

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1,$2)`, trackedCat, staleCat)
	})
	pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}
	scanID := func(sql string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&id); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
		return id
	}

	exec(`INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, year)
	periodID := scanID(`INSERT INTO periods (year, month) VALUES ($1,$2) RETURNING id`, year, month)
	trackedID := scanID(`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1,0,true,0) RETURNING id`, trackedCat)
	staleID := scanID(`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1,0,false,1) RETURNING id`, staleCat)

	exec(`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order) VALUES ($1,$2,$3,20000,true,0)`, periodID, trackedID, trackedCat)
	exec(`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date) VALUES ($1,$2,20000,'AH','2062-05-04')`, periodID, trackedID)

	// Untracked line: its 35000 amount is what counts, and the 31000 of stale
	// transactions under it must stay inert on both sides of the round trip.
	exec(`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order) VALUES ($1,$2,$3,35000,false,1)`, periodID, staleID, staleCat)
	exec(`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date) VALUES ($1,$2,31000,'Oude boeking','2062-05-09')`, periodID, staleID)

	const wantExpense int64 = 55000 // 20000 tracked + 35000 untracked amount

	path := exportPeriodToFile(t, pool, year, month)

	sheets, skipped, err := importer.DetectAndParse(path)
	if err != nil {
		t.Fatalf("DetectAndParse: %v", err)
	}
	if len(sheets) == 0 {
		t.Fatalf("no sheets parsed (skipped: %v)", skipped)
	}
	rep, err := importer.Run(ctx, pool, sheets, importer.ImportOptions{Year: year})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Months) != 1 {
		t.Fatalf("months: want 1, got %d", len(rep.Months))
	}
	if got := rep.Months[0].ExpenseTotalCents; got != wantExpense {
		t.Errorf("expense total after round trip: want %d, got %d", wantExpense, got)
	}

	var tracks bool
	var amount int64
	if err := pool.QueryRow(ctx,
		`SELECT tracks_transactions, amount_cents FROM budget_lines WHERE period_id=$1 AND category_id=$2`,
		periodID, staleID).Scan(&tracks, &amount); err != nil {
		t.Fatalf("read untracked budget line: %v", err)
	}
	if tracks {
		t.Error("untracked budget line flipped to tracks_transactions on reimport")
	}
	if amount != 35000 {
		t.Errorf("untracked budget line amount: want 35000, got %d", amount)
	}

	if err := pool.QueryRow(ctx,
		`SELECT tracks_transactions FROM budget_lines WHERE period_id=$1 AND category_id=$2`,
		periodID, trackedID).Scan(&tracks); err != nil {
		t.Fatalf("read tracked budget line: %v", err)
	}
	if !tracks {
		t.Error("tracked budget line lost tracks_transactions on reimport")
	}
}
