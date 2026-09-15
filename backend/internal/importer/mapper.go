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
	Months        []MonthReport `json:"months"`
	SkippedSheets []string      `json:"skippedSheets,omitempty"`
	// Problems lists cells that could not be imported (unreadable or
	// out-of-range amounts, rows missing a category). The rows they name were
	// skipped; everything else was imported.
	Problems []string `json:"problems,omitempty"`
	// ResetCounts is the per-table row count deleted by ResetMaster, captured
	// before the wipe so the audit record shows its real blast radius.
	ResetCounts map[string]int64 `json:"resetCounts,omitempty"`
}

type MonthReport struct {
	Month             int   `json:"month"`
	IncomeTotalCents  int64 `json:"incomeTotalCents"`
	ExpenseTotalCents int64 `json:"expenseTotalCents"`
	SurplusCents      int64 `json:"surplusCents"`
	Closed            bool  `json:"closed"`
}

// ValidationError is an import failure the caller can act on. Its Message is
// written here and never carries database or filesystem internals, and
// Month/Reference locate the offending row, so a handler can hand it straight
// back to the user — unlike every other error out of Run, which is a driver or
// constraint error and must only be logged.
type ValidationError struct {
	Month     int    `json:"month,omitempty"`
	Reference string `json:"reference,omitempty"`
	Message   string `json:"message"`
}

