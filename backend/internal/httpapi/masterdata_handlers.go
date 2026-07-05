package httpapi

import (
	"net/http"
	"time"
)

// ── Categories ───────────────────────────────────────────────────

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID                 int64   `json:"id"`
		Name               string  `json:"name"`
		DefaultAmountCents int64   `json:"defaultAmountCents"`
		IsItemized         bool    `json:"isItemized"`
		IncludeInTemplate  bool    `json:"includeInTemplate"`
		SortOrder          int     `json:"sortOrder"`
		ArchivedAt         *string `json:"archivedAt,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,default_amount_cents,is_itemized,include_in_template,sort_order,archived_at FROM categories ORDER BY sort_order,id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		rows.Scan(&ro.ID, &ro.Name, &ro.DefaultAmountCents, &ro.IsItemized, &ro.IncludeInTemplate, &ro.SortOrder, &aa)
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IsItemized         bool   `json:"isItemized"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO categories (name,default_amount_cents,is_itemized,include_in_template,sort_order) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		body.Name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate, body.SortOrder).Scan(&id)
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": body.Name, "defaultAmountCents": body.DefaultAmountCents, "isItemized": body.IsItemized, "includeInTemplate": body.IncludeInTemplate, "sortOrder": body.SortOrder})
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IsItemized         bool   `json:"isItemized"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.pool.Exec(r.Context(), `UPDATE categories SET name=$2,default_amount_cents=$3,is_itemized=$4,include_in_template=$5 WHERE id=$1`, id, body.Name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", "could not start transaction")
		return
	}
	defer tx.Rollback(ctx)
	for i, cid := range body.IDs {
		tx.Exec(ctx, `UPDATE categories SET sort_order=$2 WHERE id=$1`, cid, i)
	}
	if err := tx.Commit(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE categories SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1`, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Income sources ────────────────────────────────────────────────

func (s *Server) handleListIncomeSources(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID                 int64   `json:"id"`
		Name               string  `json:"name"`
		DefaultAmountCents int64   `json:"defaultAmountCents"`
		IncludeInTemplate  bool    `json:"includeInTemplate"`
		SortOrder          int     `json:"sortOrder"`
		ArchivedAt         *string `json:"archivedAt,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,default_amount_cents,include_in_template,sort_order,archived_at FROM income_sources ORDER BY sort_order,id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		rows.Scan(&ro.ID, &ro.Name, &ro.DefaultAmountCents, &ro.IncludeInTemplate, &ro.SortOrder, &aa)
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateIncomeSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO income_sources (name,default_amount_cents,include_in_template,sort_order) VALUES ($1,$2,$3,$4) RETURNING id`,
		body.Name, body.DefaultAmountCents, body.IncludeInTemplate, body.SortOrder).Scan(&id)
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": body.Name, "defaultAmountCents": body.DefaultAmountCents, "includeInTemplate": body.IncludeInTemplate, "sortOrder": body.SortOrder})
}

func (s *Server) handleUpdateIncomeSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.pool.Exec(r.Context(), `UPDATE income_sources SET name=$2,default_amount_cents=$3,include_in_template=$4,sort_order=$5 WHERE id=$1`, id, body.Name, body.DefaultAmountCents, body.IncludeInTemplate, body.SortOrder)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveIncomeSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE income_sources SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1`, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Pots ─────────────────────────────────────────────────────────

func (s *Server) handleListPots(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID         int64   `json:"id"`
		Name       string  `json:"name"`
		Kind       string  `json:"kind"`
		SortOrder  int     `json:"sortOrder"`
		ArchivedAt *string `json:"archivedAt,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,kind,sort_order,archived_at FROM pots ORDER BY sort_order,id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		rows.Scan(&ro.ID, &ro.Name, &ro.Kind, &ro.SortOrder, &aa)
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleGetPotBalances(w http.ResponseWriter, r *http.Request) {
	type row struct {
		PotID        int64  `json:"potId"`
		Name         string `json:"name"`
		Kind         string `json:"kind"`
		BalanceCents int64  `json:"balanceCents"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id, p.name, p.kind, COALESCE(SUM(pl.amount_cents),0)
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL
		GROUP BY p.id,p.name,p.kind,p.sort_order
		ORDER BY p.sort_order,p.id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		rows.Scan(&ro.PotID, &ro.Name, &ro.Kind, &ro.BalanceCents)
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreatePot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.Kind == "" {
		body.Kind = "normal"
	}
	var id int64
	err := s.pool.QueryRow(r.Context(),
		`INSERT INTO pots (name,kind,sort_order) VALUES ($1,$2,$3) RETURNING id`,
		body.Name, body.Kind, body.SortOrder).Scan(&id)
	if err != nil {
		Error(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": body.Name, "kind": body.Kind, "sortOrder": body.SortOrder})
}

func (s *Server) handleUpdatePot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.Kind == "" {
		body.Kind = "normal"
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE pots SET name=$2,kind=$3,sort_order=$4 WHERE id=$1`, id, body.Name, body.Kind, body.SortOrder); err != nil {
		Error(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchivePot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	s.pool.Exec(r.Context(), `UPDATE pots SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1`, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetPotLedger(w http.ResponseWriter, r *http.Request) {
	potID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	type row struct {
		ID              int64   `json:"id"`
		PeriodID        *int64  `json:"periodId,omitempty"`
		SourcePeriodID  *int64  `json:"sourcePeriodId,omitempty"`
		EntryType       string  `json:"entryType"`
		AmountCents     int64   `json:"amountCents"`
		RunningBalance  int64   `json:"runningBalance"`
		Description     string  `json:"description"`
		EntryDate       *string `json:"entryDate,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, period_id, source_period_id, entry_type, amount_cents,
		  SUM(amount_cents) OVER (ORDER BY entry_date,id ROWS UNBOUNDED PRECEDING) AS running_balance,
		  description, entry_date
		FROM pot_ledger WHERE pot_id=$1 ORDER BY entry_date,id`, potID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ed *time.Time
		rows.Scan(&ro.ID, &ro.PeriodID, &ro.SourcePeriodID, &ro.EntryType, &ro.AmountCents, &ro.RunningBalance, &ro.Description, &ed)
		if ed != nil {
			ds := ed.Format("2006-01-02")
			ro.EntryDate = &ds
		}
		out = append(out, ro)
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreatePotEntry(w http.ResponseWriter, r *http.Request) {
	potID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		EntryType   string  `json:"entryType"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		EntryDate   *string `json:"entryDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	switch body.EntryType {
	case "withdrawal", "deposit", "adjustment", "opening_balance":
	default:
		Error(w, http.StatusBadRequest, "bad_request", "invalid entry_type")
		return
	}
	if body.EntryType == "withdrawal" && body.AmountCents > 0 {
		body.AmountCents = -body.AmountCents
	}
	var id int64
	s.pool.QueryRow(r.Context(),
		`INSERT INTO pot_ledger (pot_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		potID, body.EntryType, body.AmountCents, body.Description, body.EntryDate).Scan(&id)
	s.auditLog(r.Context(), "pot.entry", "pot", potID, map[string]any{"entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description})
	JSON(w, http.StatusCreated, map[string]any{"id": id, "potId": potID, "entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description, "entryDate": body.EntryDate})
}
