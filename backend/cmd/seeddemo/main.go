// Command seeddemo populates a local dev database with a deterministic,
// realistic-looking demo dataset for year 2026: income, categories, a few
// hundred transactions (including a deliberately overstuffed catch-all
// category and inconsistently-spelled recurring descriptions), pots with
// splits and ledger activity, and kid savings entries.
//
// It mirrors cmd/importer's entry-point style (flags, pgxpool, DATABASE_URL
// fallback, slog) but is NOT a general-purpose tool: it only ever seeds the
// fixed year 2026 dataset described in its own code, and refuses to run
// against anything that doesn't look like a local database.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const demoYear = 2026

// seedRand sources ALL randomness in this program. It is seeded once, in
// main, with a fixed value — every call site below must draw from this one
// source, and the order in which the program draws from it must stay fixed,
// or two runs will stop producing byte-identical data.
var seedRand = rand.New(rand.NewSource(20260101))

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "postgres DSN")
	wipe := flag.Bool("wipe", false, "delete existing demo-relevant rows before seeding")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or --dsn required")
		os.Exit(1)
	}

	if err := requireLocalHost(*dsn); err != nil {
		slog.Error("refusing to run against a non-local database", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping", "err", err)
		os.Exit(1)
	}

	if err := run(ctx, pool, *wipe); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
}

