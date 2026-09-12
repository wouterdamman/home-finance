package httpapi

import (
	"net/http"
	"time"
)

// ── Kids savings ─────────────────────────────────────────────────

func (s *Server) handleListKids(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID                   int64   `json:"id"`
		Name                 string  `json:"name"`
		SortOrder            int     `json:"sortOrder"`
		ArchivedAt           *string `json:"archivedAt,omitempty"`
		ReportedBalanceCents *int64  `json:"reportedBalanceCents,omitempty"`
		ReportedBalanceDate  *string `json:"reportedBalanceDate,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,sort_order,archived_at,reported_balance_cents,reported_balance_date FROM kids WHERE archived_at IS NULL ORDER BY sort_order,id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		var rbd *time.Time
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.SortOrder, &aa, &ro.ReportedBalanceCents, &rbd); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		if rbd != nil {
			ds := rbd.Format("2006-01-02")
			ro.ReportedBalanceDate = &ds
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleGetKidBalances(w http.ResponseWriter, r *http.Request) {
	type row struct {
		KidID                int64   `json:"kidId"`
		Name                 string  `json:"name"`
		OursCents            int64   `json:"oursCents"`
		TheirsCents          int64   `json:"theirsCents"`
		TotalCents           int64   `json:"totalCents"`
		ReportedBalanceCents *int64  `json:"reportedBalanceCents,omitempty"`
		ReportedBalanceDate  *string `json:"reportedBalanceDate,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT k.id, k.name,
		       COALESCE(SUM(l.amount_cents) FILTER (WHERE l.owner = 'ours'), 0)::BIGINT AS ours_cents,
		       COALESCE(SUM(l.amount_cents) FILTER (WHERE l.owner = 'theirs'), 0)::BIGINT AS theirs_cents,
		       COALESCE(SUM(l.amount_cents), 0)::BIGINT AS total_cents,
		       k.reported_balance_cents, k.reported_balance_date
		FROM kids k
		LEFT JOIN kid_savings_ledger l ON l.kid_id = k.id
		WHERE k.archived_at IS NULL
		GROUP BY k.id, k.name, k.sort_order, k.reported_balance_cents, k.reported_balance_date
		ORDER BY k.sort_order, k.id`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var rbd *time.Time
		if err := rows.Scan(&ro.KidID, &ro.Name, &ro.OursCents, &ro.TheirsCents, &ro.TotalCents, &ro.ReportedBalanceCents, &rbd); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		if rbd != nil {
			ds := rbd.Format("2006-01-02")
			ro.ReportedBalanceDate = &ds
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdateKidReportedBalance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		ReportedBalanceCents *int64  `json:"reportedBalanceCents"`
		ReportedBalanceDate  *string `json:"reportedBalanceDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.ReportedBalanceCents != nil {
		if err := validateSignedAmountCents(*body.ReportedBalanceCents); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE kids SET reported_balance_cents=$2, reported_balance_date=$3 WHERE id=$1`, id, body.ReportedBalanceCents, body.ReportedBalanceDate); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(r.Context(), "kid.reported_balance.update", "kid", id, map[string]any{"reportedBalanceCents": body.ReportedBalanceCents, "reportedBalanceDate": body.ReportedBalanceDate})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetKidLedger(w http.ResponseWriter, r *http.Request) {
	kidID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	type row struct {
		ID                 int64  `json:"id"`
		Owner              string `json:"owner"`
		EntryType          string `json:"entryType"`
		AmountCents        int64  `json:"amountCents"`
		RunningOursCents   int64  `json:"runningOursCents"`
		RunningTheirsCents int64  `json:"runningTheirsCents"`
		Description        string `json:"description"`
		EntryDate          string `json:"entryDate"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, owner, entry_type, amount_cents,
		  SUM(amount_cents) FILTER (WHERE owner = 'ours') OVER (ORDER BY entry_date, id ROWS UNBOUNDED PRECEDING) AS running_ours,
		  SUM(amount_cents) FILTER (WHERE owner = 'theirs') OVER (ORDER BY entry_date, id ROWS UNBOUNDED PRECEDING) AS running_theirs,
		  description, entry_date
		FROM kid_savings_ledger WHERE kid_id=$1 ORDER BY entry_date, id`, kidID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ed time.Time
		var runOurs, runTheirs *int64
		if err := rows.Scan(&ro.ID, &ro.Owner, &ro.EntryType, &ro.AmountCents, &runOurs, &runTheirs, &ro.Description, &ed); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		if runOurs != nil {
			ro.RunningOursCents = *runOurs
		}
		if runTheirs != nil {
			ro.RunningTheirsCents = *runTheirs
		}
		ro.EntryDate = ed.Format("2006-01-02")
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateKidLedgerEntry(w http.ResponseWriter, r *http.Request) {
	kidID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Owner       string  `json:"owner"`
		EntryType   string  `json:"entryType"`
		AmountCents int64   `json:"amountCents"`
		Description string  `json:"description"`
		EntryDate   *string `json:"entryDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	switch body.Owner {
	case "ours", "theirs":
	default:
		Error(w, http.StatusBadRequest, "bad_request", "invalid owner")
		return
	}
	switch body.EntryType {
	case "opening_balance", "deposit", "withdrawal", "adjustment":
	default:
		Error(w, http.StatusBadRequest, "bad_request", "invalid entry_type")
		return
	}
	if body.EntryType == "withdrawal" && body.AmountCents > 0 {
		body.AmountCents = -body.AmountCents
	}
	if err := validateSignedAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO kid_savings_ledger (kid_id,owner,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,$4,$5,COALESCE($6,CURRENT_DATE)) RETURNING id`,
		kidID, body.Owner, body.EntryType, body.AmountCents, body.Description, body.EntryDate).Scan(&id); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(r.Context(), "kid.entry", "kid", kidID, map[string]any{"owner": body.Owner, "entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description})
	JSON(w, http.StatusCreated, map[string]any{"id": id, "kidId": kidID, "owner": body.Owner, "entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description, "entryDate": body.EntryDate})
}

func (s *Server) handleUpdateKidLedgerEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Owner       string `json:"owner"`
		AmountCents int64  `json:"amountCents"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var kidID int64
	var entryType string
	if err := s.pool.QueryRow(r.Context(), `SELECT kid_id, entry_type FROM kid_savings_ledger WHERE id=$1`, id).Scan(&kidID, &entryType); err != nil {
		Error(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	switch body.Owner {
	case "ours", "theirs":
	default:
		Error(w, http.StatusBadRequest, "bad_request", "invalid owner")
		return
	}
	amount := body.AmountCents
	if entryType == "withdrawal" && amount > 0 {
		amount = -amount
	}
	if err := validateSignedAmountCents(amount); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE kid_savings_ledger SET owner=$2, amount_cents=$3 WHERE id=$1`, id, body.Owner, amount); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(r.Context(), "kid.entry.update", "kid", kidID, map[string]any{"entryId": id, "owner": body.Owner, "amountCents": amount})
	JSON(w, http.StatusOK, map[string]any{"id": id, "kidId": kidID, "owner": body.Owner, "entryType": entryType, "amountCents": amount})
}

func (s *Server) handleDeleteKidLedgerEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var kidID int64
	if err := s.pool.QueryRow(r.Context(), `SELECT kid_id FROM kid_savings_ledger WHERE id=$1`, id).Scan(&kidID); err != nil {
		Error(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `DELETE FROM kid_savings_ledger WHERE id=$1`, id); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(r.Context(), "kid.entry.delete", "kid", kidID, map[string]any{"entryId": id})
	w.WriteHeader(http.StatusNoContent)
}
