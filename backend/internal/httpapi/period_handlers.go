package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wouterdamman/home-finance/internal/domain"
)

const pgUniqueViolation = "23505"
const pgForeignKeyViolation = "23503"

// ── Periods ──────────────────────────────────────────────────────

// querier is a common interface for both *pgxpool.Pool and pgx.Tx
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

// nextPeriodOf returns the calendar month following (year, month).
func nextPeriodOf(year, month int) (int, int) {
	if month >= 12 {
		return year + 1, 1
	}
	return year, month + 1
}

// findTemplateSourcePeriod finds the most recent period with data that can be used as a template.
// It excludes the destination period (excludeID) to avoid selecting a pre-created empty next period as its own source.
func findTemplateSourcePeriod(ctx context.Context, q querier, year, month, excludeID int64) (int64, bool) {
	var srcID int64
	err := q.QueryRow(ctx, `
		SELECT id FROM periods
		WHERE id != $1
		AND (year < $2 OR (year = $2 AND month < $3))
		AND (
			EXISTS (SELECT 1 FROM income_entries WHERE period_id=periods.id AND entry_type='normal')
			OR EXISTS (SELECT 1 FROM budget_lines WHERE period_id=periods.id)
		)
		ORDER BY year DESC, month DESC LIMIT 1`,
		excludeID, year, month).Scan(&srcID)
	if err == nil {
		return srcID, true
	}
	return 0, false
}

// copyPeriodTemplate copies budget lines, pot splits, income entries, and income transactions
// from a source period to a destination period, respecting template-only flags.
func (s *Server) copyPeriodTemplate(ctx context.Context, q querier, destPeriodID, srcPeriodID int64, destYear, destMonth int) error {
	// Copy budget lines (respecting include_in_template flag on categories)
	if _, err := q.Exec(ctx, `
		INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order)
		SELECT $1,bl.category_id,bl.label,CASE WHEN COALESCE(c.autofill_actual,false) THEN COALESCE(c.default_amount_cents,0) ELSE 0 END,COALESCE(c.is_itemized,false),bl.sort_order
		FROM budget_lines bl
		LEFT JOIN categories c ON c.id = bl.category_id
		WHERE bl.period_id=$2
		AND (bl.category_id IS NULL OR bl.category_id IN (SELECT id FROM categories WHERE include_in_template=true))`,
		destPeriodID, srcPeriodID); err != nil {
		return err
	}

	// Copy pot splits
	if _, err := q.Exec(ctx, `INSERT INTO pot_splits (period_id,pot_id,percentage) SELECT $1,pot_id,percentage FROM pot_splits WHERE period_id=$2`, destPeriodID, srcPeriodID); err != nil {
		return err
	}

	// Copy itemized expense transactions (for categories marked itemized, autofill_actual, and include_in_template)
	if _, err := q.Exec(ctx, `
		INSERT INTO transactions (period_id,category_id,amount_cents,description,tx_date)
		SELECT $1,t.category_id,t.amount_cents,t.description,make_date($3,$4,1)
		FROM transactions t
		WHERE t.period_id=$2
		AND t.category_id IN (SELECT id FROM categories WHERE include_in_template=true AND is_itemized=true AND autofill_actual=true)`,
		destPeriodID, srcPeriodID, destYear, destMonth); err != nil {
		return err
	}

	// Copy normal income entries (respecting include_in_template flag on income sources)
	if _, err := q.Exec(ctx, `
		INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order)
		SELECT $1,ie.source_id,ie.label,ie.amount_cents,'normal',ie.notes,ie.sort_order
		FROM income_entries ie
		WHERE ie.period_id=$2 AND ie.entry_type='normal'
		AND (ie.source_id IS NULL OR ie.source_id IN (SELECT id FROM income_sources WHERE include_in_template=true))`,
		destPeriodID, srcPeriodID); err != nil {
		return err
	}

	// Copy itemized income transactions (for sources marked as itemized, autofill_actual, and include_in_template)
	if _, err := q.Exec(ctx, `
		INSERT INTO income_transactions (period_id,source_id,amount_cents,description,tx_date)
		SELECT $1,it.source_id,it.amount_cents,it.description,make_date($3,$4,1)
		FROM income_transactions it
		WHERE it.period_id=$2
		AND it.source_id IN (SELECT id FROM income_sources WHERE include_in_template=true AND is_itemized=true AND autofill_actual=true)`,
		destPeriodID, srcPeriodID, destYear, destMonth); err != nil {
		return err
	}

	return nil
}

func (s *Server) handleListPeriods(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	type row struct {
		ID                int64   `json:"id"`
		Year              int     `json:"year"`
		Month             int     `json:"month"`
		Status            string  `json:"status"`
		ClosedAt          *string `json:"closedAt,omitempty"`
		IncomeTotalCents  int64   `json:"incomeTotalCents"`
		ExpenseTotalCents int64   `json:"expenseTotalCents"`
		SurplusCents      int64   `json:"surplusCents"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id, p.year, p.month, p.status, p.closed_at,
		  (SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id = ie.source_id WHERE ie.period_id = p.id),
		  (SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id = p.id)
		FROM periods p WHERE p.year = $1 ORDER BY p.month`, year)
	if err != nil {
		dbError(w, "handleListPeriods", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ca *time.Time
		if err := rows.Scan(&ro.ID, &ro.Year, &ro.Month, &ro.Status, &ca, &ro.IncomeTotalCents, &ro.ExpenseTotalCents); err != nil {
			dbError(w, "handleListPeriods scan", err)
			return
		}
		if ca != nil {
			s := ca.Format(time.RFC3339)
			ro.ClosedAt = &s
		}
		ro.SurplusCents = ro.IncomeTotalCents - ro.ExpenseTotalCents
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListPeriods", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreatePeriod(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year             int    `json:"year"`
		Month            int    `json:"month"`
		CopyFromPeriodID *int64 `json:"copyFromPeriodId"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	ctx := r.Context()
	// Creating a period is open to any signed-in user while the years registry
	// itself is admin-gated, so validate before touching either — an unbounded
	// year here was a way around that gate.
	if body.Year < 2000 || body.Year > 2100 {
		Error(w, http.StatusBadRequest, "bad_request", "year out of range")
		return
	}
	if body.Month < 1 || body.Month > 12 {
		Error(w, http.StatusBadRequest, "bad_request", "month out of range")
		return
	}

	if locked, err := isYearLocked(ctx, s.pool, body.Year); err != nil {
		dbError(w, "handleCreatePeriod year lock", err)
		return
	} else if locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}

	if body.CopyFromPeriodID == nil {
		if srcID, found := findTemplateSourcePeriod(ctx, s.pool, int64(body.Year), int64(body.Month), 0); found {
			body.CopyFromPeriodID = &srcID
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, body.Year); err != nil {
		dbError(w, "handleCreatePeriod years", err)
		return
	}

	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO periods (year, month) VALUES ($1, $2) RETURNING id`, body.Year, body.Month).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			Error(w, http.StatusConflict, "already_exists", "period already exists for that month")
			return
		}
		dbError(w, "handleCreatePeriod", err)
		return
	}
	if body.CopyFromPeriodID != nil {
		src := *body.CopyFromPeriodID
		if err := s.copyPeriodTemplate(ctx, tx, id, src, body.Year, body.Month); err != nil {
			dbError(w, "handleCreatePeriod", err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "commit failed")
		return
	}
	type out struct {
		ID     int64  `json:"id"`
		Year   int    `json:"year"`
		Month  int    `json:"month"`
		Status string `json:"status"`
	}
	var o out
	if err := s.pool.QueryRow(ctx, `SELECT id,year,month,status FROM periods WHERE id=$1`, id).Scan(&o.ID, &o.Year, &o.Month, &o.Status); err != nil {
		dbError(w, "handleCreatePeriod", err)
		return
	}
	JSON(w, http.StatusCreated, o)
}

