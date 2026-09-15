//go:build integration

package importer

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise Run() end-to-end against a real Postgres instance,
// using far-future test years (periods.year is CHECK-constrained to
// 2000-2100) and uniquely-prefixed masterdata names so they never collide
// with real household data, and always clean up what they create.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestRunImportsAtomically(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2095
	const incomeLabel = "ZTest Income 2095"
	const categoryLabel = "ZTest Category 2095"
	const potName = "ZTest Pot 2095"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name=$1`, incomeLabel)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
	})

	sheets := []SheetData{
		{
			Year: year, Month: 6, Kind: "Overview",
			Incomes: []IncomeRow{{Label: incomeLabel, AmountCents: 500000}},
			Lines:   []BudgetLineRow{{Label: categoryLabel, AmountCents: 120000}},
			Splits:  []SplitRow{{PotName: potName, Percentage: 100}},
		},
	}

	rep, err := Run(ctx, pool, sheets, ImportOptions{Year: year})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Months) != 1 {
		t.Fatalf("months: want 1, got %d", len(rep.Months))
	}
	mr := rep.Months[0]
	if mr.Month != 6 {
		t.Errorf("month: want 6, got %d", mr.Month)
	}
	if mr.IncomeTotalCents != 500000 {
		t.Errorf("incomeTotalCents: want 500000, got %d", mr.IncomeTotalCents)
	}
	if mr.ExpenseTotalCents != 120000 {
		t.Errorf("expenseTotalCents: want 120000, got %d", mr.ExpenseTotalCents)
	}
	if mr.SurplusCents != 380000 {
		t.Errorf("surplusCents: want 380000, got %d", mr.SurplusCents)
	}

	var periodCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM periods WHERE year=$1`, year).Scan(&periodCount); err != nil {
		t.Fatalf("count periods: %v", err)
	}
	if periodCount != 1 {
		t.Errorf("periods for year %d: want 1, got %d", year, periodCount)
	}

	var potBalance int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM pot_ledger pl JOIN pots p ON p.id=pl.pot_id WHERE p.name=$1`, potName).Scan(&potBalance); err != nil {
		t.Fatalf("pot balance: %v", err)
	}
	if potBalance != 0 {
		t.Errorf("pot ledger before close: want 0 (no close requested), got %d", potBalance)
	}
}

// TestRunRollsBackOnFailure forces a failure partway through Run (the
// period insert violates periods.year's CHECK(year BETWEEN 2000 AND 2100))
// and asserts the whole import — including the masterdata rows created
// earlier in the same call — rolled back instead of leaving a
// half-imported year with orphaned categories/income sources behind.
func TestRunRollsBackOnFailure(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 3000 // outside periods.year's CHECK(year BETWEEN 2000 AND 2100)
	const incomeLabel = "ZTest Income 3000 Rollback"
	const categoryLabel = "ZTest Category 3000 Rollback"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name=$1`, incomeLabel)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
	})

	sheets := []SheetData{
		{
			Year: year, Month: 1, Kind: "Overview",
			Incomes: []IncomeRow{{Label: incomeLabel, AmountCents: 100000}},
			Lines:   []BudgetLineRow{{Label: categoryLabel, AmountCents: 50000}},
		},
	}

	_, err := Run(ctx, pool, sheets, ImportOptions{Year: year})
	if err == nil {
		t.Fatal("Run: want error for out-of-range year, got nil")
	}

	var srcCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM income_sources WHERE name=$1`, incomeLabel).Scan(&srcCount); err != nil {
		t.Fatalf("count income_sources: %v", err)
	}
	if srcCount != 0 {
		t.Errorf("income source leaked after rollback: want 0, got %d", srcCount)
	}

	var catCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE name=$1`, categoryLabel).Scan(&catCount); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if catCount != 0 {
		t.Errorf("category leaked after rollback: want 0, got %d", catCount)
	}

	var periodCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM periods WHERE year=$1`, year).Scan(&periodCount); err != nil {
		t.Fatalf("count periods: %v", err)
	}
	if periodCount != 0 {
		t.Errorf("period leaked after rollback: want 0, got %d", periodCount)
	}
}

// TestRunResolvesCategoryAliasAsChild verifies the alias lookup added in
// mapper.go: a Details-table header with no exact-name category match, but
// a category_aliases row, should create a category with parent_id set
// instead of a disconnected top-level one — and a second import month
// reusing the same header should resolve to the same child category via
// the ordinary exact-name path, not re-consult the alias table.
func TestRunResolvesCategoryAliasAsChild(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2094
	const parentLabel = "ZTest Parent 2094"
	const aliasName = "ZTest Alias 2094"

	var parentID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1, 0, false, 0) RETURNING id`,
		parentLabel).Scan(&parentID); err != nil {
		t.Fatalf("seed parent category: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO category_aliases (alias_name, parent_category_id) VALUES ($1,$2)`, aliasName, parentID); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM category_aliases WHERE alias_name=$1`, aliasName)
		pool.Exec(ctx, `DELETE FROM categories WHERE name IN ($1,$2)`, parentLabel, aliasName)
	})

	month1 := []SheetData{
		{Year: year, Month: 1, Kind: "Overview", Lines: []BudgetLineRow{{Label: parentLabel, AmountCents: 50000}}},
		{Year: year, Month: 1, Kind: "Details", Txs: []TxRow{{CategoryLabel: aliasName, AmountCents: 1500, Description: "alias tx"}}},
	}
	if _, err := Run(ctx, pool, month1, ImportOptions{Year: year}); err != nil {
		t.Fatalf("Run (month 1): %v", err)
	}

	var childID int64
	var childParentID *int64
	if err := pool.QueryRow(ctx, `SELECT id, parent_id FROM categories WHERE name=$1`, aliasName).Scan(&childID, &childParentID); err != nil {
		t.Fatalf("lookup child category: %v", err)
	}
	if childParentID == nil || *childParentID != parentID {
		t.Fatalf("child parent_id: want %d, got %v", parentID, childParentID)
	}

	month2 := []SheetData{
		{Year: year, Month: 2, Kind: "Overview", Lines: []BudgetLineRow{{Label: parentLabel, AmountCents: 50000}}},
		{Year: year, Month: 2, Kind: "Details", Txs: []TxRow{{CategoryLabel: aliasName, AmountCents: 2000, Description: "alias tx month 2"}}},
	}
	if _, err := Run(ctx, pool, month2, ImportOptions{Year: year}); err != nil {
		t.Fatalf("Run (month 2): %v", err)
	}

	var catCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM categories WHERE name=$1`, aliasName).Scan(&catCount); err != nil {
		t.Fatalf("count child categories: %v", err)
	}
	if catCount != 1 {
		t.Fatalf("expected exactly 1 category named %q after 2 import runs (alias table only consulted once), got %d", aliasName, catCount)
	}
}

// TestRunIsIdempotentOnReimport covers the whole point of re-importing a
// corrected file: transactions have no natural key, so before the mapper
// cleared what the sheet rewrites, a second import of the same file doubled
// every income entry and every tracked expense while the budget-line amounts
// stayed put.
func TestRunIsIdempotentOnReimport(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2091
	const incomeLabel = "ZTest Income 2091"
	const categoryLabel = "ZTest Category 2091"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name=$1`, incomeLabel)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
	})

	sheets := []SheetData{
		{
			Year: year, Month: 3, Kind: "Overview",
			Incomes: []IncomeRow{{Label: incomeLabel, AmountCents: 500000}},
			Lines:   []BudgetLineRow{{Label: categoryLabel, AmountCents: 120000}},
		},
		{
			Year: year, Month: 3, Kind: "Details",
			Txs: []TxRow{
				{CategoryLabel: categoryLabel, AmountCents: 70000, Description: "een", Date: "2091-03-04"},
				{CategoryLabel: categoryLabel, AmountCents: 50000, Description: "twee", Date: "2091-03-05"},
			},
		},
	}

	for pass := 1; pass <= 3; pass++ {
		rep, err := Run(ctx, pool, sheets, ImportOptions{Year: year})
		if err != nil {
			t.Fatalf("pass %d: Run: %v", pass, err)
		}
		mr := rep.Months[0]
		if mr.IncomeTotalCents != 500000 {
			t.Errorf("pass %d: income: want 500000, got %d", pass, mr.IncomeTotalCents)
		}
		if mr.ExpenseTotalCents != 120000 {
			t.Errorf("pass %d: expense: want 120000, got %d", pass, mr.ExpenseTotalCents)
		}

		var incomeRows, txRows int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM income_entries ie JOIN periods p ON p.id=ie.period_id WHERE p.year=$1`, year).Scan(&incomeRows); err != nil {
			t.Fatalf("count income entries: %v", err)
		}
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM transactions t JOIN periods p ON p.id=t.period_id WHERE p.year=$1`, year).Scan(&txRows); err != nil {
			t.Fatalf("count transactions: %v", err)
		}
		if incomeRows != 1 {
			t.Fatalf("pass %d: income entries: want 1, got %d", pass, incomeRows)
		}
		if txRows != 2 {
			t.Fatalf("pass %d: transactions: want 2, got %d", pass, txRows)
		}
	}
}

