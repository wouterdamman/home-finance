package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wouterdamman/home-finance/internal/domain"
)

// dbtx is satisfied by both *pgxpool.Pool and pgx.Tx, so masterdata/close
// helpers can run either standalone or, as Run does, all inside one
// transaction that spans the whole import.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ImportOptions struct {
	Year         int
	Wipe         bool
	ResetMaster  bool // truncate masterdata before import
	CloseThrough int  // month number, inclusive; 0 = don't close
}

type Report struct {
	Months []MonthReport `json:"months"`
}

type MonthReport struct {
	Month             int   `json:"month"`
	IncomeTotalCents  int64 `json:"incomeTotalCents"`
	ExpenseTotalCents int64 `json:"expenseTotalCents"`
	SurplusCents      int64 `json:"surplusCents"`
	Closed            bool  `json:"closed"`
}

// Run imports sheets inside a single transaction spanning the whole
// operation: a mid-import failure (a bad row, a constraint violation, a
// dropped connection) rolls back everything instead of leaving a
// half-imported year while the returned report claims success.
func Run(ctx context.Context, pool *pgxpool.Pool, sheets []SheetData, opts ImportOptions) (*Report, error) {
	dbTx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	if opts.ResetMaster {
		if err := resetMasterdata(ctx, dbTx); err != nil {
			return nil, fmt.Errorf("reset masterdata: %w", err)
		}
	} else if opts.Wipe {
		if err := wipe(ctx, dbTx, opts.Year); err != nil {
			return nil, fmt.Errorf("wipe: %w", err)
		}
	}

	// Index sheets by year+month
	overviews := map[int]*SheetData{}
	details := map[int][]TxRow{}
	for i := range sheets {
		sd := &sheets[i]
		if opts.Year != 0 && sd.Year != opts.Year {
			continue
		}
		if sd.Kind == "Overview" {
			overviews[sd.Month] = sd
		} else {
			details[sd.Month] = append(details[sd.Month], sd.Txs...)
		}
	}

	// Load/create masterdata
	catIDs := map[string]int64{}
	srcIDs := map[string]int64{}
	potIDs := map[string]int64{}
	potKinds := map[string]string{}
	if err := loadMasterdata(ctx, dbTx, catIDs, srcIDs, potIDs, potKinds); err != nil {
		return nil, fmt.Errorf("load masterdata: %w", err)
	}

	// First pass: ensure all pots exist (from first month's splits)
	for monthNum := 1; monthNum <= 12; monthNum++ {
		ov, ok := overviews[monthNum]
		if !ok {
			continue
		}
		for _, sp := range ov.Splits {
			name := sp.PotName
			if _, exists := potIDs[name]; !exists {
				kind := "normal"
				if strings.Contains(strings.ToLower(name), "doorlopen") {
					kind = "carryover"
				}
				var id int64
				err := dbTx.QueryRow(ctx, `SELECT id FROM pots WHERE name=$1`, name).Scan(&id)
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) {
						return nil, fmt.Errorf("lookup pot %q: %w", name, err)
					}
					if err := dbTx.QueryRow(ctx,
						`INSERT INTO pots (name, kind, sort_order) VALUES ($1, $2, $3) RETURNING id`,
						name, kind, len(potIDs)).Scan(&id); err != nil {
						return nil, fmt.Errorf("create pot %q: %w", name, err)
					}
				}
				potIDs[name] = id
				potKinds[name] = kind
			}
		}
		break // only need first month's splits to create pots
	}
	// Reload to capture all pots including any just created
	if err := loadMasterdata(ctx, dbTx, catIDs, srcIDs, potIDs, potKinds); err != nil {
		return nil, fmt.Errorf("reload masterdata: %w", err)
	}

	var rep Report
	for monthNum := 1; monthNum <= 12; monthNum++ {
		ov, ok := overviews[monthNum]
		if !ok {
			continue
		}

		// Upsert income sources
		for _, inc := range ov.Incomes {
			if _, exists := srcIDs[inc.Label]; !exists {
				var id int64
				err := dbTx.QueryRow(ctx, `SELECT id FROM income_sources WHERE name=$1`, inc.Label).Scan(&id)
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) {
						return nil, fmt.Errorf("month %d: lookup income source %q: %w", monthNum, inc.Label, err)
					}
					if err := dbTx.QueryRow(ctx,
						`INSERT INTO income_sources (name, default_amount_cents, sort_order) VALUES ($1, $2, $3) RETURNING id`,
						inc.Label, inc.AmountCents, len(srcIDs)).Scan(&id); err != nil {
						return nil, fmt.Errorf("month %d: create income source %q: %w", monthNum, inc.Label, err)
					}
				}
				srcIDs[inc.Label] = id
			}
		}

		// Upsert categories from budget lines
		for _, bl := range ov.Lines {
			if _, exists := catIDs[bl.Label]; !exists {
				var id int64
				err := dbTx.QueryRow(ctx, `SELECT id FROM categories WHERE name=$1`, bl.Label).Scan(&id)
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) {
						return nil, fmt.Errorf("month %d: lookup category %q: %w", monthNum, bl.Label, err)
					}
					if err := dbTx.QueryRow(ctx,
						`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1, $2, false, $3) RETURNING id`,
						bl.Label, bl.AmountCents, len(catIDs)).Scan(&id); err != nil {
						return nil, fmt.Errorf("month %d: create category %q: %w", monthNum, bl.Label, err)
					}
				}
				catIDs[bl.Label] = id
			}
		}

		// Upsert categories from Details headers
		for _, tx := range details[monthNum] {
			if _, exists := catIDs[tx.CategoryLabel]; !exists {
				var id int64
				err := dbTx.QueryRow(ctx, `SELECT id FROM categories WHERE name=$1`, tx.CategoryLabel).Scan(&id)
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) {
						return nil, fmt.Errorf("month %d: lookup category %q: %w", monthNum, tx.CategoryLabel, err)
					}
					if err := dbTx.QueryRow(ctx,
						`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1, 0, true, $2) RETURNING id`,
						tx.CategoryLabel, len(catIDs)).Scan(&id); err != nil {
						return nil, fmt.Errorf("month %d: create category %q: %w", monthNum, tx.CategoryLabel, err)
					}
				} else {
					// Mark existing as itemized
					if _, err := dbTx.Exec(ctx, `UPDATE categories SET is_itemized=true WHERE id=$1`, id); err != nil {
						return nil, fmt.Errorf("month %d: mark category %q itemized: %w", monthNum, tx.CategoryLabel, err)
					}
				}
				catIDs[tx.CategoryLabel] = id
			}
		}

		// Create period
		var periodID int64
		if err := dbTx.QueryRow(ctx,
			`INSERT INTO periods (year, month)
			 VALUES ($1, $2)
			 ON CONFLICT (year, month) DO UPDATE SET year=EXCLUDED.year
			 RETURNING id`,
			opts.Year, monthNum).Scan(&periodID); err != nil {
			return nil, fmt.Errorf("month %d: create period: %w", monthNum, err)
		}

		// Income entries (skip carryover — generated by close)
		for i, inc := range ov.Incomes {
			srcID := srcIDs[inc.Label]
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, notes, sort_order)
				 VALUES ($1, $2, $3, $4, 'normal', '', $5)
				 ON CONFLICT DO NOTHING`,
				periodID, srcID, inc.Label, inc.AmountCents, i); err != nil {
				return nil, fmt.Errorf("month %d: insert income entry %q: %w", monthNum, inc.Label, err)
			}
		}

		// Budget lines
		for i, bl := range ov.Lines {
			catID := catIDs[bl.Label]
			// tracks_transactions = true if a Details category has the same name
			tracksTransactions := false
			if _, ok := catIDs[bl.Label]; ok {
				for _, tx := range details[monthNum] {
					if strings.EqualFold(tx.CategoryLabel, bl.Label) {
						tracksTransactions = true
						break
					}
				}
			}
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT (period_id, category_id) DO NOTHING`,
				periodID, catID, bl.Label, bl.AmountCents, tracksTransactions, i); err != nil {
				return nil, fmt.Errorf("month %d: insert budget line %q: %w", monthNum, bl.Label, err)
			}
		}

		// Transactions from Details
		for _, tx := range details[monthNum] {
			catID := catIDs[tx.CategoryLabel]
			txDate := tx.Date
			if txDate == "" {
				txDate = time.Date(opts.Year, time.Month(monthNum), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			}
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date)
				 VALUES ($1, $2, $3, $4, $5)`,
				periodID, catID, tx.AmountCents, tx.Description, txDate); err != nil {
				return nil, fmt.Errorf("month %d: insert transaction %q: %w", monthNum, tx.Description, err)
			}
		}

		// Pot splits for this period
		for _, sp := range ov.Splits {
			potID, ok := potIDs[sp.PotName]
			if !ok {
				slog.Warn("pot not found for split", "pot", sp.PotName)
				continue
			}
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO pot_splits (period_id, pot_id, percentage)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (period_id, pot_id) DO UPDATE SET percentage=EXCLUDED.percentage`,
				periodID, potID, sp.Percentage); err != nil {
				return nil, fmt.Errorf("month %d: insert pot split %q: %w", monthNum, sp.PotName, err)
			}
		}

		// Calculate totals for report
		var incTotal, expTotal int64
		if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=$1 AND ie.entry_type='normal'`, periodID).Scan(&incTotal); err != nil {
			return nil, fmt.Errorf("month %d: compute income total: %w", monthNum, err)
		}
		if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=$1`, periodID).Scan(&expTotal); err != nil {
			return nil, fmt.Errorf("month %d: compute expense total: %w", monthNum, err)
		}

		mr := MonthReport{
			Month:             monthNum,
			IncomeTotalCents:  incTotal,
			ExpenseTotalCents: expTotal,
			SurplusCents:      incTotal - expTotal,
		}

		if opts.CloseThrough >= monthNum {
			if err := closePeriod(ctx, dbTx, periodID, potIDs, potKinds, opts.Year, monthNum); err != nil {
				return nil, fmt.Errorf("month %d: close period: %w", monthNum, err)
			}
			mr.Closed = true
		}

		rep.Months = append(rep.Months, mr)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &rep, nil
}