func (s *Server) handleGetPeriodOverview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	ctx := r.Context()

	type period struct {
		ID       int64   `json:"id"`
		Year     int     `json:"year"`
		Month    int     `json:"month"`
		Status   string  `json:"status"`
		ClosedAt *string `json:"closedAt,omitempty"`
	}
	var p period
	var ca *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT id,year,month,status,closed_at FROM periods WHERE id=$1`, id).
		Scan(&p.ID, &p.Year, &p.Month, &p.Status, &ca); err != nil {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	if ca != nil {
		ts := ca.Format(time.RFC3339)
		p.ClosedAt = &ts
	}

	type income struct {
		ID                     int64   `json:"id"`
		SourceID               *int64  `json:"sourceId,omitempty"`
		Label                  *string `json:"label,omitempty"`
		AmountCents            int64   `json:"amountCents"`
		EntryType              string  `json:"entryType"`
		Notes                  string  `json:"notes"`
		SortOrder              int     `json:"sortOrder"`
		IsItemized             bool    `json:"isItemized"`
		TransactionsTotalCents int64   `json:"transactionsTotalCents"`
		EffectiveCents         int64   `json:"effectiveCents"`
	}
	incRows, err := s.pool.Query(ctx, `
		SELECT ie.id, ie.source_id, ie.label, ie.amount_cents, ie.entry_type, ie.notes, ie.sort_order,
		  COALESCE(isrc.is_itemized,false),
		  COALESCE((SELECT SUM(it.amount_cents) FROM income_transactions it WHERE it.period_id=ie.period_id AND it.source_id=ie.source_id),0)
		FROM income_entries ie
		LEFT JOIN income_sources isrc ON isrc.id = ie.source_id
		WHERE ie.period_id=$1 ORDER BY ie.sort_order,ie.id`, id)
	if err != nil {
		dbError(w, "handleGetPeriodOverview", err)
		return
	}
	incomes := make([]income, 0)
	var incomeTotal int64
	for incRows.Next() {
		var e income
		if err := incRows.Scan(&e.ID, &e.SourceID, &e.Label, &e.AmountCents, &e.EntryType, &e.Notes, &e.SortOrder, &e.IsItemized, &e.TransactionsTotalCents); err != nil {
			incRows.Close()
			dbError(w, "handleGetPeriodOverview scan", err)
			return
		}
		if e.IsItemized {
			e.EffectiveCents = e.TransactionsTotalCents
		} else {
			e.EffectiveCents = e.AmountCents
		}
		incomeTotal += e.EffectiveCents
		incomes = append(incomes, e)
	}
	incRows.Close()
	// The totals are accumulated in-loop: a mid-stream failure would otherwise
	// be served as a smaller-but-plausible income total under a 200.
	if err := incRows.Err(); err != nil {
		dbError(w, "handleGetPeriodOverview incomes", err)
		return
	}

	type budgetLine struct {
		ID                     int64   `json:"id"`
		CategoryID             *int64  `json:"categoryId,omitempty"`
		Label                  *string `json:"label,omitempty"`
		AmountCents            int64   `json:"amountCents"`
		TracksTransactions     bool    `json:"tracksTransactions"`
		TransactionsTotalCents int64   `json:"transactionsTotalCents"`
		EffectiveCents         int64   `json:"effectiveCents"`
		SortOrder              int     `json:"sortOrder"`
		TargetCents            int64   `json:"targetCents"`
	}
	blRows, err := s.pool.Query(ctx, `
		SELECT bl.id, bl.category_id, bl.label, bl.amount_cents, bl.tracks_transactions, bl.sort_order,
		  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t JOIN category_rollup cr ON cr.member_id=t.category_id WHERE t.period_id=bl.period_id AND cr.category_id=bl.category_id),0),
		  COALESCE(c.default_amount_cents,0), bl.target_cents_at_close
		FROM budget_lines bl LEFT JOIN categories c ON c.id = bl.category_id WHERE bl.period_id=$1 ORDER BY bl.sort_order,bl.id`, id)
	if err != nil {
		dbError(w, "handleGetPeriodOverview", err)
		return
	}
	lines := make([]budgetLine, 0)
	var expenseTotal int64
	for blRows.Next() {
		var bl budgetLine
		var defaultAmountCents int64
		var targetCentsAtClose *int64
		if err := blRows.Scan(&bl.ID, &bl.CategoryID, &bl.Label, &bl.AmountCents, &bl.TracksTransactions, &bl.SortOrder, &bl.TransactionsTotalCents, &defaultAmountCents, &targetCentsAtClose); err != nil {
			blRows.Close()
			dbError(w, "handleGetPeriodOverview scan", err)
			return
		}
		if p.Status == "closed" && targetCentsAtClose != nil {
			bl.TargetCents = *targetCentsAtClose
		} else {
			bl.TargetCents = defaultAmountCents
		}
		if bl.TracksTransactions {
			bl.EffectiveCents = bl.TransactionsTotalCents
		} else {
			bl.EffectiveCents = bl.AmountCents
		}
		expenseTotal += bl.EffectiveCents
		lines = append(lines, bl)
	}
	blRows.Close()
	if err := blRows.Err(); err != nil {
		dbError(w, "handleGetPeriodOverview budget lines", err)
		return
	}

	surplus := incomeTotal - expenseTotal

	type split struct {
		PotID          int64  `json:"potId"`
		PotName        string `json:"potName"`
		PotKind        string `json:"potKind"`
		Percentage     string `json:"percentage"`
		ProjectedCents int64  `json:"projectedCents"`
	}
	spRows, err := s.pool.Query(ctx, `
		SELECT ps.pot_id, p.name, p.kind, ps.percentage
		FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id
		WHERE ps.period_id=$1 ORDER BY p.sort_order,p.id`, id)
	if err != nil {
		dbError(w, "handleGetPeriodOverview", err)
		return
	}
	splits := make([]split, 0)
	var splitPctTotal float64
	var splitInputs []domain.PotSplitInput
	for spRows.Next() {
		var sp split
		var pct float64
		if err := spRows.Scan(&sp.PotID, &sp.PotName, &sp.PotKind, &pct); err != nil {
			spRows.Close()
			dbError(w, "handleGetPeriodOverview scan", err)
			return
		}
		sp.Percentage = strconv.FormatFloat(pct, 'f', 2, 64)
		splitPctTotal += pct
		splitInputs = append(splitInputs, domain.PotSplitInput{PotID: sp.PotID, Percentage: pct})
		splits = append(splits, sp)
	}
	spRows.Close()
	if err := spRows.Err(); err != nil {
		dbError(w, "handleGetPeriodOverview splits", err)
		return
	}
	allocs := domain.LargestRemainderSplit(surplus, splitInputs)
	for i := range splits {
		if i < len(allocs) {
			splits[i].ProjectedCents = allocs[i].AmountCents
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"period":               p,
		"incomes":              incomes,
		"incomeTotalCents":     incomeTotal,
		"budgetLines":          lines,
		"expenseTotalCents":    expenseTotal,
		"surplusCents":         surplus,
		"splits":               splits,
		"splitPercentageTotal": strconv.FormatFloat(splitPctTotal, 'f', 2, 64),
	})
}

func (s *Server) handleClosePeriod(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	ctx := r.Context()

	var preYear, preMonth int
	var preStatus string
	if err := s.pool.QueryRow(ctx, `SELECT year, month, status FROM periods WHERE id=$1`, id).
		Scan(&preYear, &preMonth, &preStatus); err != nil {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	if locked, err := isYearLocked(ctx, s.pool, preYear); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	} else if locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}
	nextYear, nextMonth := nextPeriodOf(preYear, preMonth)
	var nextStatus string
	if err := s.pool.QueryRow(ctx, `SELECT status FROM periods WHERE year=$1 AND month=$2`, nextYear, nextMonth).
		Scan(&nextStatus); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, "handleClosePeriod", err)
		return
	}
	if nextStatus == "closed" {
		Error(w, http.StatusConflict, "next_period_closed", "cannot close: next period is already closed")
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)

	var status string
	var periodYear, periodMonth int
	if err := tx.QueryRow(ctx, `SELECT status, year, month FROM periods WHERE id=$1 FOR UPDATE`, id).
		Scan(&status, &periodYear, &periodMonth); err != nil {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	if status == "closed" {
		Error(w, http.StatusConflict, "period_already_closed", "already closed")
		return
	}
	carryYear, carryMonth := nextPeriodOf(periodYear, periodMonth)

	var incomeTotal, expenseTotal int64
	if err := tx.QueryRow(ctx, `SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=$1`, id).Scan(&incomeTotal); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	if err := tx.QueryRow(ctx, `SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=$1`, id).Scan(&expenseTotal); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	surplus := incomeTotal - expenseTotal

	type splitRow struct {
		PotID int64
		Pct   float64
		Kind  string
	}
	spRows, err := tx.Query(ctx, `SELECT ps.pot_id, ps.percentage, p.kind FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id WHERE ps.period_id=$1`, id)
	if err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	var rawSplits []splitRow
	for spRows.Next() {
		var sr splitRow
		if err := spRows.Scan(&sr.PotID, &sr.Pct, &sr.Kind); err != nil {
			spRows.Close()
			dbError(w, "handleClosePeriod", err)
			return
		}
		rawSplits = append(rawSplits, sr)
	}
	spRows.Close()
	if err := spRows.Err(); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}

	inputs := make([]domain.PotSplitInput, len(rawSplits))
	hasCarryoverSplit := false
	for i, sr := range rawSplits {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
		if sr.Kind == "carryover" {
			hasCarryoverSplit = true
		}
	}

	// A December close writes its carryover into January of the *next* year,
	// which locks independently of this one — a year can be locked while it
	// still has no periods at all, and the check above only covered this
	// period's own year.
	if hasCarryoverSplit && carryYear != periodYear {
		var nextYearLocked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM locked_years WHERE year=$1)`, carryYear).Scan(&nextYearLocked); err != nil {
			dbError(w, "handleClosePeriod next year lock", err)
			return
		}
		if nextYearLocked {
			Error(w, http.StatusConflict, "year_locked", "cannot close: the next year is locked")
			return
		}
	}

	allocs := domain.LargestRemainderSplit(surplus, inputs)

	today := time.Now().Format("2006-01-02")

	for i, a := range allocs {
		_, err = tx.Exec(ctx,
			`INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date)
			 VALUES ($1,$2,$2,'allocation',$3,'Monthly allocation',$4)`,
			a.PotID, id, a.AmountCents, today)
		if err != nil {
			dbError(w, "handleClosePeriod", err)
			return
		}
		if rawSplits[i].Kind == "carryover" {
			// The sidebar's year list reads the years registry, not the
			// periods table — without this a December close creates a January
			// period nobody can navigate to.
			if carryYear != periodYear {
				if _, err := tx.Exec(ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, carryYear); err != nil {
					dbError(w, "handleClosePeriod years", err)
					return
				}
			}
			var nextPeriodID int64
			if err := tx.QueryRow(ctx, `INSERT INTO periods (year,month) VALUES ($1,$2) ON CONFLICT (year,month) DO UPDATE SET year=EXCLUDED.year RETURNING id`, carryYear, carryMonth).Scan(&nextPeriodID); err != nil {
				dbError(w, "handleClosePeriod", err)
				return
			}

			// Template copy: only if next period has no budget_lines yet (idempotency)
			var hasBudgetLines bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM budget_lines WHERE period_id=$1)`, nextPeriodID).Scan(&hasBudgetLines); err != nil {
				dbError(w, "handleClosePeriod", err)
				return
			}
			if !hasBudgetLines {
				if srcID, found := findTemplateSourcePeriod(ctx, tx, int64(carryYear), int64(carryMonth), nextPeriodID); found {
					if err := s.copyPeriodTemplate(ctx, tx, nextPeriodID, srcID, carryYear, carryMonth); err != nil {
						dbError(w, "handleClosePeriod", err)
						return
					}
				}
			}

			if _, err := tx.Exec(ctx, `INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,'carryover_out',$4,'Carryover out',$5)`,
				a.PotID, nextPeriodID, id, -a.AmountCents, today); err != nil {
				dbError(w, "handleClosePeriod", err)
				return
			}
			months := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni", "Juli", "Augustus", "September", "Oktober", "November", "December"}
			monthName := ""
			if periodMonth >= 1 && periodMonth <= 12 {
				monthName = months[periodMonth]
			}
			if _, err := tx.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,source_period_id,notes,sort_order) VALUES ($1,NULL,$2,$3,'carryover',$4,'',0)`,
				nextPeriodID, "Doorlopen maand "+monthName, a.AmountCents, id); err != nil {
				dbError(w, "handleClosePeriod", err)
				return
			}
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE budget_lines SET target_cents_at_close = COALESCE((SELECT default_amount_cents FROM categories WHERE id = budget_lines.category_id), 0) WHERE period_id = $1`, id); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE periods SET status='closed', closed_at=now() WHERE id=$1`, id); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "handleClosePeriod", err)
		return
	}
	s.auditLog(ctx, "period.close", "period", id, map[string]any{"year": periodYear, "month": periodMonth, "surplusCents": surplus})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReopenPeriod(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	ctx := r.Context()

	var year, month int
	var currentStatus string
	if err := s.pool.QueryRow(ctx, `SELECT year,month,status FROM periods WHERE id=$1`, id).Scan(&year, &month, &currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "period not found")
			return
		}
		dbError(w, "handleReopenPeriod", err)
		return
	}
	if currentStatus == "open" {
		Error(w, http.StatusConflict, "period_already_open", "period is already open")
		return
	}
	if locked, err := isYearLocked(ctx, s.pool, year); err != nil {
		dbError(w, "handleReopenPeriod year lock", err)
		return
	} else if locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}
	nextYear, nextMonth := nextPeriodOf(year, month)
	var nextStatus string
	// A swallowed error here reads as an empty status, which passes the guard
	// below and goes on to delete the next period's carryover rows.
	if err := s.pool.QueryRow(ctx, `SELECT status FROM periods WHERE year=$1 AND month=$2`, nextYear, nextMonth).
		Scan(&nextStatus); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		dbError(w, "handleReopenPeriod next period", err)
		return
	}
	if nextStatus == "closed" {
		Error(w, http.StatusConflict, "next_period_closed", "cannot reopen: next period is closed")
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)
	// Re-read the status under a row lock: the check above ran on the pool,
	// so a close that committed in between would otherwise have its freshly
	// written allocation rows deleted while the period stays marked closed —
	// or, the other way round, leave this period open with them still in place.
	if err := tx.QueryRow(ctx, `SELECT status FROM periods WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "period not found")
			return
		}
		dbError(w, "handleReopenPeriod lock", err)
		return
	}
	if currentStatus == "open" {
		Error(w, http.StatusConflict, "period_already_open", "period is already open")
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM pot_ledger WHERE source_period_id=$1`, id); err != nil {
		dbError(w, "handleReopenPeriod", err)
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM income_entries WHERE entry_type='carryover' AND source_period_id=$1`, id); err != nil {
		dbError(w, "handleReopenPeriod", err)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE budget_lines SET target_cents_at_close = NULL WHERE period_id = $1`, id); err != nil {
		dbError(w, "handleReopenPeriod", err)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE periods SET status='open', closed_at=NULL WHERE id=$1`, id); err != nil {
		dbError(w, "handleReopenPeriod", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "handleReopenPeriod", err)
		return
	}
	s.auditLog(ctx, "period.reopen", "period", id, map[string]any{"year": year, "month": month})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeletePeriod(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !s.requireFreshReauth(w, r) {
		return
	}

	ctx := r.Context()
	var year, month int
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT year,month,status FROM periods WHERE id=$1`, id).Scan(&year, &month, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "period not found")
			return
		}
		dbError(w, "handleDeletePeriod", err)
		return
	}
	if status == "closed" {
		Error(w, http.StatusConflict, "period_closed", "cannot delete a closed period; reopen it first")
		return
	}
	if locked, err := isYearLocked(ctx, s.pool, year); err != nil {
		dbError(w, "handleDeletePeriod year lock", err)
		return
	} else if locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)
	// Re-read under a row lock — the status check above ran on the pool and a
	// close committing in between would otherwise have its period deleted out
	// from under it.
	if err := tx.QueryRow(ctx, `SELECT status FROM periods WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "period not found")
			return
		}
		dbError(w, "handleDeletePeriod lock", err)
		return
	}
	if status == "closed" {
		Error(w, http.StatusConflict, "period_closed", "cannot delete a closed period; reopen it first")
		return
	}

	// A closed predecessor writes its carryover into this period as an income
	// entry plus a carryover_out ledger row. Both are scoped to this period and
	// cascade away with it, but the matching allocation row is scoped to the
	// predecessor and survives — leaving the carryover pot holding money that
	// no longer exists as income anywhere. Reopening the predecessor first
	// unwinds all three rows together.
	var hasCarryoverIn bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM income_entries WHERE period_id=$1 AND entry_type='carryover')`, id).Scan(&hasCarryoverIn); err != nil {
		dbError(w, "handleDeletePeriod carryover", err)
		return
	}
	if hasCarryoverIn {
		Error(w, http.StatusConflict, "carryover_present", "cannot delete: the previous period's carryover lands here; reopen the previous period first")
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM periods WHERE id=$1`, id); err != nil {
		dbError(w, "handleDeletePeriod", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "handleDeletePeriod commit", err)
		return
	}
	s.auditLog(ctx, "period.delete", "period", id, map[string]any{"year": year, "month": month})
	w.WriteHeader(http.StatusNoContent)
}

// handleListAuditLog supports keyset pagination (before=<id>, walking
// backwards through id DESC — id order tracks created_at order since both
// are monotonic on insert, and it sidesteps created_at tie-breaking) plus
// optional filters. All filters are exact match except the date range,
// which are applied server-side so the audit-log UI never has to page
// through everything client-side to find a narrow slice.
func (s *Server) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID         int64   `json:"id"`
		CreatedAt  string  `json:"createdAt"`
		UserEmail  *string `json:"userEmail,omitempty"`
		Action     string  `json:"action"`
		EntityType *string `json:"entityType,omitempty"`
		EntityID   *int64  `json:"entityId,omitempty"`
		Details    *string `json:"details,omitempty"`
	}

	q := r.URL.Query()
	limit := 50
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}

	conds := []string{}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if v := q.Get("before"); v != "" {
		if beforeID, err := strconv.ParseInt(v, 10, 64); err == nil {
			conds = append(conds, "id < "+arg(beforeID))
		}
	}
	if v := q.Get("action"); v != "" {
		conds = append(conds, "action = "+arg(v))
	}
	if v := q.Get("entityType"); v != "" {
		conds = append(conds, "entity_type = "+arg(v))
	}
	if v := q.Get("userEmail"); v != "" {
		conds = append(conds, "user_email = "+arg(v))
	}
	// Validate the date bounds in Go: passed straight through they reach
	// Postgres as an untyped string and anything unparseable comes back as a
	// raw cast error under a 500 instead of a 400.
	if v := q.Get("from"); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "invalid from date, expected YYYY-MM-DD")
			return
		}
		conds = append(conds, "created_at >= "+arg(v)+"::date")
	}
	if v := q.Get("to"); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "invalid to date, expected YYYY-MM-DD")
			return
		}
		conds = append(conds, "created_at < ("+arg(v)+"::date + interval '1 day')")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT id, created_at, user_email, action, entity_type, entity_id, details::text
		FROM audit_log %s ORDER BY id DESC LIMIT $%d`, where, len(args))

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		dbError(w, "handleListAuditLog", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ts time.Time
		if err := rows.Scan(&ro.ID, &ts, &ro.UserEmail, &ro.Action, &ro.EntityType, &ro.EntityID, &ro.Details); err != nil {
			dbError(w, "handleListAuditLog scan", err)
			return
		}
		ro.CreatedAt = ts.Format(time.RFC3339)
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListAuditLog", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// ── Income entries ───────────────────────────────────────────────

func (s *Server) handleCreateIncomeEntry(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	var body struct {
		SourceID    *int64  `json:"sourceId"`
		Label       *string `json:"label"`
		AmountCents int64   `json:"amountCents"`
		Notes       string  `json:"notes"`
		SortOrder   int     `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order) VALUES ($1,$2,$3,$4,'normal',$5,$6) RETURNING id`,
		periodID, body.SourceID, body.Label, body.AmountCents, body.Notes, body.SortOrder).Scan(&id); err != nil {
		dbError(w, "handleCreateIncomeEntry", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "periodId": periodID, "sourceId": body.SourceID, "label": body.Label, "amountCents": body.AmountCents, "entryType": "normal", "notes": body.Notes, "sortOrder": body.SortOrder})
}

func (s *Server) handleUpdateIncomeEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Label       *string `json:"label"`
		AmountCents int64   `json:"amountCents"`
		Notes       string  `json:"notes"`
		SortOrder   int     `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var periodID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id FROM income_entries WHERE id=$1`, id).Scan(&periodID); err != nil {
		Error(w, http.StatusNotFound, "not_found", "income entry not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE income_entries SET label=$2,amount_cents=$3,notes=$4,sort_order=$5 WHERE id=$1`, id, body.Label, body.AmountCents, body.Notes, body.SortOrder); err != nil {
		dbError(w, "handleUpdateIncomeEntry", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteIncomeEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var periodID, amountCents int64
	var sourceID *int64
	var label *string
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id, source_id, amount_cents, label FROM income_entries WHERE id=$1`, id).Scan(&periodID, &sourceID, &amountCents, &label); err != nil {
		Error(w, http.StatusNotFound, "not_found", "income entry not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if amountCents != 0 {
		Error(w, http.StatusConflict, "conflict", "income entry has a non-zero amount")
		return
	}
	if sourceID != nil {
		var txCount int64
		if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM income_transactions WHERE period_id=$1 AND source_id=$2`, periodID, *sourceID).Scan(&txCount); err != nil {
			dbError(w, "handleDeleteIncomeEntry", err)
			return
		}
		if txCount > 0 {
			Error(w, http.StatusConflict, "conflict", "income entry has transactions")
			return
		}
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM income_entries WHERE id=$1`, id); err != nil {
		dbError(w, "handleDeleteIncomeEntry", err)
		return
	}
	s.auditLog(r.Context(), "income_entry.delete", "income_entry", id, map[string]any{"periodId": periodID, "sourceId": sourceID, "label": label, "amountCents": amountCents})
	w.WriteHeader(http.StatusNoContent)
}

// ── Budget lines ─────────────────────────────────────────────────

func (s *Server) handleCreateBudgetLine(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	var body struct {
		CategoryID  *int64  `json:"categoryId"`
		Label       *string `json:"label"`
		AmountCents int64   `json:"amountCents"`
		SortOrder   int     `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.CategoryID != nil {
		var hasParent bool
		if err := s.pool.QueryRow(r.Context(), `SELECT parent_id IS NOT NULL FROM categories WHERE id=$1`, *body.CategoryID).Scan(&hasParent); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "invalid category")
			return
		}
		if hasParent {
			Error(w, http.StatusBadRequest, "bad_request", "cannot create a budget line for a child category; use its parent")
			return
		}
	}
	// tracksTransactions is never client-supplied — it always mirrors the
	// category's is_itemized flag at creation time, so a month can never
	// drift out of sync with its category's Settings-defined itemized state.
	var id int64
	var tracksTransactions bool
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order)
		 VALUES ($1,$2,$3,$4,COALESCE((SELECT is_itemized FROM categories WHERE id=$2),false),$5)
		 RETURNING id, tracks_transactions`,
		periodID, body.CategoryID, body.Label, body.AmountCents, body.SortOrder).Scan(&id, &tracksTransactions); err != nil {
		dbError(w, "handleCreateBudgetLine", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "periodId": periodID, "categoryId": body.CategoryID, "label": body.Label, "amountCents": body.AmountCents, "tracksTransactions": tracksTransactions, "sortOrder": body.SortOrder})
}

func (s *Server) handleUpdateBudgetLine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Label              *string `json:"label"`
		AmountCents        int64   `json:"amountCents"`
		TracksTransactions bool    `json:"tracksTransactions"`
		SortOrder          int     `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var periodID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id FROM budget_lines WHERE id=$1`, id).Scan(&periodID); err != nil {
		Error(w, http.StatusNotFound, "not_found", "budget line not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE budget_lines SET label=$2,amount_cents=$3,tracks_transactions=$4,sort_order=$5 WHERE id=$1`, id, body.Label, body.AmountCents, body.TracksTransactions, body.SortOrder); err != nil {
		dbError(w, "handleUpdateBudgetLine", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteBudgetLine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	// category_id is nullable (label-only lines); scanning it into a plain
	// int64 made pgx error on those rows, which surfaced as a 404 and left
	// them undeletable.
	var periodID, amountCents int64
	var categoryID *int64
	var label *string
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id, category_id, amount_cents, label FROM budget_lines WHERE id=$1`, id).Scan(&periodID, &categoryID, &amountCents, &label); err != nil {
		Error(w, http.StatusNotFound, "not_found", "budget line not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if amountCents != 0 {
		Error(w, http.StatusConflict, "conflict", "budget line has a non-zero amount")
		return
	}
	if categoryID != nil {
		// Same category_rollup join the expense total uses: a child category's
		// transactions roll into its parent's line, so counting only exact
		// category_id matches let a parent line be deleted while the spend it
		// represented was booked on its children.
		var txCount int64
		if err := s.pool.QueryRow(r.Context(), `SELECT count(*) FROM transactions t JOIN category_rollup cr ON cr.member_id = t.category_id WHERE t.period_id=$1 AND cr.category_id=$2`, periodID, *categoryID).Scan(&txCount); err != nil {
			dbError(w, "handleDeleteBudgetLine", err)
			return
		}
		if txCount > 0 {
			Error(w, http.StatusConflict, "conflict", "budget line has transactions")
			return
		}
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM budget_lines WHERE id=$1`, id); err != nil {
		dbError(w, "handleDeleteBudgetLine", err)
		return
	}
	s.auditLog(r.Context(), "budget_line.delete", "budget_line", id, map[string]any{"periodId": periodID, "categoryId": categoryID, "label": label, "amountCents": amountCents})
	w.WriteHeader(http.StatusNoContent)
}