// TestRunRejectsIncompleteSplits: LargestRemainderSplit assumes the total is
// already 100% and dumps the whole shortfall onto one arbitrary pot, so a sheet
// whose splits don't add up has to fail loudly instead of misallocating.
func TestRunRejectsIncompleteSplits(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2090
	const potName = "ZTest Pot 2090"
	const categoryLabel = "ZTest Category 2090"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
	})

	sheets := []SheetData{{
		Year: year, Month: 1, Kind: "Overview",
		Lines:  []BudgetLineRow{{Label: categoryLabel, AmountCents: 1000}},
		Splits: []SplitRow{{PotName: potName, Percentage: 60}},
	}}

	_, err := Run(ctx, pool, sheets, ImportOptions{Year: year})
	if err == nil {
		t.Fatal("Run: want an error for splits totalling 60%, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want a *ValidationError the UI can show, got %T: %v", err, err)
	}
	if ve.Month != 1 {
		t.Errorf("validation error month: want 1, got %d", ve.Month)
	}

	var periodCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM periods WHERE year=$1`, year).Scan(&periodCount); err != nil {
		t.Fatalf("count periods: %v", err)
	}
	if periodCount != 0 {
		t.Errorf("failed import must roll back: periods for %d want 0, got %d", year, periodCount)
	}
}

// TestRunCreatesPotsForEveryMonth: pot creation used to stop after the first
// month with an Overview, so a pot first appearing later was never created and
// its split was silently dropped.
func TestRunCreatesPotsForEveryMonth(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2089
	const potA = "ZTest Pot A 2089"
	const potB = "ZTest Pot B 2089"
	const categoryLabel = "ZTest Category 2089"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM pots WHERE name IN ($1,$2)`, potA, potB)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
	})

	sheets := []SheetData{
		{
			Year: year, Month: 1, Kind: "Overview",
			Lines:  []BudgetLineRow{{Label: categoryLabel, AmountCents: 1000}},
			Splits: []SplitRow{{PotName: potA, Percentage: 100}},
		},
		{
			Year: year, Month: 2, Kind: "Overview",
			Lines:  []BudgetLineRow{{Label: categoryLabel, AmountCents: 1000}},
			Splits: []SplitRow{{PotName: potA, Percentage: 40}, {PotName: potB, Percentage: 60}},
		},
	}

	if _, err := Run(ctx, pool, sheets, ImportOptions{Year: year}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var potCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pots WHERE name IN ($1,$2)`, potA, potB).Scan(&potCount); err != nil {
		t.Fatalf("count pots: %v", err)
	}
	if potCount != 2 {
		t.Fatalf("pots created: want 2, got %d", potCount)
	}

	var splitTotal float64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(ps.percentage),0) FROM pot_splits ps JOIN periods p ON p.id=ps.period_id WHERE p.year=$1 AND p.month=2`,
		year).Scan(&splitTotal); err != nil {
		t.Fatalf("sum splits: %v", err)
	}
	if splitTotal != 100 {
		t.Errorf("month 2 splits total: want 100, got %v", splitTotal)
	}
}