// requireLocalHost is the hard safety gate: this tool deletes and inserts
// bulk rows, and must never be pointed at anything but a disposable local
// dev database.
//
// It deliberately asks pgx itself which hosts the DSN resolves to, rather
// than parsing the string a second time. Hand-parsing with net/url checks a
// different thing than pgx connects to: in a "postgres://" URI, libpq (and
// therefore pgx) lets a "?host=" query parameter override the host in the
// authority, so "postgres://user:pass@localhost/db?host=prod.internal"
// looks local to net/url and connects to prod.internal. Using
// pgconn.ParseConfig removes that parser disagreement, and covers
// keyword-style ("host=... port=...") DSNs in the same step.
func requireLocalHost(dsn string) error {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	hosts := make([]string, 0, len(cfg.Fallbacks)+1)
	if cfg.Host != "" {
		hosts = append(hosts, cfg.Host)
	}
	for _, fb := range cfg.Fallbacks {
		if fb.Host != "" {
			hosts = append(hosts, fb.Host)
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("could not determine host from DSN; refusing to run")
	}
	// Every candidate must be local: pgx tries the fallbacks in turn, so a
	// single non-local entry is enough to end up connected to it.
	for _, host := range hosts {
		if !isLocalHost(host) {
			return fmt.Errorf("DSN host %q is not localhost/127.0.0.1; this tool only runs against a local dev database", host)
		}
	}
	return nil
}

// isLocalHost accepts the loopback names and addresses, plus a Unix socket
// directory (pgx reports those as a path starting with "/"), which cannot
// reach another machine.
func isLocalHost(host string) bool {
	if strings.HasPrefix(host, "/") {
		return true
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func run(ctx context.Context, pool *pgxpool.Pool, wipe bool) error {
	var existing int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM periods WHERE year=$1`, demoYear).Scan(&existing); err != nil {
		return fmt.Errorf("check existing periods: %w", err)
	}
	if existing > 0 && !wipe {
		return fmt.Errorf("periods already exist for year %d; pass --wipe to delete existing demo-relevant rows first", demoYear)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if wipe {
		if err := wipeExisting(ctx, tx); err != nil {
			return fmt.Errorf("wipe: %w", err)
		}
		slog.Info("wiped existing demo-relevant rows")
	}

	s := &seeder{ctx: ctx, tx: tx}

	if err := s.seedYearAndPeriods(); err != nil {
		return fmt.Errorf("periods: %w", err)
	}
	if err := s.seedIncome(); err != nil {
		return fmt.Errorf("income: %w", err)
	}
	if err := s.seedCategories(); err != nil {
		return fmt.Errorf("categories: %w", err)
	}
	if err := s.seedBudgetLinesAndTransactions(); err != nil {
		return fmt.Errorf("budget lines / transactions: %w", err)
	}
	if err := s.seedPots(); err != nil {
		return fmt.Errorf("pots: %w", err)
	}
	if err := s.seedKidSavings(); err != nil {
		return fmt.Errorf("kid savings: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("seed complete",
		"year", demoYear,
		"periods", len(s.periodID),
		"incomeSources", len(s.sourceID),
		"categories", len(s.categoryID),
		"transactions", s.txCount,
		"pots", len(s.potID),
		"potLedgerEntries", s.potLedgerCount,
	)
	return nil
}

// wipeExisting deletes only the rows this tool itself owns, identified by
// the exact names its own specs use — this is a shared dev database that
// may hold other years' real/imported data, so a wipe must never touch
// masterdata (categories/income_sources/pots) that other periods still
// reference.
//
// Deleting `periods` for demoYear cascades into that year's
// income_entries, income_transactions, budget_lines, transactions,
// pot_splits, and any pot_ledger rows carrying one of those period_ids.
// What's left after that — pot_ledger rows with period_id NULL (manual
// deposit/withdrawal/opening_balance/adjustment entries) belonging to our
// named pots and dated within demoYear, and kid_savings_ledger rows dated
// within demoYear — have no
// cascade from periods and are deleted explicitly, in FK-safe order
// (children before parent categories; periods/ledger rows before the
// masterdata rows they reference). kids itself is never touched: those
// rows predate this tool and are not demo data.
func wipeExisting(ctx context.Context, tx pgx.Tx) error {
	yearStart := time.Date(demoYear, 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(demoYear+1, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := tx.Exec(ctx, `DELETE FROM periods WHERE year = $1`, demoYear); err != nil {
		return fmt.Errorf("delete periods: %w", err)
	}
	// Scoped to this tool's own rows, not to every row of a same-named pot:
	// deleting the periods above already cascaded the demo allocations, so
	// what is left to remove is the manual (period_id NULL) entries this
	// seeder writes, all of which are dated inside demoYear. Matching on the
	// pot name alone would wipe a real same-named pot's entire history across
	// every year — and would then also defeat the NOT EXISTS guard on the
	// `pots` delete below, dropping that pot too.
	if _, err := tx.Exec(ctx,
		`DELETE FROM pot_ledger
		 WHERE period_id IS NULL
		   AND entry_date >= $2 AND entry_date < $3
		   AND pot_id IN (SELECT id FROM pots WHERE name = ANY($1))`,
		potNames(), yearStart, yearEnd,
	); err != nil {
		return fmt.Errorf("delete pot_ledger: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM kid_savings_ledger WHERE entry_date >= $1 AND entry_date < $2`,
		yearStart, yearEnd,
	); err != nil {
		return fmt.Errorf("delete kid_savings_ledger: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM category_aliases WHERE alias_name = ANY($1)`,
		categoryAliasNames(),
	); err != nil {
		return fmt.Errorf("delete category_aliases: %w", err)
	}
	// Guarded by NOT EXISTS: a same-named row this tool doesn't own (e.g. a
	// real category called "Boodschappen" from an actual year's import)
	// must survive if anything other than what we just deleted still
	// references it — the FK would refuse the delete anyway, but checking
	// up front means an unrelated year's masterdata is silently left alone
	// instead of aborting the whole wipe.
	if _, err := tx.Exec(ctx,
		`DELETE FROM pots p WHERE p.name = ANY($1)
		   AND NOT EXISTS (SELECT 1 FROM pot_splits WHERE pot_id = p.id)
		   AND NOT EXISTS (SELECT 1 FROM pot_ledger WHERE pot_id = p.id)`,
		potNames(),
	); err != nil {
		return fmt.Errorf("delete pots: %w", err)
	}
	childCats, parentCats := categoryNamesByTier()
	categoryDeleteSQL := `DELETE FROM categories c WHERE c.name = ANY($1)
		AND NOT EXISTS (SELECT 1 FROM budget_lines WHERE category_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM transactions WHERE category_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM categories child WHERE child.parent_id = c.id)
		AND NOT EXISTS (SELECT 1 FROM category_aliases WHERE parent_category_id = c.id)`
	if _, err := tx.Exec(ctx, categoryDeleteSQL, childCats); err != nil {
		return fmt.Errorf("delete child categories: %w", err)
	}
	if _, err := tx.Exec(ctx, categoryDeleteSQL, parentCats); err != nil {
		return fmt.Errorf("delete parent categories: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM income_sources s WHERE s.name = ANY($1)
		   AND NOT EXISTS (SELECT 1 FROM income_entries WHERE source_id = s.id)
		   AND NOT EXISTS (SELECT 1 FROM income_transactions WHERE source_id = s.id)`,
		incomeSourceNames(),
	); err != nil {
		return fmt.Errorf("delete income_sources: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM years WHERE year = $1`, demoYear); err != nil {
		return fmt.Errorf("delete years: %w", err)
	}
	return nil
}

func potNames() []string {
	specs := potSpecs()
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.name
	}
	return names
}

func incomeSourceNames() []string {
	specs := incomeSourceSpecs()
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.name
	}
	return names
}

func categoryAliasNames() []string {
	return []string{"Bunq", "AH Bezorgservice"}
}

// categoryNamesByTier splits categorySpecs() into children (non-empty
// parent) and top-level names, so the caller can delete children before
// parents.
func categoryNamesByTier() (children, parents []string) {
	for _, s := range categorySpecs() {
		if s.parent == "" {
			parents = append(parents, s.name)
		} else {
			children = append(children, s.name)
		}
	}
	return children, parents
}

type seeder struct {
	ctx context.Context
	tx  pgx.Tx

	periodID   map[int]int64 // month -> period id
	sourceID   map[string]int64
	categoryID map[string]int64
	potID      map[string]int64
	catSpecs   []categorySpec

	txCount        int
	potLedgerCount int
}

func (s *seeder) scanID(sql string, args ...any) (int64, error) {
	var id int64
	err := s.tx.QueryRow(s.ctx, sql, args...).Scan(&id)
	return id, err
}
