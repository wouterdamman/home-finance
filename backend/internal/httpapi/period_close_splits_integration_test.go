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
// 2070s (periods.year is CHECK-constrained to 2000-2100) and wipe every
// period they touch in t.Cleanup.

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

// carryoverPotID returns the household's carryover pot, which every split
// replacement implicitly tops up to keep the total at exactly 100%.
func carryoverPotID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM pots WHERE kind='carryover' AND archived_at IS NULL ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("carryover pot: %v", err)
	}
	return id
}

// createTestPot inserts a pot directly (there is no delete endpoint, so the
// cleanup is raw SQL too) and returns its id.
func createTestPot(t *testing.T, pool *pgxpool.Pool, name string, sortOrder int) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO pots (name,kind,sort_order) VALUES ($1,'normal',$2) RETURNING id`,
		name, sortOrder).Scan(&id); err != nil {
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

// putSplits replaces a period's splits over HTTP. The carryover pot is
// topped up to the remainder by the handler itself.
func putSplits(t *testing.T, c *apiClient, periodID int64, splits ...map[string]any) {
	t.Helper()
	if splits == nil {
		splits = []map[string]any{}
	}
	resp := c.do(http.MethodPut, "/api/periods/"+strconv.FormatInt(periodID, 10)+"/splits",
		map[string]any{"splits": splits})
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("put splits: want 204, got %d", resp.StatusCode)
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

// TestClosePeriodRejectsSurplusWithoutValidSplits pins the guard that keeps a
// close from silently swallowing the surplus: domain.LargestRemainderSplit
// returns nothing for an empty or non-100% split set, so the allocation loop
// never runs and the money is gone with a 204.
func TestClosePeriodRejectsSurplusWithoutValidSplits(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

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

		// The period must still be open and the surplus untouched.
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM periods WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatalf("read status: %v", err)
		}
		if status != "open" {
			t.Errorf("status after rejected close: want open, got %s", status)
		}
	})

	t.Run("splits that do not total 100", func(t *testing.T) {
		const year, month = 2071, 7
		wipeTestYear(t, pool, year)
		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 500000)

		// The HTTP split endpoint always tops the total up to 100% via the
		// carryover pot, so a partial set has to be written directly — the
		// state an import or an older release could leave behind.
		potID := createTestPot(t, pool, "ZTest Partial Split Pot", 900)
		if _, err := pool.Exec(ctx,
			`INSERT INTO pot_splits (period_id,pot_id,percentage) VALUES ($1,$2,50)`, id, potID); err != nil {
			t.Fatalf("insert partial split: %v", err)
		}

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("close with 50%% of splits: want 409, got %d", resp.StatusCode)
		}
		if code := errorCode(t, c, resp); code != "split_percentage_not_100" {
			t.Errorf("error code: want split_percentage_not_100, got %q", code)
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
		potID := createTestPot(t, pool, "ZTest Full Split Pot", 901)
		putSplits(t, c, id, map[string]any{"potId": potID, "percentage": "100"})

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
		carryID := carryoverPotID(t, pool)
		potID := createTestPot(t, pool, "ZTest Archived Template Pot", 910)

		srcID := createTestPeriod(t, c, year, 1)
		addIncome(t, c, srcID, "Test salary", 100000)
		putSplits(t, c, srcID, map[string]any{"potId": potID, "percentage": "60"})

		// Closing the source period is what makes the split row survive the
		// archive: detachPotSplits is scoped to p.status <> 'closed'.
		resp := closePeriod(t, c, srcID)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close source period: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		resp = c.do(http.MethodPost, "/api/pots/"+strconv.FormatInt(potID, 10)+"/archive", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("archive pot: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		var stillThere bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pot_splits WHERE period_id=$1 AND pot_id=$2)`,
			srcID, potID).Scan(&stillThere); err != nil {
			t.Fatalf("read source split: %v", err)
		}
		if !stillThere {
			t.Fatal("precondition: closed period should still hold the archived pot's split")
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

		var carryPct, total float64
		if err := pool.QueryRow(ctx, `
			SELECT COALESCE((SELECT percentage FROM pot_splits WHERE period_id=$1 AND pot_id=$2),0)::float8,
			       COALESCE((SELECT SUM(percentage) FROM pot_splits WHERE period_id=$1),0)::float8`,
			dest.ID, carryID).Scan(&carryPct, &total); err != nil {
			t.Fatalf("read copied percentages: %v", err)
		}
		if carryPct != 100 {
			t.Errorf("carryover percentage after reassignment: want 100, got %v", carryPct)
		}
		if total != 100 {
			t.Errorf("copied split total: want 100, got %v", total)
		}
	})

	t.Run("close ignores an archived pot's split", func(t *testing.T) {
		const year, month = 2073, 5
		wipeTestYear(t, pool, year)
		carryID := carryoverPotID(t, pool)
		potID := createTestPot(t, pool, "ZTest Archived Close Pot", 911)
		if _, err := pool.Exec(ctx, `UPDATE pots SET archived_at=now() WHERE id=$1`, potID); err != nil {
			t.Fatalf("archive pot: %v", err)
		}

		id := createTestPeriod(t, c, year, month)
		addIncome(t, c, id, "Test salary", 100000)
		putSplits(t, c, id)

		// 100% to the carryover pot plus a leftover 30% for the archived pot:
		// filtering the archived row leaves exactly 100%, which keeps the
		// split validation out of the way so the allocation is what is tested.
		if _, err := pool.Exec(ctx,
			`INSERT INTO pot_splits (period_id,pot_id,percentage) VALUES ($1,$2,30)`, id, potID); err != nil {
			t.Fatalf("insert archived split: %v", err)
		}

		resp := closePeriod(t, c, id)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("close: want 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		if got := allocatedCents(t, pool, id, potID); got != 0 {
			t.Errorf("allocation into archived pot: want 0, got %d", got)
		}
		if got := allocatedCents(t, pool, id, carryID); got != 100000 {
			t.Errorf("allocation into carryover pot: want 100000, got %d", got)
		}
	})
}
