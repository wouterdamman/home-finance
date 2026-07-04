package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/TheIronRock95/home-finance/internal/domain"
)

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

	if body.CopyFromPeriodID == nil {
		var srcID int64
		if err2 := s.pool.QueryRow(ctx,
			`SELECT id FROM periods WHERE (year < $1 OR (year = $1 AND month < $2)) ORDER BY year DESC, month DESC LIMIT 1`,
			body.Year, body.Month).Scan(&srcID); err2 == nil {
			body.CopyFromPeriodID = &srcID
		}
	}

	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO periods (year, month) VALUES ($1, $2) RETURNING id`, body.Year, body.Month).Scan(&id)
	if err != nil {
		Error(w, http.StatusConflict, "already_exists", "period already exists for that month")
		return
	}
	if body.CopyFromPeriodID != nil {
		src := *body.CopyFromPeriodID
		s.pool.Exec(ctx, `INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order) SELECT $1,category_id,label,amount_cents,tracks_transactions,sort_order FROM budget_lines WHERE period_id=$2`, id, src)
		s.pool.Exec(ctx, `INSERT INTO pot_splits (period_id,pot_id,percentage) SELECT $1,pot_id,percentage FROM pot_splits WHERE period_id=$2`, id, src)
		s.pool.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order) SELECT $1,source_id,label,amount_cents,'normal',notes,sort_order FROM income_entries WHERE period_id=$2 AND entry_type='normal'`, id, src)
	}
	type out struct {
		ID     int64  `json:"id"`
		Year   int    `json:"year"`
		Month  int    `json:"month"`
		Status string `json:"status"`
	}
	var o out
	s.pool.QueryRow(ctx, `SELECT id,year,month,status FROM periods WHERE id=$1`, id).Scan(&o.ID, &o.Year, &o.Month, &o.Status)
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
	incRows, _ := s.pool.Query(ctx, `SELECT id,source_id,label,amount_cents,entry_type,notes,sort_order FROM income_entries WHERE period_id=$1 ORDER BY sort_order,id`, id)
	incomes := make([]income, 0)
	var incomeTotal int64
	for incRows.Next() {
		var e income
		incRows.Scan(&e.ID, &e.SourceID, &e.Label, &e.AmountCents, &e.EntryType, &e.Notes, &e.SortOrder)
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
	blRows, _ := s.pool.Query(ctx, `
		SELECT bl.id, bl.category_id, bl.label, bl.amount_cents, bl.tracks_transactions, bl.sort_order,
		  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
		FROM budget_lines bl WHERE bl.period_id=$1 ORDER BY bl.sort_order,bl.id`, id)
	lines := make([]budgetLine, 0)
	var expenseTotal int64
	for blRows.Next() {
		var bl budgetLine
		blRows.Scan(&bl.ID, &bl.CategoryID, &bl.Label, &bl.AmountCents, &bl.TracksTransactions, &bl.SortOrder, &bl.TransactionsTotalCents)
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
	spRows, _ := s.pool.Query(ctx, `
		SELECT ps.pot_id, p.name, p.kind, ps.percentage
		FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id
		WHERE ps.period_id=$1 ORDER BY p.sort_order,p.id`, id)
	splits := make([]split, 0)
	var splitPctTotal float64
	var splitInputs []domain.PotSplitInput
	for spRows.Next() {
		var sp split
		var pct float64
		spRows.Scan(&sp.PotID, &sp.PotName, &sp.PotKind, &pct)
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

	var status string
	s.pool.QueryRow(ctx, `SELECT status FROM periods WHERE id=$1`, id).Scan(&status)
	if status == "closed" {
		Error(w, http.StatusConflict, "period_already_closed", "already closed")
		return
	}

	var incomeTotal, expenseTotal int64
	s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM income_entries WHERE period_id=$1`, id).Scan(&incomeTotal)
	s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN bl.tracks_transactions
		  THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
		  ELSE bl.amount_cents END),0)
		FROM budget_lines bl WHERE bl.period_id=$1`, id).Scan(&expenseTotal)
	surplus := incomeTotal - expenseTotal

	type splitRow struct {
		PotID int64
		Pct   float64
		Kind  string
	}
	spRows, _ := s.pool.Query(ctx, `SELECT ps.pot_id, ps.percentage, p.kind FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id WHERE ps.period_id=$1`, id)
	var rawSplits []splitRow
	for spRows.Next() {
		var sr splitRow
		spRows.Scan(&sr.PotID, &sr.Pct, &sr.Kind)
		rawSplits = append(rawSplits, sr)
	}
	spRows.Close()

	inputs := make([]domain.PotSplitInput, len(rawSplits))
	for i, sr := range rawSplits {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
	}
	allocs := domain.LargestRemainderSplit(surplus, inputs)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer tx.Rollback(ctx)

	today := time.Now().Format("2006-01-02")
	var periodYear, periodMonth int
	tx.QueryRow(ctx, `SELECT year, month FROM periods WHERE id=$1`, id).Scan(&periodYear, &periodMonth)

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
			tx.QueryRow(ctx, `INSERT INTO periods (year,month) VALUES ($1,$2) ON CONFLICT (year,month) DO UPDATE SET year=EXCLUDED.year RETURNING id`, nextYear, nextMonth).Scan(&nextPeriodID)
			tx.Exec(ctx, `INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,'carryover_out',$4,'Carryover out',$5)`,
				a.PotID, nextPeriodID, id, -a.AmountCents, today)
			months := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni", "Juli", "Augustus", "September", "Oktober", "November", "December"}
			monthName := ""
			if periodMonth >= 1 && periodMonth <= 12 {
				monthName = months[periodMonth]
			}
			tx.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,source_period_id,notes,sort_order) VALUES ($1,NULL,$2,$3,'carryover',$4,'',0)`,
				nextPeriodID, "Doorlopen maand "+monthName, a.AmountCents, id)
		}
	}

	tx.Exec(ctx, `UPDATE periods SET status='closed', closed_at=now() WHERE id=$1`, id)
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
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
	s.pool.QueryRow(ctx, `SELECT year,month FROM periods WHERE id=$1`, id).Scan(&year, &month)
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

	tx, _ := s.pool.Begin(ctx)
	defer tx.Rollback(ctx)
	tx.Exec(ctx, `DELETE FROM pot_ledger WHERE source_period_id=$1`, id)
	tx.Exec(ctx, `DELETE FROM income_entries WHERE entry_type='carryover' AND source_period_id=$1`, id)
	tx.Exec(ctx, `UPDATE periods SET status='open', closed_at=NULL WHERE id=$1`, id)
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Income entries ───────────────────────────────────────────────

func (s *Server) handleCreateIncomeEntry(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
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
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order) VALUES ($1,$2,$3,$4,'normal',$5,$6) RETURNING id`,
		periodID, body.SourceID, body.Label, body.AmountCents, body.Notes, body.SortOrder).Scan(&id)
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM income_entries WHERE id=$1`, id).Scan(&periodID)
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE income_entries SET label=$2,amount_cents=$3,notes=$4,sort_order=$5 WHERE id=$1`, id, body.Label, body.AmountCents, body.Notes, body.SortOrder)
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
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
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
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
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
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		periodID, body.CategoryID, body.Label, body.AmountCents, body.TracksTransactions, body.SortOrder).Scan(&id)
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM budget_lines WHERE id=$1`, id).Scan(&periodID)
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE budget_lines SET label=$2,amount_cents=$3,tracks_transactions=$4,sort_order=$5 WHERE id=$1`, id, body.Label, body.AmountCents, body.TracksTransactions, body.SortOrder)
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
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
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
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
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
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO transactions (period_id,category_id,amount_cents,description,tx_date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		periodID, body.CategoryID, body.AmountCents, body.Description, body.TxDate).Scan(&id)
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
	var periodID int64
	s.pool.QueryRow(r.Context(), `SELECT period_id FROM transactions WHERE id=$1`, id).Scan(&periodID)
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE transactions SET amount_cents=$2,description=$3,tx_date=$4 WHERE id=$1`, id, body.AmountCents, body.Description, body.TxDate)
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
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return
	}
	s.pool.Exec(r.Context(), `DELETE FROM transactions WHERE id=$1`, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Splits ───────────────────────────────────────────────────────

func (s *Server) handleReplaceSplits(w http.ResponseWriter, r *http.Request) {
	periodID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if closed, _ := isPeriodClosed(r.Context(), s.pool, periodID); closed {
		Error(w, http.StatusConflict, "period_closed", "period is closed")
		return
	}
	var body struct {
		Splits []struct {
			PotID      int64  `json:"potId"`
			Percentage string `json:"percentage"`
		} `json:"splits"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := r.Context()
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

	JSON(w, http.StatusOK, map[string]any{
		"year":                  year,
		"months":                months,
		"yearIncomeTotalCents":  yearIncome,
		"yearExpenseTotalCents": yearExpense,
		"yearSurplusCents":      yearIncome - yearExpense,
		"potBalances":           balances,
	})
}
