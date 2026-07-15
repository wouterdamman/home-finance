package httpapi

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wouterdamman/home-finance/internal/domain"
)

const pgUniqueViolation = "23505"

// ── Periods ──────────────────────────────────────────────────────

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
		  COALESCE((SELECT SUM(ie.amount_cents) FROM income_entries ie WHERE ie.period_id = p.id), 0),
		  COALESCE((SELECT SUM(CASE WHEN bl.tracks_transactions
		    THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id = bl.period_id AND t.category_id = bl.category_id), 0)
		    ELSE bl.amount_cents END)
		  FROM budget_lines bl WHERE bl.period_id = p.id), 0)
		FROM periods p WHERE p.year = $1 ORDER BY p.month`, year)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ca *time.Time
		if err := rows.Scan(&ro.ID, &ro.Year, &ro.Month, &ro.Status, &ca, &ro.IncomeTotalCents, &ro.ExpenseTotalCents); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		if ca != nil {
			s := ca.Format(time.RFC3339)
			ro.ClosedAt = &s
		}
		ro.SurplusCents = ro.IncomeTotalCents - ro.ExpenseTotalCents
		out = append(out, ro)
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := r.Context()
	s.pool.Exec(ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, body.Year)

	if locked, _ := isYearLocked(ctx, s.pool, body.Year); locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}

	if body.CopyFromPeriodID == nil {
		var srcID int64
		if err2 := s.pool.QueryRow(ctx, `
			SELECT id FROM periods
			WHERE (year < $1 OR (year = $1 AND month < $2))
			AND (
				EXISTS (SELECT 1 FROM income_entries WHERE period_id=periods.id AND entry_type='normal')
				OR EXISTS (SELECT 1 FROM budget_lines WHERE period_id=periods.id)
			)
			ORDER BY year DESC, month DESC LIMIT 1`,
			body.Year, body.Month).Scan(&srcID); err2 == nil {
			body.CopyFromPeriodID = &srcID
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)

	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO periods (year, month) VALUES ($1, $2) RETURNING id`, body.Year, body.Month).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			Error(w, http.StatusConflict, "already_exists", "period already exists for that month")
			return
		}
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if body.CopyFromPeriodID != nil {
		src := *body.CopyFromPeriodID
		if _, err := tx.Exec(ctx, `
			INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order)
			SELECT $1,bl.category_id,bl.label,bl.amount_cents,bl.tracks_transactions,bl.sort_order
			FROM budget_lines bl
			WHERE bl.period_id=$2
			AND (bl.category_id IS NULL OR bl.category_id IN (SELECT id FROM categories WHERE include_in_template=true))`,
			id, src); err != nil {
			Error(w, http.StatusInternalServerError, "db_error", "failed to copy budget lines")
			return
		}
		if _, err := tx.Exec(ctx, `INSERT INTO pot_splits (period_id,pot_id,percentage) SELECT $1,pot_id,percentage FROM pot_splits WHERE period_id=$2`, id, src); err != nil {
			Error(w, http.StatusInternalServerError, "db_error", "failed to copy pot splits")
			return
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order)
			SELECT $1,ie.source_id,ie.label,ie.amount_cents,'normal',ie.notes,ie.sort_order
			FROM income_entries ie
			WHERE ie.period_id=$2 AND ie.entry_type='normal'
			AND (ie.source_id IS NULL OR ie.source_id IN (SELECT id FROM income_sources WHERE include_in_template=true))`,
			id, src); err != nil {
			Error(w, http.StatusInternalServerError, "db_error", "failed to copy income entries")
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
		ID          int64   `json:"id"`
		SourceID    *int64  `json:"sourceId,omitempty"`
		Label       *string `json:"label,omitempty"`
		AmountCents int64   `json:"amountCents"`
		EntryType   string  `json:"entryType"`
		Notes       string  `json:"notes"`
		SortOrder   int     `json:"sortOrder"`
	}
	incRows, err := s.pool.Query(ctx, `SELECT id,source_id,label,amount_cents,entry_type,notes,sort_order FROM income_entries WHERE period_id=$1 ORDER BY sort_order,id`, id)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	incomes := make([]income, 0)
	var incomeTotal int64
	for incRows.Next() {
		var e income
		if err := incRows.Scan(&e.ID, &e.SourceID, &e.Label, &e.AmountCents, &e.EntryType, &e.Notes, &e.SortOrder); err != nil {
			incRows.Close()
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		incomeTotal += e.AmountCents
		incomes = append(incomes, e)
	}
	incRows.Close()

	type budgetLine struct {
		ID                     int64   `json:"id"`
		CategoryID             *int64  `json:"categoryId,omitempty"`
		Label                  *string `json:"label,omitempty"`
		AmountCents            int64   `json:"amountCents"`
		TracksTransactions     bool    `json:"tracksTransactions"`
		TransactionsTotalCents int64   `json:"transactionsTotalCents"`
		EffectiveCents         int64   `json:"effectiveCents"`
		SortOrder              int     `json:"sortOrder"`
	}
	blRows, err := s.pool.Query(ctx, `
		SELECT bl.id, bl.category_id, bl.label, bl.amount_cents, bl.tracks_transactions, bl.sort_order,
		  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
		FROM budget_lines bl WHERE bl.period_id=$1 ORDER BY bl.sort_order,bl.id`, id)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	lines := make([]budgetLine, 0)
	var expenseTotal int64
	for blRows.Next() {
		var bl budgetLine
		if err := blRows.Scan(&bl.ID, &bl.CategoryID, &bl.Label, &bl.AmountCents, &bl.TracksTransactions, &bl.SortOrder, &bl.TransactionsTotalCents); err != nil {
			blRows.Close()
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		sp.Percentage = strconv.FormatFloat(pct, 'f', 2, 64)
		splitPctTotal += pct
		splitInputs = append(splitInputs, domain.PotSplitInput{PotID: sp.PotID, Percentage: pct})
		splits = append(splits, sp)
	}
	spRows.Close()
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

	var incomeTotal, expenseTotal int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM income_entries WHERE period_id=$1`, id).Scan(&incomeTotal); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN bl.tracks_transactions
		  THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
		  ELSE bl.amount_cents END),0)
		FROM budget_lines bl WHERE bl.period_id=$1`, id).Scan(&expenseTotal); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	var rawSplits []splitRow
	for spRows.Next() {
		var sr splitRow
		if err := spRows.Scan(&sr.PotID, &sr.Pct, &sr.Kind); err != nil {
			spRows.Close()
			Error(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		rawSplits = append(rawSplits, sr)
	}
	spRows.Close()
	if err := spRows.Err(); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	inputs := make([]domain.PotSplitInput, len(rawSplits))
	for i, sr := range rawSplits {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
	}
	allocs := domain.LargestRemainderSplit(surplus, inputs)

	today := time.Now().Format("2006-01-02")

	for i, a := range allocs {
		_, err = tx.Exec(ctx,
			`INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date)
			 VALUES ($1,$2,$2,'allocation',$3,'Monthly allocation',$4)`,
			a.PotID, id, a.AmountCents, today)
		if err != nil {
			Error(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if rawSplits[i].Kind == "carryover" {
			nextMonth := periodMonth + 1
			nextYear := periodYear
			if nextMonth > 12 {
				nextMonth = 1
				nextYear++
			}
			var nextPeriodID int64
			if err := tx.QueryRow(ctx, `INSERT INTO periods (year,month) VALUES ($1,$2) ON CONFLICT (year,month) DO UPDATE SET year=EXCLUDED.year RETURNING id`, nextYear, nextMonth).Scan(&nextPeriodID); err != nil {
				Error(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			if _, err := tx.Exec(ctx, `INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,'carryover_out',$4,'Carryover out',$5)`,
				a.PotID, nextPeriodID, id, -a.AmountCents, today); err != nil {
				Error(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
			months := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni", "Juli", "Augustus", "September", "Oktober", "November", "December"}
			monthName := ""
			if periodMonth >= 1 && periodMonth <= 12 {
				monthName = months[periodMonth]
			}
			if _, err := tx.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,source_period_id,notes,sort_order) VALUES ($1,NULL,$2,$3,'carryover',$4,'',0)`,
				nextPeriodID, "Doorlopen maand "+monthName, a.AmountCents, id); err != nil {
				Error(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE periods SET status='closed', closed_at=now() WHERE id=$1`, id); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	if currentStatus == "open" {
		Error(w, http.StatusConflict, "period_already_open", "period is already open")
		return
	}
	if locked, _ := isYearLocked(ctx, s.pool, year); locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}
	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}
	var nextStatus string
	s.pool.QueryRow(ctx, `SELECT status FROM periods WHERE year=$1 AND month=$2`, nextYear, nextMonth).Scan(&nextStatus)
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
	tx.Exec(ctx, `DELETE FROM pot_ledger WHERE source_period_id=$1`, id)
	tx.Exec(ctx, `DELETE FROM income_entries WHERE entry_type='carryover' AND source_period_id=$1`, id)
	tx.Exec(ctx, `UPDATE periods SET status='open', closed_at=NULL WHERE id=$1`, id)
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
	var body struct {
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// Credential check: dev mode accepts any non-empty password; prod requires DELETE_PASSWORD
	if s.cfg.DevFakeAuth {
		if body.Password == "" {
			Error(w, http.StatusForbidden, "forbidden", "password required")
			return
		}
	} else {
		if !checkPassword(s.cfg.DeletePassword, body.Password) {
			Error(w, http.StatusForbidden, "forbidden", "incorrect password")
			return
		}
	}

	ctx := r.Context()
	var year, month int
	var status string
	if err := s.pool.QueryRow(ctx, `SELECT year,month,status FROM periods WHERE id=$1`, id).Scan(&year, &month, &status); err != nil {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	if status == "closed" {
		Error(w, http.StatusConflict, "period_closed", "cannot delete a closed period; reopen it first")
		return
	}
	if locked, _ := isYearLocked(ctx, s.pool, year); locked {
		Error(w, http.StatusConflict, "year_locked", "year is locked")
		return
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM periods WHERE id=$1`, id); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "delete failed")
		return
	}
	s.auditLog(ctx, "period.delete", "period", id, map[string]any{"year": year, "month": month})
	w.WriteHeader(http.StatusNoContent)
}

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
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, created_at, user_email, action, entity_type, entity_id, details::text
		FROM audit_log ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ts time.Time
		rows.Scan(&ro.ID, &ts, &ro.UserEmail, &ro.Action, &ro.EntityType, &ro.EntityID, &ro.Details)
		ro.CreatedAt = ts.Format(time.RFC3339)
		out = append(out, ro)
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM income_entries WHERE id=$1`, id).Scan(&periodID)
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	s.pool.Exec(r.Context(), `DELETE FROM income_entries WHERE id=$1`, id)
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
		CategoryID         *int64  `json:"categoryId"`
		Label              *string `json:"label"`
		AmountCents        int64   `json:"amountCents"`
		TracksTransactions bool    `json:"tracksTransactions"`
		SortOrder          int     `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := validateAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		periodID, body.CategoryID, body.Label, body.AmountCents, body.TracksTransactions, body.SortOrder).Scan(&id); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "periodId": periodID, "categoryId": body.CategoryID, "label": body.Label, "amountCents": body.AmountCents, "tracksTransactions": body.TracksTransactions, "sortOrder": body.SortOrder})
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM budget_lines WHERE id=$1`, id).Scan(&periodID)
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	s.pool.Exec(r.Context(), `DELETE FROM budget_lines WHERE id=$1`, id)
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
	if catID != nil {
		rows, _ = s.pool.Query(r.Context(), `SELECT id,period_id,category_id,amount_cents,description,tx_date FROM transactions WHERE period_id=$1 AND category_id=$2 ORDER BY tx_date DESC NULLS LAST,id DESC`, periodID, *catID)
	} else {
		rows, _ = s.pool.Query(r.Context(), `SELECT id,period_id,category_id,amount_cents,description,tx_date FROM transactions WHERE period_id=$1 ORDER BY tx_date DESC NULLS LAST,id DESC`, periodID)
	}
	defer rows.Close()
	out := make([]txRow, 0)
	for rows.Next() {
		var tx txRow
		var d *time.Time
		rows.Scan(&tx.ID, &tx.PeriodID, &tx.CategoryID, &tx.AmountCents, &tx.Description, &d)
		if d != nil {
			ds := d.Format("2006-01-02")
			tx.TxDate = &ds
		}
		out = append(out, tx)
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
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
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM transactions WHERE id=$1`, id).Scan(&periodID)
	if !isPeriodWritable(r.Context(), s.pool, w, periodID) {
		return
	}
	s.pool.Exec(r.Context(), `DELETE FROM transactions WHERE id=$1`, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Splits ───────────────────────────────────────────────────────

type splitInput struct {
	PotID      int64  `json:"potId"`
	Percentage string `json:"percentage"`
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
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := r.Context()

	// The carryover pot always absorbs whatever percentage isn't explicitly
	// allocated to another pot — its percentage is never stored as-submitted,
	// it's recomputed here so it always keeps the total at exactly 100%.
	var carryoverPotID int64
	hasCarryover := s.pool.QueryRow(ctx, `SELECT id FROM pots WHERE kind='carryover' AND archived_at IS NULL`).Scan(&carryoverPotID) == nil

	var otherTotal float64
	carryoverIdx := -1
	for i, sp := range body.Splits {
		if hasCarryover && sp.PotID == carryoverPotID {
			carryoverIdx = i
			continue
		}
		pct, _ := strconv.ParseFloat(sp.Percentage, 64)
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

	tx, _ := s.pool.Begin(ctx)
	defer tx.Rollback(ctx)
	tx.Exec(ctx, `DELETE FROM pot_splits WHERE period_id=$1`, periodID)
	for _, sp := range body.Splits {
		pct, _ := strconv.ParseFloat(sp.Percentage, 64)
		tx.Exec(ctx, `INSERT INTO pot_splits (period_id,pot_id,percentage) VALUES ($1,$2,$3)`, periodID, sp.PotID, pct)
	}
	tx.Commit(ctx)
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

	rows, _ := s.pool.Query(ctx, `
		SELECT p.month, p.id, p.status,
		  COALESCE((SELECT SUM(ie.amount_cents) FROM income_entries ie WHERE ie.period_id=p.id),0),
		  COALESCE((SELECT SUM(CASE WHEN bl.tracks_transactions
		    THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
		    ELSE bl.amount_cents END) FROM budget_lines bl WHERE bl.period_id=p.id),0)
		FROM periods p WHERE p.year=$1 ORDER BY p.month`, year)
	defer rows.Close()

	monthMap := map[int]monthRow{}
	var yearIncome, yearExpense int64
	for rows.Next() {
		var mr monthRow
		rows.Scan(&mr.Month, &mr.PeriodID, &mr.Status, &mr.IncomeTotalCents, &mr.ExpenseTotalCents)
		mr.SurplusCents = mr.IncomeTotalCents - mr.ExpenseTotalCents
		yearIncome += mr.IncomeTotalCents
		yearExpense += mr.ExpenseTotalCents
		monthMap[mr.Month] = mr
	}
	rows.Close()

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
	balRows, _ := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.kind, COALESCE(SUM(pl.amount_cents),0)
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL GROUP BY p.id,p.name,p.kind,p.sort_order ORDER BY p.sort_order,p.id`)
	defer balRows.Close()
	balances := make([]potBal, 0)
	for balRows.Next() {
		var b potBal
		balRows.Scan(&b.PotID, &b.Name, &b.Kind, &b.BalanceCents)
		balances = append(balances, b)
	}

	locked, _ := isYearLocked(ctx, s.pool, year)
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