func (e *ValidationError) Error() string {
	switch {
	case e.Month != 0 && e.Reference != "":
		return fmt.Sprintf("month %d, %q: %s", e.Month, e.Reference, e.Message)
	case e.Month != 0:
		return fmt.Sprintf("month %d: %s", e.Month, e.Message)
	}
	return e.Message
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

	var rep Report

	if opts.ResetMaster {
		counts, err := resetMasterdata(ctx, dbTx)
		if err != nil {
			return nil, fmt.Errorf("reset masterdata: %w", err)
		}
		rep.ResetCounts = counts
	} else if opts.Wipe {
		if err := wipe(ctx, dbTx, opts.Year); err != nil {
			return nil, fmt.Errorf("wipe: %w", err)
		}
	}

	// Index sheets by year+month
	overviews := map[int]*SheetData{}
	details := map[int][]TxRow{}
	incomeDetails := map[int][]IncomeTxRow{}
	for i := range sheets {
		sd := &sheets[i]
		if opts.Year != 0 && sd.Year != opts.Year {
			continue
		}
		rep.Problems = append(rep.Problems, sd.Problems...)
		if sd.Kind == "Overview" {
			overviews[sd.Month] = sd
		} else {
			details[sd.Month] = append(details[sd.Month], sd.Txs...)
		}
		incomeDetails[sd.Month] = append(incomeDetails[sd.Month], sd.IncomeTxs...)
	}

	// Load/create masterdata
	catIDs := map[string]int64{}
	srcIDs := map[string]int64{}
	potIDs := map[string]int64{}
	if err := loadMasterdata(ctx, dbTx, catIDs, srcIDs, potIDs, nil); err != nil {
		return nil, fmt.Errorf("load masterdata: %w", err)
	}

	// First pass: ensure every pot named by any month's splits exists. Stopping
	// after the first month left a pot that first appears later uncreated, and
	// its split silently dropped — which leaves the remaining splits totalling
	// under 100% and LargestRemainderSplit piling the leftover onto one pot.
	for monthNum := 1; monthNum <= 12; monthNum++ {
		ov, ok := overviews[monthNum]
		if !ok {
			continue
		}
		for _, sp := range ov.Splits {
			name := sp.PotName
			if _, exists := potIDs[name]; exists {
				continue
			}
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
		}
	}
	// Reload to capture all pots including any just created
	if err := loadMasterdata(ctx, dbTx, catIDs, srcIDs, potIDs, nil); err != nil {
		return nil, fmt.Errorf("reload masterdata: %w", err)
	}

	for monthNum := 1; monthNum <= 12; monthNum++ {
		ov, ok := overviews[monthNum]
		if !ok {
			continue
		}

		// Upsert income sources
		for _, inc := range ov.Incomes {
			if err := ensureIncomeSource(ctx, dbTx, srcIDs, monthNum, inc.Label, inc.AmountCents); err != nil {
				return nil, err
			}
		}

		// Upsert income sources named only by itemized line items, and mark
		// them itemized so EffectiveIncomeCentsSQL sums those rows instead of
		// reading the (redundant) header amount.
		for _, itx := range incomeDetails[monthNum] {
			if err := ensureIncomeSource(ctx, dbTx, srcIDs, monthNum, itx.SourceLabel, 0); err != nil {
				return nil, err
			}
			if _, err := dbTx.Exec(ctx,
				`UPDATE income_sources SET is_itemized=true WHERE id=$1 AND is_itemized=false`,
				srcIDs[itx.SourceLabel]); err != nil {
				return nil, fmt.Errorf("month %d: mark income source %q itemized: %w", monthNum, itx.SourceLabel, err)
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
					// No exact-name match — check for a known alias (e.g.
					// "Bunq" -> "Boodschappen") before falling back to
					// creating a brand-new top-level category.
					var parentID int64
					aliasErr := dbTx.QueryRow(ctx, `SELECT parent_category_id FROM category_aliases WHERE alias_name=$1`, tx.CategoryLabel).Scan(&parentID)
					switch {
					case aliasErr == nil:
						if err := dbTx.QueryRow(ctx,
							`INSERT INTO categories (name, parent_id, default_amount_cents, is_itemized, sort_order) VALUES ($1, $2, 0, true, $3) RETURNING id`,
							tx.CategoryLabel, parentID, len(catIDs)).Scan(&id); err != nil {
							return nil, fmt.Errorf("month %d: create child category %q: %w", monthNum, tx.CategoryLabel, err)
						}
					case errors.Is(aliasErr, pgx.ErrNoRows):
						if err := dbTx.QueryRow(ctx,
							`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order) VALUES ($1, 0, true, $2) RETURNING id`,
							tx.CategoryLabel, len(catIDs)).Scan(&id); err != nil {
							return nil, fmt.Errorf("month %d: create category %q: %w", monthNum, tx.CategoryLabel, err)
						}
					default:
						return nil, fmt.Errorf("month %d: lookup category alias %q: %w", monthNum, tx.CategoryLabel, aliasErr)
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
		if _, err := dbTx.Exec(ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, opts.Year); err != nil {
			return nil, fmt.Errorf("month %d: register year: %w", monthNum, err)
		}
		var periodID int64
		if err := dbTx.QueryRow(ctx,
			`INSERT INTO periods (year, month)
			 VALUES ($1, $2)
			 ON CONFLICT (year, month) DO UPDATE SET year=EXCLUDED.year
			 RETURNING id`,
			opts.Year, monthNum).Scan(&periodID); err != nil {
			return nil, fmt.Errorf("month %d: create period: %w", monthNum, err)
		}

		// Re-importing a corrected file must not append a second copy of every
		// row: transactions carry no natural key (two €5 coffees on one day are
		// both real), so idempotency comes from clearing what this sheet is
		// about to rewrite. Scoped to the sources/categories the sheet actually
		// carries — a category the sheet doesn't mention, and the carryover
		// entry that close generated, are left untouched.
		sheetSrcIDs := idsFor(srcIDs, incomeLabels(ov.Incomes))
		if len(sheetSrcIDs) > 0 {
			if _, err := dbTx.Exec(ctx,
				`DELETE FROM income_entries WHERE period_id=$1 AND entry_type='normal' AND source_id = ANY($2)`,
				periodID, sheetSrcIDs); err != nil {
				return nil, fmt.Errorf("month %d: clear income entries: %w", monthNum, err)
			}
		}
		itemizedSrcIDs := idsFor(srcIDs, incomeTxLabels(incomeDetails[monthNum]))
		if len(itemizedSrcIDs) > 0 {
			if _, err := dbTx.Exec(ctx,
				`DELETE FROM income_transactions WHERE period_id=$1 AND source_id = ANY($2)`,
				periodID, itemizedSrcIDs); err != nil {
				return nil, fmt.Errorf("month %d: clear income transactions: %w", monthNum, err)
			}
		}
		sheetCatIDs := idsFor(catIDs, txLabels(details[monthNum]))
		if len(sheetCatIDs) > 0 {
			if _, err := dbTx.Exec(ctx,
				`DELETE FROM transactions WHERE period_id=$1 AND category_id = ANY($2)`,
				periodID, sheetCatIDs); err != nil {
				return nil, fmt.Errorf("month %d: clear transactions: %w", monthNum, err)
			}
		}

		// Income entries (carryover entries are generated by close, never imported)
		for i, inc := range ov.Incomes {
			// A sheet that lists the same source twice means two real payments;
			// income_entries is unique per (period, source), so fold them.
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, notes, sort_order)
				 VALUES ($1, $2, $3, $4, 'normal', '', $5)
				 ON CONFLICT (period_id, source_id) WHERE source_id IS NOT NULL
				 DO UPDATE SET amount_cents = income_entries.amount_cents + EXCLUDED.amount_cents`,
				periodID, srcIDs[inc.Label], inc.Label, inc.AmountCents, i); err != nil {
				return nil, fmt.Errorf("month %d: insert income entry %q: %w", monthNum, inc.Label, err)
			}
		}

		// Income transactions (line items of an itemized source)
		for _, itx := range incomeDetails[monthNum] {
			txDate := itx.Date
			if txDate == "" {
				txDate = firstOfMonth(opts.Year, monthNum)
			}
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO income_transactions (period_id, source_id, amount_cents, description, tx_date)
				 VALUES ($1, $2, $3, $4, $5)`,
				periodID, srcIDs[itx.SourceLabel], itx.AmountCents, itx.Description, txDate); err != nil {
				return nil, fmt.Errorf("month %d: insert income transaction %q: %w", monthNum, itx.Description, err)
			}
		}

		// Budget lines
		for i, bl := range ov.Lines {
			// tracks_transactions = true if a Details category has the same name
			tracksTransactions := false
			for _, tx := range details[monthNum] {
				if strings.EqualFold(tx.CategoryLabel, bl.Label) {
					tracksTransactions = true
					break
				}
			}
			// tracks_transactions is deliberately a per-period user choice, so
			// importing a sheet that happens to carry no transactions for a
			// tracked line must not silently untrack it.
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO budget_lines (period_id, category_id, label, amount_cents, tracks_transactions, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 ON CONFLICT (period_id, category_id) DO UPDATE
				 SET label=EXCLUDED.label,
				     amount_cents=EXCLUDED.amount_cents,
				     tracks_transactions = budget_lines.tracks_transactions OR EXCLUDED.tracks_transactions,
				     sort_order=EXCLUDED.sort_order`,
				periodID, catIDs[bl.Label], bl.Label, bl.AmountCents, tracksTransactions, i); err != nil {
				return nil, fmt.Errorf("month %d: insert budget line %q: %w", monthNum, bl.Label, err)
			}
		}

		// Transactions from Details
		for _, tx := range details[monthNum] {
			txDate := tx.Date
			if txDate == "" {
				txDate = firstOfMonth(opts.Year, monthNum)
			}
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date)
				 VALUES ($1, $2, $3, $4, $5)`,
				periodID, catIDs[tx.CategoryLabel], tx.AmountCents, tx.Description, txDate); err != nil {
				return nil, fmt.Errorf("month %d: insert transaction %q: %w", monthNum, tx.Description, err)
			}
		}

		// Pot splits for this period
		if err := storeSplits(ctx, dbTx, periodID, monthNum, potIDs, ov.Splits); err != nil {
			return nil, err
		}

		// Calculate totals for report. No entry_type filter: close and every
		// list endpoint total a period the same way, and filtering here made
		// the report disagree with what the app shows for any month that
		// received a carryover during this same run.
		var incTotal, expTotal int64
		if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=$1`, periodID).Scan(&incTotal); err != nil {
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
			closed, err := closePeriod(ctx, dbTx, periodID, opts.Year, monthNum)
			if err != nil {
				return nil, err
			}
			mr.Closed = closed
		}

		rep.Months = append(rep.Months, mr)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &rep, nil
}

func firstOfMonth(year, month int) string {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func incomeLabels(rows []IncomeRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Label)
	}
	return out
}

func incomeTxLabels(rows []IncomeTxRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.SourceLabel)
	}
	return out
}

func txLabels(rows []TxRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.CategoryLabel)
	}
	return out
}

// idsFor resolves labels to their masterdata ids, deduplicated.
func idsFor(ids map[string]int64, labels []string) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, label := range labels {
		id, ok := ids[label]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func ensureIncomeSource(ctx context.Context, dbTx dbtx, srcIDs map[string]int64, monthNum int, label string, defaultCents int64) error {
	if _, exists := srcIDs[label]; exists {
		return nil
	}
	var id int64
	err := dbTx.QueryRow(ctx, `SELECT id FROM income_sources WHERE name=$1`, label).Scan(&id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("month %d: lookup income source %q: %w", monthNum, label, err)
		}
		if err := dbTx.QueryRow(ctx,
			`INSERT INTO income_sources (name, default_amount_cents, sort_order) VALUES ($1, $2, $3) RETURNING id`,
			label, defaultCents, len(srcIDs)).Scan(&id); err != nil {
			return fmt.Errorf("month %d: create income source %q: %w", monthNum, label, err)
		}
	}
	srcIDs[label] = id
	return nil
}

// storeSplits replaces a period's pot splits with the sheet's. A split naming a
// pot that couldn't be resolved, or a set that doesn't total 100%, fails the
// import: LargestRemainderSplit assumes the total is already 100 and dumps the
// whole shortfall on one arbitrary pot otherwise, which is silently wrong money.
func storeSplits(ctx context.Context, dbTx dbtx, periodID int64, monthNum int, potIDs map[string]int64, splits []SplitRow) error {
	if len(splits) == 0 {
		return nil
	}
	inputs := make([]domain.PotSplitInput, 0, len(splits))
	potIDList := make([]int64, 0, len(splits))
	for _, sp := range splits {
		potID, ok := potIDs[sp.PotName]
		if !ok {
			return &ValidationError{Month: monthNum, Reference: sp.PotName,
				Message: "pot split refers to a pot that does not exist and could not be created"}
		}
		inputs = append(inputs, domain.PotSplitInput{PotID: potID, Percentage: sp.Percentage})
		potIDList = append(potIDList, potID)
	}
	if err := domain.ValidateSplits(inputs); err != nil {
		return &ValidationError{Month: monthNum,
			Message: fmt.Sprintf("pot splits must total 100%% (%v)", err)}
	}

	if _, err := dbTx.Exec(ctx,
		`DELETE FROM pot_splits WHERE period_id=$1 AND pot_id <> ALL($2)`, periodID, potIDList); err != nil {
		return fmt.Errorf("month %d: clear pot splits: %w", monthNum, err)
	}
	for i, in := range inputs {
		if _, err := dbTx.Exec(ctx,
			`INSERT INTO pot_splits (period_id, pot_id, percentage)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (period_id, pot_id) DO UPDATE SET percentage=EXCLUDED.percentage`,
			periodID, in.PotID, in.Percentage); err != nil {
			return fmt.Errorf("month %d: insert pot split %q: %w", monthNum, splits[i].PotName, err)
		}
	}
	return nil
}

// closePeriod mirrors handleClosePeriod: same guards, same allocation formula.
// Returns whether the period is closed once it's done — a period that was
// already closed is left exactly as it is.
func closePeriod(ctx context.Context, dbTx dbtx, periodID int64, year, month int) (bool, error) {
	var status string
	if err := dbTx.QueryRow(ctx, `SELECT status FROM periods WHERE id=$1 FOR UPDATE`, periodID).Scan(&status); err != nil {
		return false, fmt.Errorf("month %d: read period status: %w", month, err)
	}
	if status == "closed" {
		// pot_ledger has no uniqueness, so re-running an import with
		// --close-through would otherwise write a second allocation and a
		// second carryover for every pot and double the balances.
		slog.Info("period already closed, skipping close", "period_id", periodID, "month", month)
		return true, nil
	}

	nextMonth, nextYear := month+1, year
	if nextMonth > 12 {
		nextMonth, nextYear = 1, year+1
	}
	var nextStatus string
	if err := dbTx.QueryRow(ctx, `SELECT status FROM periods WHERE year=$1 AND month=$2`, nextYear, nextMonth).
		Scan(&nextStatus); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("month %d: read next period status: %w", month, err)
	}
	if nextStatus == "closed" {
		return false, &ValidationError{Month: month,
			Message: "cannot close: the next period is already closed"}
	}

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
		return false, err
	}
	var rawSplits []splitRow
	for rows.Next() {
		var sr splitRow
		if err := rows.Scan(&sr.PotID, &sr.Pct, &sr.Kind); err != nil {
			rows.Close()
			return false, err
		}
		rawSplits = append(rawSplits, sr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	if len(rawSplits) == 0 {
		slog.Info("no splits, skipping close", "period_id", periodID)
		return false, nil
	}

	inputs := make([]domain.PotSplitInput, len(rawSplits))
	for i, sr := range rawSplits {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
	}
	if err := domain.ValidateSplits(inputs); err != nil {
		return false, &ValidationError{Month: month,
			Message: fmt.Sprintf("cannot close: pot splits must total 100%% (%v)", err)}
	}

	var incTotal, expTotal int64
	if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=$1`, periodID).Scan(&incTotal); err != nil {
		return false, err
	}
	if err := dbTx.QueryRow(ctx, `SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=$1`, periodID).Scan(&expTotal); err != nil {
		return false, err
	}
	surplus := incTotal - expTotal

	allocs := domain.LargestRemainderSplit(surplus, inputs)
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	for i, a := range allocs {
		if _, err := dbTx.Exec(ctx,
			`INSERT INTO pot_ledger (pot_id, period_id, source_period_id, entry_type, amount_cents, description, entry_date)
			 VALUES ($1, $2, $2, 'allocation', $3, 'Maandelijkse allocatie', $4)`,
			a.PotID, periodID, a.AmountCents, lastDay); err != nil {
			return false, err
		}

		if rawSplits[i].Kind == "carryover" {
			if _, err := dbTx.Exec(ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, nextYear); err != nil {
				return false, err
			}
			var nextPeriodID int64
			if err := dbTx.QueryRow(ctx,
				`INSERT INTO periods (year, month) VALUES ($1, $2)
				 ON CONFLICT (year, month) DO UPDATE SET year=EXCLUDED.year
				 RETURNING id`,
				nextYear, nextMonth).Scan(&nextPeriodID); err != nil {
				return false, err
			}

			if _, err := dbTx.Exec(ctx,
				`INSERT INTO pot_ledger (pot_id, period_id, source_period_id, entry_type, amount_cents, description, entry_date)
				 VALUES ($1, $2, $3, 'carryover_out', $4, 'Doorlopen', $5)`,
				a.PotID, nextPeriodID, periodID, -a.AmountCents, lastDay); err != nil {
				return false, err
			}

			monthNames := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni",
				"Juli", "Augustus", "September", "Oktober", "November", "December"}
			label := "Doorlopen maand " + monthNames[month]
			if _, err := dbTx.Exec(ctx,
				`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, source_period_id, notes, sort_order)
				 VALUES ($1, NULL, $2, $3, 'carryover', $4, '', 0)`,
				nextPeriodID, label, a.AmountCents, periodID); err != nil {
				return false, err
			}
		}
	}

	if _, err := dbTx.Exec(ctx, `UPDATE periods SET status='closed', closed_at=now() WHERE id=$1`, periodID); err != nil {
		return false, err
	}
	return true, nil
}