// ── Transactions ─────────────────────────────────────────────────

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var catID *int64
	if c := r.URL.Query().Get("categoryId"); c != "" {
		if v, err := strconv.ParseInt(c, 10, 64); err == nil {
			catID = &v
		}
	}
	type txRow struct {
		ID          int64   `json:"id"`
		PeriodID    int64   `json:"periodId"`
		CategoryID  int64   `json:"categoryId"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate,omitempty"`
	}
	var rows pgx.Rows
	var err error
	if catID != nil {
		// category_rollup so filtering by a parent category's id also returns
		// its children's transactions — a budget line's "view transactions"
		// drill-down should show everything that rolled into its total.
		rows, err = s.pool.Query(r.Context(), `SELECT t.id,t.period_id,t.category_id,t.amount_cents,t.description,t.tx_date FROM transactions t JOIN category_rollup cr ON cr.member_id=t.category_id WHERE t.period_id=$1 AND cr.category_id=$2 ORDER BY t.tx_date DESC NULLS LAST,t.id DESC`, periodID, *catID)
	} else {
		rows, err = s.pool.Query(r.Context(), `SELECT id,period_id,category_id,amount_cents,description,tx_date FROM transactions WHERE period_id=$1 ORDER BY tx_date DESC NULLS LAST,id DESC`, periodID)
	}
	if err != nil {
		dbError(w, "handleListTransactions", err)
		return
	}
	defer rows.Close()
	out := make([]txRow, 0)
	for rows.Next() {
		var tx txRow
		var d *time.Time
		if err := rows.Scan(&tx.ID, &tx.PeriodID, &tx.CategoryID, &tx.AmountCents, &tx.Description, &d); err != nil {
			dbError(w, "handleListTransactions scan", err)
			return
		}
		if d != nil {
			ds := d.Format("2006-01-02")
			tx.TxDate = &ds
		}
		out = append(out, tx)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListTransactions", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	var body struct {
		CategoryID  int64   `json:"categoryId"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO transactions (period_id,category_id,amount_cents,description,tx_date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		periodID, body.CategoryID, body.AmountCents, body.Description, body.TxDate).Scan(&id); err != nil {
		dbError(w, "handleCreateTransaction", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "periodId": periodID, "categoryId": body.CategoryID, "amountCents": body.AmountCents, "description": body.Description, "txDate": body.TxDate})
}

func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var periodID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id FROM transactions WHERE id=$1`, id).Scan(&periodID); err != nil {
		Error(w, http.StatusNotFound, "not_found", "transaction not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE transactions SET amount_cents=$2,description=$3,tx_date=$4 WHERE id=$1`, id, body.AmountCents, body.Description, body.TxDate); err != nil {
		dbError(w, "handleUpdateTransaction", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var periodID, categoryID, amountCents int64
	var description string
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id, category_id, amount_cents, description FROM transactions WHERE id=$1`, id).
		Scan(&periodID, &categoryID, &amountCents, &description); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "transaction not found")
			return
		}
		dbError(w, "handleDeleteTransaction", err)
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM transactions WHERE id=$1`, id); err != nil {
		dbError(w, "handleDeleteTransaction", err)
		return
	}
	s.auditLog(r.Context(), "transaction.delete", "transaction", id, map[string]any{"periodId": periodID, "categoryId": categoryID, "amountCents": amountCents, "description": description})
	w.WriteHeader(http.StatusNoContent)
}

// ── Income transactions ─────────────────────────────────────────

func (s *Server) handleListIncomeTransactions(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var sourceID *int64
	if c := r.URL.Query().Get("sourceId"); c != "" {
		if v, err := strconv.ParseInt(c, 10, 64); err == nil {
			sourceID = &v
		}
	}
	type txRow struct {
		ID          int64   `json:"id"`
		PeriodID    int64   `json:"periodId"`
		SourceID    int64   `json:"sourceId"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate,omitempty"`
	}
	var rows pgx.Rows
	var err error
	if sourceID != nil {
		rows, err = s.pool.Query(r.Context(), `SELECT id,period_id,source_id,amount_cents,description,tx_date FROM income_transactions WHERE period_id=$1 AND source_id=$2 ORDER BY tx_date DESC NULLS LAST,id DESC`, periodID, *sourceID)
	} else {
		rows, err = s.pool.Query(r.Context(), `SELECT id,period_id,source_id,amount_cents,description,tx_date FROM income_transactions WHERE period_id=$1 ORDER BY tx_date DESC NULLS LAST,id DESC`, periodID)
	}
	if err != nil {
		dbError(w, "handleListIncomeTransactions", err)
		return
	}
	defer rows.Close()
	out := make([]txRow, 0)
	for rows.Next() {
		var tx txRow
		var d *time.Time
		if err := rows.Scan(&tx.ID, &tx.PeriodID, &tx.SourceID, &tx.AmountCents, &tx.Description, &d); err != nil {
			dbError(w, "handleListIncomeTransactions scan", err)
			return
		}
		if d != nil {
			ds := d.Format("2006-01-02")
			tx.TxDate = &ds
		}
		out = append(out, tx)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListIncomeTransactions", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateIncomeTransaction(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	var body struct {
		SourceID    int64   `json:"sourceId"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO income_transactions (period_id,source_id,amount_cents,description,tx_date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		periodID, body.SourceID, body.AmountCents, body.Description, body.TxDate).Scan(&id); err != nil {
		dbError(w, "handleCreateIncomeTransaction", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "periodId": periodID, "sourceId": body.SourceID, "amountCents": body.AmountCents, "description": body.Description, "txDate": body.TxDate})
}

func (s *Server) handleUpdateIncomeTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		TxDate      *string `json:"txDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var periodID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id FROM income_transactions WHERE id=$1`, id).Scan(&periodID); err != nil {
		Error(w, http.StatusNotFound, "not_found", "transaction not found")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE income_transactions SET amount_cents=$2,description=$3,tx_date=$4 WHERE id=$1`, id, body.AmountCents, body.Description, body.TxDate); err != nil {
		dbError(w, "handleUpdateIncomeTransaction", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteIncomeTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var periodID, sourceID, amountCents int64
	var description string
	if err := s.pool.QueryRow(r.Context(), `SELECT period_id, source_id, amount_cents, description FROM income_transactions WHERE id=$1`, id).
		Scan(&periodID, &sourceID, &amountCents, &description); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "transaction not found")
			return
		}
		dbError(w, "handleDeleteIncomeTransaction", err)
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM income_transactions WHERE id=$1`, id); err != nil {
		dbError(w, "handleDeleteIncomeTransaction", err)
		return
	}
	s.auditLog(r.Context(), "income_transaction.delete", "income_transaction", id, map[string]any{"periodId": periodID, "sourceId": sourceID, "amountCents": amountCents, "description": description})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListCategoryTransactionDescriptions(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	limit := 15
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	type descRow struct {
		Description string `json:"description"`
		LastUsed    string `json:"lastUsed"`
		Count       int    `json:"count"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT description, MAX(tx_date) AS last_used, COUNT(*) AS cnt
		FROM transactions
		WHERE category_id=$1 AND description <> ''
		GROUP BY description
		ORDER BY last_used DESC NULLS LAST, cnt DESC
		LIMIT $2`, categoryID, limit)
	if err != nil {
		dbError(w, "handleListCategoryTransactionDescriptions", err)
		return
	}
	defer rows.Close()
	out := make([]descRow, 0)
	for rows.Next() {
		var dr descRow
		var lastUsed *time.Time
		if err := rows.Scan(&dr.Description, &lastUsed, &dr.Count); err != nil {
			dbError(w, "handleListCategoryTransactionDescriptions scan", err)
			return
		}
		if lastUsed != nil {
			dr.LastUsed = lastUsed.Format(time.RFC3339)
		}
		out = append(out, dr)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListCategoryTransactionDescriptions", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleListIncomeSourceTransactionDescriptions(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	limit := 15
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	type descRow struct {
		Description string `json:"description"`
		LastUsed    string `json:"lastUsed"`
		Count       int    `json:"count"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT description, MAX(tx_date) AS last_used, COUNT(*) AS cnt
		FROM income_transactions
		WHERE source_id=$1 AND description <> ''
		GROUP BY description
		ORDER BY last_used DESC NULLS LAST, cnt DESC
		LIMIT $2`, sourceID, limit)
	if err != nil {
		dbError(w, "handleListIncomeSourceTransactionDescriptions", err)
		return
	}
	defer rows.Close()
	out := make([]descRow, 0)
	for rows.Next() {
		var dr descRow
		var lastUsed *time.Time
		if err := rows.Scan(&dr.Description, &lastUsed, &dr.Count); err != nil {
			dbError(w, "handleListIncomeSourceTransactionDescriptions scan", err)
			return
		}
		if lastUsed != nil {
			dr.LastUsed = lastUsed.Format(time.RFC3339)
		}
		out = append(out, dr)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleListIncomeSourceTransactionDescriptions", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// ── Splits ───────────────────────────────────────────────────────

type splitInput struct {
	PotID      int64  `json:"potId"`
	Percentage string `json:"percentage"`
}

// parseSplitPercentage parses a client-supplied pot percentage, rejecting
// anything the allocation math can't reason about.
//
// strconv.ParseFloat accepts "NaN" and "Inf", and every guard downstream is a
// comparison — which is false for a NaN on both sides, so the "exceeds 100%"
// check, the "must total 100%" check and Postgres's own CHECK (NaN >= 0 is
// TRUE there) all wave it through. The value then reaches
// domain.LargestRemainderSplit, where int64(NaN) is the most negative int64
// and the leftover it computes wraps into a loop that does not terminate —
// inside the open transaction of a period close.
func parseSplitPercentage(raw string) (float64, error) {
	pct, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New("percentage is not a number")
	}
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return 0, errors.New("percentage must be a finite number")
	}
	if pct < 0 || pct > 100 {
		return 0, errors.New("percentage must be between 0 and 100")
	}
	return pct, nil
}

func (s *Server) handleReplaceSplits(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	var body struct {
		Splits []splitInput `json:"splits"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	ctx := r.Context()

	// The carryover pot always absorbs whatever percentage isn't explicitly
	// allocated to another pot — its percentage is never stored as-submitted,
	// it's recomputed here so it always keeps the total at exactly 100%.
	var carryoverPotID int64
	hasCarryover := true
	if err := s.pool.QueryRow(ctx, `SELECT id FROM pots WHERE kind='carryover' AND archived_at IS NULL`).Scan(&carryoverPotID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			// Treating a query failure as "no carryover pot" would store the
			// client's own percentage for it verbatim, breaking the
			// always-100% invariant the close algorithm depends on.
			dbError(w, "handleReplaceSplits carryover pot", err)
			return
		}
		hasCarryover = false
	}

	var otherTotal float64
	carryoverIdx := -1
	for i, sp := range body.Splits {
		pct, err := parseSplitPercentage(sp.Percentage)
		if err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "invalid percentage for pot "+strconv.FormatInt(sp.PotID, 10)+": "+err.Error())
			return
		}
		if hasCarryover && sp.PotID == carryoverPotID {
			carryoverIdx = i
			continue
		}
		otherTotal += pct
	}

	if hasCarryover {
		remainder := 100 - otherTotal
		if remainder < -0.005 {
			Error(w, http.StatusBadRequest, "bad_request", "pot percentages exceed 100%")
			return
		}
		if remainder < 0 {
			remainder = 0
		}
		remainderStr := strconv.FormatFloat(remainder, 'f', 2, 64)
		if carryoverIdx >= 0 {
			body.Splits[carryoverIdx].Percentage = remainderStr
		} else {
			body.Splits = append(body.Splits, splitInput{PotID: carryoverPotID, Percentage: remainderStr})
		}
	} else if math.Abs(otherTotal-100) > 0.01 {
		Error(w, http.StatusBadRequest, "bad_request", "pot percentages must total 100%")
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Read the splits being replaced so the audit record shows where this
	// period's surplus used to land, not just where it lands from now on.
	prevRows, err := tx.Query(ctx, `SELECT pot_id, percentage FROM pot_splits WHERE period_id=$1 ORDER BY pot_id`, periodID)
	if err != nil {
		dbError(w, "handleReplaceSplits previous", err)
		return
	}
	previous := make([]map[string]any, 0)
	for prevRows.Next() {
		var potID int64
		var pct float64
		if err := prevRows.Scan(&potID, &pct); err != nil {
			prevRows.Close()
			dbError(w, "handleReplaceSplits previous scan", err)
			return
		}
		previous = append(previous, map[string]any{"potId": potID, "percentage": pct})
	}
	prevRows.Close()
	if err := prevRows.Err(); err != nil {
		dbError(w, "handleReplaceSplits previous", err)
		return
	}

	if _, err := tx.Exec(ctx, `DELETE FROM pot_splits WHERE period_id=$1`, periodID); err != nil {
		dbError(w, "handleReplaceSplits", err)
		return
	}
	stored := make([]map[string]any, 0, len(body.Splits))
	for _, sp := range body.Splits {
		pct, err := parseSplitPercentage(sp.Percentage)
		if err != nil {
			Error(w, http.StatusBadRequest, "bad_request", "invalid percentage for pot "+strconv.FormatInt(sp.PotID, 10)+": "+err.Error())
			return
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pot_splits (period_id,pot_id,percentage) VALUES ($1,$2,$3)`, periodID, sp.PotID, pct); err != nil {
			dbError(w, "handleReplaceSplits", err)
			return
		}
		stored = append(stored, map[string]any{"potId": sp.PotID, "percentage": pct})
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "handleReplaceSplits", err)
		return
	}
	s.auditLog(ctx, "period.splits.replace", "period", periodID, map[string]any{"previous": previous, "splits": stored})
	w.WriteHeader(http.StatusNoContent)
}

// ── Year summary ─────────────────────────────────────────────────

func (s *Server) handleYearSummary(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	ctx := r.Context()

	type monthRow struct {
		Month             int     `json:"month"`
		PeriodID          *int64  `json:"periodId,omitempty"`
		Status            *string `json:"status,omitempty"`
		IncomeTotalCents  int64   `json:"incomeTotalCents"`
		ExpenseTotalCents int64   `json:"expenseTotalCents"`
		SurplusCents      int64   `json:"surplusCents"`
	}

	rows, err := s.pool.Query(ctx, `
		SELECT p.month, p.id, p.status,
		  (SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=p.id),
		  (SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=p.id)
		FROM periods p WHERE p.year=$1 ORDER BY p.month`, year)
	if err != nil {
		dbError(w, "handleYearSummary months", err)
		return
	}
	defer rows.Close()

	monthMap := map[int]monthRow{}
	var yearIncome, yearExpense int64
	for rows.Next() {
		var mr monthRow
		if err := rows.Scan(&mr.Month, &mr.PeriodID, &mr.Status, &mr.IncomeTotalCents, &mr.ExpenseTotalCents); err != nil {
			dbError(w, "handleYearSummary months scan", err)
			return
		}
		mr.SurplusCents = mr.IncomeTotalCents - mr.ExpenseTotalCents
		yearIncome += mr.IncomeTotalCents
		yearExpense += mr.ExpenseTotalCents
		monthMap[mr.Month] = mr
	}
	rows.Close()
	// Without this a failed query renders as twelve zeroed months under a 200.
	if err := rows.Err(); err != nil {
		dbError(w, "handleYearSummary months", err)
		return
	}

	months := make([]monthRow, 12)
	for i := range months {
		if mr, ok := monthMap[i+1]; ok {
			months[i] = mr
		} else {
			months[i] = monthRow{Month: i + 1}
		}
	}

	type potBal struct {
		PotID        int64  `json:"potId"`
		Name         string `json:"name"`
		Kind         string `json:"kind"`
		BalanceCents int64  `json:"balanceCents"`
	}
	balRows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.kind, COALESCE(SUM(pl.amount_cents),0)
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL GROUP BY p.id,p.name,p.kind,p.sort_order ORDER BY p.sort_order,p.id`)
	if err != nil {
		dbError(w, "handleYearSummary pot balances", err)
		return
	}
	defer balRows.Close()
	balances := make([]potBal, 0)
	for balRows.Next() {
		var b potBal
		if err := balRows.Scan(&b.PotID, &b.Name, &b.Kind, &b.BalanceCents); err != nil {
			dbError(w, "handleYearSummary pot balances scan", err)
			return
		}
		balances = append(balances, b)
	}
	if err := balRows.Err(); err != nil {
		dbError(w, "handleYearSummary pot balances", err)
		return
	}

	locked, err := isYearLocked(ctx, s.pool, year)
	if err != nil {
		dbError(w, "handleYearSummary year lock", err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"year":                  year,
		"locked":                locked,
		"months":                months,
		"yearIncomeTotalCents":  yearIncome,
		"yearExpenseTotalCents": yearExpense,
		"yearSurplusCents":      yearIncome - yearExpense,
		"potBalances":           balances,
	})
}

// ── Trends (cross-year) ──────────────────────────────────────────

func (s *Server) handleTrendsYears(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type yearRow struct {
		Year              int   `json:"year"`
		IncomeTotalCents  int64 `json:"incomeTotalCents"`
		ExpenseTotalCents int64 `json:"expenseTotalCents"`
		SurplusCents      int64 `json:"surplusCents"`
	}

	rows, err := s.pool.Query(ctx, `
		SELECT y.year,
		  (SELECT `+domain.EffectiveIncomeCentsSQL+`
		  FROM income_entries ie
		  JOIN periods p ON p.id=ie.period_id
		  LEFT JOIN income_sources isrc ON isrc.id=ie.source_id
		  WHERE p.year=y.year),
		  (SELECT `+domain.EffectiveExpenseCentsSQL+`
		  FROM budget_lines bl
		  JOIN periods p ON p.id=bl.period_id
		  WHERE p.year=y.year)
		FROM years y ORDER BY y.year`)
	if err != nil {
		dbError(w, "handleTrendsYears", err)
		return
	}
	defer rows.Close()
	out := make([]yearRow, 0)
	for rows.Next() {
		var yr yearRow
		if err := rows.Scan(&yr.Year, &yr.IncomeTotalCents, &yr.ExpenseTotalCents); err != nil {
			dbError(w, "handleTrendsYears scan", err)
			return
		}
		yr.SurplusCents = yr.IncomeTotalCents - yr.ExpenseTotalCents
		out = append(out, yr)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleTrendsYears", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleTrendsMonthlyTotals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type monthlyTotal struct {
		Year              int   `json:"year"`
		Month             int   `json:"month"`
		IncomeTotalCents  int64 `json:"incomeTotalCents"`
		ExpenseTotalCents int64 `json:"expenseTotalCents"`
		SurplusCents      int64 `json:"surplusCents"`
	}

	rows, err := s.pool.Query(ctx, `
		SELECT p.year, p.month,
		  (SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=p.id),
		  (SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=p.id)
		FROM periods p ORDER BY p.year, p.month`)
	if err != nil {
		dbError(w, "handleTrendsMonthlyTotals", err)
		return
	}
	defer rows.Close()
	out := make([]monthlyTotal, 0)
	for rows.Next() {
		var mt monthlyTotal
		if err := rows.Scan(&mt.Year, &mt.Month, &mt.IncomeTotalCents, &mt.ExpenseTotalCents); err != nil {
			dbError(w, "handleTrendsMonthlyTotals scan", err)
			return
		}
		mt.SurplusCents = mt.IncomeTotalCents - mt.ExpenseTotalCents
		out = append(out, mt)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "handleTrendsMonthlyTotals", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleTrendsCategoryTotals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := s.pool.Query(ctx, `
		SELECT p.year, p.month, bl.category_id, c.name,
		  CASE WHEN bl.tracks_transactions
		    THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t JOIN category_rollup cr ON cr.member_id=t.category_id WHERE t.period_id=bl.period_id AND cr.category_id=bl.category_id),0)
		    ELSE bl.amount_cents END AS effective_cents
		FROM budget_lines bl
		JOIN periods p ON p.id = bl.period_id
		JOIN categories c ON c.id = bl.category_id
		ORDER BY p.year, p.month`)
	if err != nil {
		dbError(w, "handleTrendsCategoryTotals", err)
		return
	}
	defer rows.Close()

	type catInfo struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		total int64
	}
	catOrder := make([]int64, 0)
	cats := map[int64]*catInfo{}
	type periodKey struct{ year, month int }
	periodOrder := make([]periodKey, 0)
	periodSeen := map[periodKey]bool{}
	values := map[periodKey]map[int64]int64{}

	for rows.Next() {
		var year, month int
		var catID int64
		var name string
		var cents int64
		if err := rows.Scan(&year, &month, &catID, &name, &cents); err != nil {
			rows.Close()
			dbError(w, "handleTrendsCategoryTotals scan", err)
			return
		}
		if _, ok := cats[catID]; !ok {
			cats[catID] = &catInfo{ID: catID, Name: name}
			catOrder = append(catOrder, catID)
		}
		cats[catID].total += cents
		pk := periodKey{year, month}
		if !periodSeen[pk] {
			periodSeen[pk] = true
			periodOrder = append(periodOrder, pk)
			values[pk] = map[int64]int64{}
		}
		values[pk][catID] += cents
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		dbError(w, "handleTrendsCategoryTotals", err)
		return
	}

	sort.Slice(catOrder, func(i, j int) bool {
		return cats[catOrder[i]].total > cats[catOrder[j]].total
	})
	categories := make([]catInfo, 0, len(catOrder))
	for _, id := range catOrder {
		categories = append(categories, *cats[id])
	}

	sort.Slice(periodOrder, func(i, j int) bool {
		if periodOrder[i].year != periodOrder[j].year {
			return periodOrder[i].year < periodOrder[j].year
		}
		return periodOrder[i].month < periodOrder[j].month
	})

	type entry struct {
		Year   int             `json:"year"`
		Month  int             `json:"month"`
		Values map[int64]int64 `json:"values"`
	}
	entries := make([]entry, 0, len(periodOrder))
	for _, pk := range periodOrder {
		entries = append(entries, entry{Year: pk.year, Month: pk.month, Values: values[pk]})
	}

	JSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"entries":    entries,
	})
}
