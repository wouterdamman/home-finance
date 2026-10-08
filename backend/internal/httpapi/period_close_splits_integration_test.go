//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tests in this file cover the pot-split side of the period close: the
// surplus must never be able to disappear, and the pot it lands in must be
// the pot every other screen says it will be. They use test years in the
// 2070s (periods.year is CHECK-constrained to 2000-2100), create their own
// pots, and wipe everything they touch in t.Cleanup — the household's own
// pots are deliberately never used, so a concurrently archived or
// re-percentaged pot in a shared development database cannot flip a result.

// wipeTestYear removes every period (and everything cascading off it) in a
// test year, including the next-period rows a close creates on its own.
func wipeTestYear(t *testing.T, pool *pgxpool.Pool, years ...int) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		for _, y := range years {
			if _, err := pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, y); err != nil {
				t.Errorf("cleanup periods %d: %v", y, err)
			}
			if _, err := pool.Exec(ctx, `DELETE FROM years WHERE year=$1`, y); err != nil {
				t.Errorf("cleanup years %d: %v", y, err)
			}
			if _, err := pool.Exec(ctx, `DELETE FROM locked_years WHERE year=$1`, y); err != nil {
				t.Errorf("cleanup locked_years %d: %v", y, err)
			}
		}
	})
}

// carryoverPotID returns the household's carryover pot. A partial index
// (pots_one_carryover_idx) allows exactly one unarchived carryover pot, so a
// test that needs the carryover leg of a close has to use this one — it
// cannot create its own. Its ledger rows hang off the test period and
// cascade away with it.
func carryoverPotID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM pots WHERE kind='carryover' AND archived_at IS NULL`).Scan(&id); err != nil {
		t.Fatalf("carryover pot: %v", err)
	}
	return id
}

// createTestPot inserts a pot directly (there is no delete endpoint, so the
// cleanup is raw SQL too) and returns its id.
func createTestPot(t *testing.T, pool *pgxpool.Pool, name, kind string, sortOrder int) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO pots (name,kind,sort_order) VALUES ($1,$2,$3) RETURNING id`,
		name, kind, sortOrder).Scan(&id); err != nil {
		t.Fatalf("create pot %s: %v", name, err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM pot_ledger WHERE pot_id=$1`, id)
		pool.Exec(ctx, `DELETE FROM pot_splits WHERE pot_id=$1`, id)
		if _, err := pool.Exec(ctx, `DELETE FROM pots WHERE id=$1`, id); err != nil {
			t.Errorf("cleanup pot %d: %v", id, err)
		}
	})
	return id
}

// insertSplit writes a split row directly. The HTTP endpoint always tops the
// total up to 100% through the household's carryover pot, which these tests
// must not depend on — and writing the row directly is also the only way to
// reach the states an import or an older release can leave behind.
func insertSplit(t *testing.T, pool *pgxpool.Pool, periodID, potID int64, percentage string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO pot_splits (period_id,pot_id,percentage) VALUES ($1,$2,$3::numeric)`,
		periodID, potID, percentage); err != nil {
		t.Fatalf("insert split: %v", err)
	}
}

func addIncome(t *testing.T, c *apiClient, periodID int64, label string, cents int64) {
	t.Helper()
	resp := c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/incomes", map[string]any{
		"label": label, "amountCents": cents, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income: want 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func closePeriod(t *testing.T, c *apiClient, periodID int64) *http.Response {
	t.Helper()
	return c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/close", nil)
}

func errorCode(t *testing.T, c *apiClient, resp *http.Response) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	c.decode(resp, &body)
	return body.Error.Code
}

func allocatedCents(t *testing.T, pool *pgxpool.Pool, periodID, potID int64) int64 {
	t.Helper()
	var cents int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount_cents),0) FROM pot_ledger
		 WHERE period_id=$1 AND pot_id=$2 AND entry_type='allocation'`, periodID, potID).Scan(&cents); err != nil {
		t.Fatalf("allocated cents: %v", err)
	}
	return cents
}

func periodStatus(t *testing.T, pool *pgxpool.Pool, periodID int64) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM periods WHERE id=$1`, periodID).Scan(&status); err != nil {
		t.Fatalf("read period status: %v", err)
	}
	return status
}