// wipe deletes every period for the year — pot_ledger, pot_splits,
// transactions, budget_lines and income_entries all reference periods(id)
// ON DELETE CASCADE (migration 0001), so one statement is enough.
func wipe(ctx context.Context, dbTx dbtx, year int) error {
	_, err := dbTx.Exec(ctx, `DELETE FROM periods WHERE year=$1`, year)
	return err
}

// resetCountTables are counted before the wipe so the audit record can state the
// real blast radius: nothing here is scoped to a year, which the audit row
// previously implied, and the four explicit deletes cascade into nine further
// tables. Untouched by a reset: kids, kid_savings_ledger, years, locked_years,
// users, audit_log, audit_log_export_state, sessions.
var resetCountTables = []string{
	"periods", "income_entries", "income_transactions", "budget_lines",
	"transactions", "pot_splits", "pot_ledger", "pots", "categories",
	"income_sources", "category_aliases", "category_description_presets",
	"income_source_description_presets",
}

// resetMasterdata wipes all periods (cascading to every period-scoped table:
// income_entries, income_transactions, budget_lines, transactions, pot_splits,
// pot_ledger) plus the masterdata tables themselves, for every year — the
// year the import was requested for does not scope it. pgx v5's extended query
// protocol rejects multiple semicolon-separated statements in one Exec, so each
// DELETE runs as its own statement; the caller's transaction (Run's dbTx) makes
// a failure partway through roll back everything, not leave masterdata
// half-wiped.
func resetMasterdata(ctx context.Context, dbTx dbtx) (map[string]int64, error) {
	counts := make(map[string]int64, len(resetCountTables))
	for _, table := range resetCountTables {
		var n int64
		if err := dbTx.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = n
	}
	for _, table := range []string{"periods", "pots", "categories", "income_sources"} {
		if _, err := dbTx.Exec(ctx, `DELETE FROM `+table); err != nil {
			return nil, fmt.Errorf("delete %s: %w", table, err)
		}
	}
	return counts, nil
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
