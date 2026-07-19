//go:build integration

package importer

import (
	"context"
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