// TestClosePeriodRejectsSurplusWithoutValidSplits pins the guard that keeps a
// close from silently swallowing the surplus: domain.LargestRemainderSplit
// returns nothing for an empty or non-100% split set, so the allocation loop
// never runs and the money is gone with a 204.
func TestClosePeriodRejectsSurplusWithoutValidSplits(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	t.Run("no splits at all", func(t *testing.T) {
		const year, month = 2071, 5
		wipeTestYear(t, pool, year)
		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 500000)

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("close without splits: want 409, got %d", resp.StatusCode)
		}
		if code := errorCode(t, c, resp); code != "no_pot_splits" {
			t.Errorf("error code: want no_pot_splits, got %q", code)
		}
		if got := periodStatus(t, pool, id); got != "open" {
			t.Errorf("status after rejected close: want open, got %s", got)
		}
	})

	t.Run("splits that do not total 100", func(t *testing.T) {
		const year, month = 2071, 7
		wipeTestYear(t, pool, year)
		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 500000)
		potID := createTestPot(t, pool, "ZTest Partial Split Pot", "normal", 900)
		insertSplit(t, pool, id, potID, "50")

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("close with 50%% of splits: want 409, got %d", resp.StatusCode)
		}
		if code := errorCode(t, c, resp); code != "split_percentage_not_100" {
			t.Errorf("error code: want split_percentage_not_100, got %q", code)
		}
		if got := periodStatus(t, pool, id); got != "open" {
			t.Errorf("status after rejected close: want open, got %s", got)
		}
	})

	t.Run("zero surplus closes without splits", func(t *testing.T) {
		// A period with nothing to allocate has nothing to lose, and the
		// year-lock tests rely on being able to close an empty period.
		const year, month = 2071, 9
		wipeTestYear(t, pool, year)
		id := createTestPeriod(t, c, year, month)

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close empty period: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("valid splits allocate the whole surplus", func(t *testing.T) {
		const year, month = 2071, 11
		wipeTestYear(t, pool, year)
		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 500000)
		potID := createTestPot(t, pool, "ZTest Full Split Pot", "normal", 901)
		insertSplit(t, pool, id, potID, "100")

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close with 100%% splits: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		if got := allocatedCents(t, pool, id, potID); got != 500000 {
			t.Errorf("allocation: want 500000, got %d", got)
		}
	})
}

// TestArchivedPotIsNotResurrectedByTemplateOrClose covers the two places an
// archived pot could still receive real money: copyPeriodTemplate copied
// every split row verbatim, and the close's split query did not filter
// archived pots either. detachPotSplits cleans the split up only for periods
// that are still open, so a closed period keeps it indefinitely.
func TestArchivedPotIsNotResurrectedByTemplateOrClose(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	t.Run("template copy drops the split and reassigns its percentage", func(t *testing.T) {
		const year = 2072
		wipeTestYear(t, pool, year)
		potID := createTestPot(t, pool, "ZTest Archived Template Pot", "normal", 910)
		carryID := carryoverPotID(t, pool)

		srcID := createTestPeriod(t, c, year, 1)
		addIncome(t, c, srcID, "Test salary", 100000)
		insertSplit(t, pool, srcID, potID, "60")
		insertSplit(t, pool, srcID, carryID, "40")

		// Closing the source period is what makes the split row survive the
		// archive: detachPotSplits is scoped to p.status <> 'closed'.
		resp := closePeriod(t, c, srcID)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close source period: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		if _, err := pool.Exec(ctx, `UPDATE pots SET archived_at=now() WHERE id=$1`, potID); err != nil {
			t.Fatalf("archive pot: %v", err)
		}

		// Copy the closed period forward.
		resp = c.do(http.MethodPost, "/api/periods", map[string]any{
			"year": year, "month": 4, "copyFromPeriodId": srcID,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create period from template: want 201, got %d", resp.StatusCode)
		}
		var dest struct {
			ID int64 `json:"id"`
		}
		c.decode(resp, &dest)

		var copied bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pot_splits WHERE period_id=$1 AND pot_id=$2)`,
			dest.ID, potID).Scan(&copied); err != nil {
			t.Fatalf("read copied split: %v", err)
		}
		if copied {
			t.Error("template copy: archived pot's split was copied forward")
		}

		// The freed 60% must not go missing either — whichever carryover pot
		// absorbs it, the copy has to add up to 100% again.
		var total float64
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(percentage),0)::float8 FROM pot_splits WHERE period_id=$1`,
			dest.ID).Scan(&total); err != nil {
			t.Fatalf("read copied percentages: %v", err)
		}
		if total != 100 {
			t.Errorf("copied split total: want 100, got %v", total)
		}
	})

	t.Run("close ignores an archived pot's split", func(t *testing.T) {
		const year, month = 2073, 5
		wipeTestYear(t, pool, year)
		livePotID := createTestPot(t, pool, "ZTest Live Close Pot", "normal", 912)
		archivedPotID := createTestPot(t, pool, "ZTest Archived Close Pot", "normal", 913)
		if _, err := pool.Exec(ctx, `UPDATE pots SET archived_at=now() WHERE id=$1`, archivedPotID); err != nil {
			t.Fatalf("archive pot: %v", err)
		}

		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 100000)
		// 100% to a live pot plus a leftover 30% for the archived one:
		// filtering the archived row leaves exactly 100%, which keeps the
		// split validation out of the way so the allocation is what is tested.
		insertSplit(t, pool, id, livePotID, "100")
		insertSplit(t, pool, id, archivedPotID, "30")

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		if got := allocatedCents(t, pool, id, archivedPotID); got != 0 {
			t.Errorf("allocation into archived pot: want 0, got %d", got)
		}
		if got := allocatedCents(t, pool, id, livePotID); got != 100000 {
			t.Errorf("allocation into live pot: want 100000, got %d", got)
		}
	})
}