// TestCloseThroughDoesNotReClose: pot_ledger has no uniqueness, so importing
// the same file twice with CloseThrough used to write a second allocation for
// every pot and silently double the balances.
func TestCloseThroughDoesNotReClose(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const year = 2088
	const potName = "ZTest Pot 2088"
	const incomeLabel = "ZTest Income 2088"
	const categoryLabel = "ZTest Category 2088"

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
		pool.Exec(ctx, `DELETE FROM pots WHERE name=$1`, potName)
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name=$1`, incomeLabel)
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, categoryLabel)
	})

	sheets := []SheetData{{
		Year: year, Month: 1, Kind: "Overview",
		Incomes: []IncomeRow{{Label: incomeLabel, AmountCents: 300000}},
		Lines:   []BudgetLineRow{{Label: categoryLabel, AmountCents: 100000}},
		Splits:  []SplitRow{{PotName: potName, Percentage: 100}},
	}}

	for pass := 1; pass <= 2; pass++ {
		if _, err := Run(ctx, pool, sheets, ImportOptions{Year: year, CloseThrough: 1}); err != nil {
			t.Fatalf("pass %d: Run: %v", pass, err)
		}
		var entries int
		var balance int64
		if err := pool.QueryRow(ctx,
			`SELECT count(*), COALESCE(SUM(pl.amount_cents),0) FROM pot_ledger pl JOIN pots p ON p.id=pl.pot_id WHERE p.name=$1`,
			potName).Scan(&entries, &balance); err != nil {
			t.Fatalf("pass %d: read pot ledger: %v", pass, err)
		}
		if entries != 1 || balance != 200000 {
			t.Fatalf("pass %d: pot ledger: want 1 entry of 200000, got %d entries totalling %d", pass, entries, balance)
		}
	}
}