func closePeriod(ctx context.Context, dbTx dbtx, periodID int64, potIDs map[string]int64, potKinds map[string]string, year, month int) error {
	type splitRow struct {
		PotID int64
		Pct   float64
		Kind  string
	}
	rows, err := dbTx.Query(ctx,
		`SELECT ps.pot_id, ps.percentage, p.kind
		 FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id
		 WHERE ps.period_id=$1`, periodID)
	if err != nil {
		return err
	}
	var rawSplits []splitRow
	for rows.Next() {
		var sr splitRow
		if err := rows.Scan(&sr.PotID, &sr.Pct, &sr.Kind); err != nil {
			rows.Close()
			return err
		}
		rawSplits = append(rawSplits, sr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(rawSplits) == 0 {
		slog.Info("no splits, skipping close", "period_id", periodID)
		return nil
	}

	inputs := make([]domain.PotSplitInput, len(rawSplits))
	for i, sr := range rawSplits {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
	}

	var incTotal, expTotal int64
	if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=$1`, periodID).Scan(&incTotal); err != nil {
		return err
	}
	if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=$1`, periodID).Scan(&expTotal); err != nil {
		return err
	}
	surplus := incTotal - expTotal

	allocs := domain.LargestRemainderSplit(surplus, inputs)
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	for i, a := range allocs {
		if _, err := dbTx.Exec(ctx,
			`INSERT INTO pot_ledger (pot_id, period_id, source_period_id, entry_type, amount_cents, description, entry_date)
			 VALUES ($1, $2, $2, 'allocation', $3, 'Maandelijkse allocatie', $4)`,
			a.PotID, periodID, a.AmountCents, lastDay); err != nil {
			return err
		}

		if rawSplits[i].Kind == "carryover" {
			nextMonth := month + 1
			nextYear := year
			if nextMonth > 12 {
				nextMonth = 1
				nextYear++
			}
			var nextPeriodID int64
			if err := dbTx.QueryRow(ctx,
				`INSERT INTO periods (year, month) VALUES ($1, $2)
				 ON CONFLICT (year, month) DO UPDATE SET year=EXCLUDED.year
				 RETURNING id`,
				nextYear, nextMonth).Scan(&nextPeriodID); err != nil {
				return err
			}

			if _, err := dbTx.Exec(ctx,
				`INSERT INTO pot_ledger (pot_id, period_id, source_period_id, entry_type, amount_cents, description, entry_date)
				 VALUES ($1, $2, $3, 'carryover_out', $4, 'Doorlopen', $5)`,
				a.PotID, nextPeriodID, periodID, -a.AmountCents, lastDay); err != nil {
				return err
			}

			monthNames := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni",
				"Juli", "Augustus", "September", "Oktober", "November", "December"}
			label := "Doorlopen maand " + monthNames[month]
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, source_period_id, notes, sort_order)
				 VALUES ($1, NULL, $2, $3, 'carryover', $4, '', 0)`,
				nextPeriodID, label, a.AmountCents, periodID); err != nil {
				return err
			}
		}
	}

	if _, err := dbTx.Exec(ctx, `UPDATE periods SET status='closed', closed_at=now() WHERE id=$1`, periodID); err != nil {
		return err
	}
	return nil
}

// wipe deletes every period for the year — pot_ledger, pot_splits,
// transactions, budget_lines and income_entries all reference periods(id)
// ON DELETE CASCADE (migration 0001), so one statement is enough.
func wipe(ctx context.Context, dbTx dbtx, year int) error {
	_, err := dbTx.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
	return err
}

// resetMasterdata wipes all periods (cascading to every period-scoped table:
// income_entries, income_transactions, budget_lines, transactions, pot_splits,
// pot_ledger) plus the masterdata tables themselves. pgx v5's extended query
// protocol rejects multiple semicolon-separated statements in one Exec, so each
// DELETE runs as its own statement; the caller's transaction (Run's dbTx) makes
// a failure partway through roll back everything, not leave masterdata
// half-wiped.
func resetMasterdata(ctx context.Context, dbTx dbtx) error {
	for _, table := range []string{"periods", "pots", "categories", "income_sources"} {
		if _, err := dbTx.Exec(ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	return nil
}

func loadMasterdata(ctx context.Context, dbTx dbtx, catIDs, srcIDs, potIDs map[string]int64, potKinds map[string]string) error {
	rows, err := dbTx.Query(ctx, `SELECT id, name FROM categories`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		catIDs[strings.TrimSpace(name)] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = dbTx.Query(ctx, `SELECT id, name FROM income_sources`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		srcIDs[strings.TrimSpace(name)] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = dbTx.Query(ctx, `SELECT id, name, kind FROM pots`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name, kind string
		if err := rows.Scan(&id, &name, &kind); err != nil {
			rows.Close()
			return err
		}
		potIDs[strings.TrimSpace(name)] = id
		if potKinds != nil {
			potKinds[strings.TrimSpace(name)] = kind
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