// TestReopenDecemberBlockedWhenNextYearLocked pins the next-year lock check
// handleReopenPeriod was missing. A December close writes its carryover into
// January of the following year; reopening deletes those rows, so a lock on
// that year has to stop the reopen the same way it stops the close.
func TestReopenDecemberBlockedWhenNextYearLocked(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const year, nextYear = 2074, 2075
	wipeTestYear(t, pool, year, nextYear)
	carryID := carryoverPotID(t, pool)

	decID := createTestPeriod(t, c, year, 12)
	addIncome(t, c, decID, "Test salary", 100000)
	insertSplit(t, pool, decID, carryID, "100")

	resp := closePeriod(t, c, decID)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("close december: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	countCarryover := func() int {
		var n int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM income_entries ie JOIN periods p ON p.id = ie.period_id
			WHERE ie.entry_type='carryover' AND ie.source_period_id=$1 AND p.year=$2`,
			decID, nextYear).Scan(&n); err != nil {
			t.Fatalf("count carryover entries: %v", err)
		}
		return n
	}
	if got := countCarryover(); got != 1 {
		t.Fatalf("precondition: want 1 carryover entry in %d, got %d", nextYear, got)
	}

	// Lock the next year directly: the lock endpoint requires every period in
	// the year to be closed, and the January the close just created is open.
	if _, err := pool.Exec(ctx, `INSERT INTO locked_years (year) VALUES ($1) ON CONFLICT DO NOTHING`, nextYear); err != nil {
		t.Fatalf("lock next year: %v", err)
	}

	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(decID, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("reopen with next year locked: want 409, got %d", resp.StatusCode)
	}
	if code := errorCode(t, c, resp); code != "year_locked" {
		t.Errorf("error code: want year_locked, got %q", code)
	}

	// Nothing in the locked year may have been touched.
	if got := countCarryover(); got != 1 {
		t.Errorf("carryover entry in locked year: want 1 left intact, got %d", got)
	}
	if got := periodStatus(t, pool, decID); got != "closed" {
		t.Errorf("status after rejected reopen: want closed, got %s", got)
	}

	// With the lock lifted the reopen goes through again.
	if _, err := pool.Exec(ctx, `DELETE FROM locked_years WHERE year=$1`, nextYear); err != nil {
		t.Fatalf("unlock next year: %v", err)
	}
	resp = c.do(http.MethodPost, "/api/periods/"+strconv.FormatInt(decID, 10)+"/reopen", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reopen after unlock: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
