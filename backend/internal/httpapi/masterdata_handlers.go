package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgCheckViolation    = "23514"
	pgNumericOutOfRange = "22003"
)

// maxReorderIDs bounds a drag-and-drop reorder payload. The UI can only ever
// send as many ids as there are categories; anything beyond this is a client
// bug or an attempt to make the server chew through an arbitrarily long array.
const maxReorderIDs = 1000

// maxNameLen caps a masterdata name. The columns are unbounded TEXT and the
// names are rendered in every list, table and select on the site.
const maxNameLen = 200

// mapDBError translates a Postgres error into the closest matching HTTP
// response. Handlers used to map *any* query failure to 409 with the raw
// driver text, which turned a CHECK violation into a "conflict", a pool
// outage into a "conflict", and leaked table, constraint and column names
// plus the conflicting value to the caller.
func mapDBError(w http.ResponseWriter, op string, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			Error(w, http.StatusConflict, "conflict", "a record with these values already exists")
			return
		case pgForeignKeyViolation:
			Error(w, http.StatusNotFound, "not_found", "referenced record not found")
			return
		case pgCheckViolation, pgNumericOutOfRange:
			Error(w, http.StatusBadRequest, "bad_request", "one of the submitted values is not allowed")
			return
		}
	}
	dbError(w, op, err)
}

// requireName trims a submitted name and rejects it when nothing is left —
// the columns are TEXT NOT NULL, which accepts an empty string just fine,
// and the inline-edit UI can submit one.
func requireName(w http.ResponseWriter, name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		Error(w, http.StatusBadRequest, "bad_request", "name is required")
		return "", false
	}
	if len(trimmed) > maxNameLen {
		Error(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("name must be at most %d characters", maxNameLen))
		return "", false
	}
	return trimmed, true
}

// entryYear resolves the calendar year a ledger entry falls in. A missing
// date means the database defaults the column to CURRENT_DATE, so today's
// year is what the locked-year check has to be applied to.
func entryYear(date *string) (int, bool) {
	if date == nil || *date == "" {
		return time.Now().Year(), true
	}
	t, err := time.Parse("2006-01-02", *date)
	if err != nil {
		return 0, false
	}
	return t.Year(), true
}

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
		ParentID           *int64  `json:"parentId,omitempty"`
		AutofillActual     bool    `json:"autofillActual"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,default_amount_cents,is_itemized,include_in_template,sort_order,archived_at,parent_id,autofill_actual FROM categories ORDER BY sort_order,id`)
	if err != nil {
		dbError(w, "listCategories", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.DefaultAmountCents, &ro.IsItemized, &ro.IncludeInTemplate, &ro.SortOrder, &aa, &ro.ParentID, &ro.AutofillActual); err != nil {
			dbError(w, "listCategories.scan", err)
			return
		}
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listCategories", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

// validateCategoryParent enforces a 2-level-max category hierarchy at the
// Go level (matching this repo's convention of no DB triggers/CHECKs for
// cross-row invariants): a category can't be its own parent, can't be
// parented under a category that is itself a child, and a category that
// already has children can't be turned into a child. selfID is nil when
// validating a brand-new category (which by construction has no children
// and no budget lines yet, so only the self-parent and parent-has-parent
// checks apply). It writes its own error response and reports whether the
// caller may proceed.
func (s *Server) validateCategoryParent(ctx context.Context, w http.ResponseWriter, selfID *int64, parentID *int64) bool {
	if parentID == nil {
		return true
	}
	if selfID != nil && *parentID == *selfID {
		Error(w, http.StatusBadRequest, "bad_request", "a category cannot be its own parent")
		return false
	}
	var parentHasParent bool
	if err := s.pool.QueryRow(ctx, `SELECT parent_id IS NOT NULL FROM categories WHERE id=$1`, *parentID).Scan(&parentHasParent); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusBadRequest, "bad_request", "parent category not found")
			return false
		}
		dbError(w, "validateCategoryParent.parent", err)
		return false
	}
	if parentHasParent {
		Error(w, http.StatusBadRequest, "bad_request", "parent category is itself a child; only 2 levels are supported")
		return false
	}
	if selfID == nil {
		return true
	}

	var currentParent *int64
	var hasChildren bool
	if err := s.pool.QueryRow(ctx,
		`SELECT parent_id, EXISTS(SELECT 1 FROM categories c WHERE c.parent_id=$1) FROM categories WHERE id=$1`,
		*selfID).Scan(&currentParent, &hasChildren); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "category not found")
			return false
		}
		dbError(w, "validateCategoryParent.self", err)
		return false
	}
	if hasChildren {
		Error(w, http.StatusBadRequest, "bad_request", "category already has children and cannot become a child itself")
		return false
	}
	if currentParent != nil && *currentParent == *parentID {
		// Unchanged parent: an ordinary rename must not be blocked by a
		// coexistence problem that already exists in the stored data.
		return true
	}
	return s.checkNoCoexistingBudgetLines(ctx, w, *selfID, *parentID)
}

// checkNoCoexistingBudgetLines rejects a re-parent that would double-count
// spending. A parent's budget line sums its children's transactions through
// category_rollup, so if the category being demoted to a child already owns
// a budget line in a period where the new parent also has one, that period's
// expenses would be counted twice — inflating the total, deflating the
// surplus and corrupting the pot allocation at close.
func (s *Server) checkNoCoexistingBudgetLines(ctx context.Context, w http.ResponseWriter, selfID, parentID int64) bool {
	rows, err := s.pool.Query(ctx, `
		SELECT p.year, p.month
		FROM budget_lines child
		JOIN budget_lines parent ON parent.period_id = child.period_id AND parent.category_id = $2
		JOIN periods p ON p.id = child.period_id
		WHERE child.category_id = $1
		ORDER BY p.year, p.month`, selfID, parentID)
	if err != nil {
		dbError(w, "checkNoCoexistingBudgetLines", err)
		return false
	}
	defer rows.Close()
	var periods []string
	for rows.Next() {
		var year, month int
		if err := rows.Scan(&year, &month); err != nil {
			dbError(w, "checkNoCoexistingBudgetLines.scan", err)
			return false
		}
		periods = append(periods, fmt.Sprintf("%04d-%02d", year, month))
	}
	if err := rows.Err(); err != nil {
		dbError(w, "checkNoCoexistingBudgetLines", err)
		return false
	}
	if len(periods) > 0 {
		Error(w, http.StatusConflict, "in_use",
			"this category and its new parent both have a budget line in "+strings.Join(periods, ", ")+
				"; remove one of the two lines in those months before nesting it")
		return false
	}
	return true
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IsItemized         bool   `json:"isItemized"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
		ParentID           *int64 `json:"parentId"`
		AutofillActual     bool   `json:"autofillActual"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	// default_amount_cents is copied verbatim into budget_lines.amount_cents
	// by copyPeriodTemplate, so a negative value here silently inflates the
	// surplus of every period templated afterwards.
	if err := validateAmountCents(body.DefaultAmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !s.validateCategoryParent(r.Context(), w, nil, body.ParentID) {
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO categories (name,default_amount_cents,is_itemized,include_in_template,sort_order,parent_id,autofill_actual) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate, body.SortOrder, body.ParentID, body.AutofillActual).Scan(&id); err != nil {
		mapDBError(w, "createCategory", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "defaultAmountCents": body.DefaultAmountCents, "isItemized": body.IsItemized, "includeInTemplate": body.IncludeInTemplate, "sortOrder": body.SortOrder, "parentId": body.ParentID, "autofillActual": body.AutofillActual})
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
		ParentID           *int64 `json:"parentId"`
		AutofillActual     bool   `json:"autofillActual"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	if err := validateAmountCents(body.DefaultAmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !s.validateCategoryParent(r.Context(), w, &id, body.ParentID) {
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE categories SET name=$2,default_amount_cents=$3,is_itemized=$4,include_in_template=$5,parent_id=$6,autofill_actual=$7 WHERE id=$1`, id, name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate, body.ParentID, body.AutofillActual)
	if err != nil {
		mapDBError(w, "updateCategory", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "category not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReorderCategories(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if len(body.IDs) > maxReorderIDs {
		Error(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("at most %d ids may be reordered at once", maxReorderIDs))
		return
	}
	// One statement instead of a round trip per category: WITH ORDINALITY
	// turns the submitted array into (id, position) pairs the UPDATE joins on.
	if _, err := s.pool.Exec(r.Context(), `
		UPDATE categories c SET sort_order = o.ord - 1
		FROM unnest($1::bigint[]) WITH ORDINALITY AS o(id, ord)
		WHERE c.id = o.id`, body.IDs); err != nil {
		dbError(w, "reorderCategories", err)
		return
	}
	s.auditLog(r.Context(), "category.reorder", "category", 0, map[string]any{"ids": body.IDs})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var archivedAt *time.Time
	if err := s.pool.QueryRow(r.Context(), `UPDATE categories SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1 RETURNING archived_at`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "category not found")
			return
		}
		dbError(w, "archiveCategory", err)
		return
	}
	s.auditLog(r.Context(), archiveAction("category", archivedAt), "category", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// archiveAction names the audit action for a toggle-style archive endpoint —
// these routes archive or unarchive depending on the row's current state, and
// the trail has to say which one actually happened.
func archiveAction(entity string, archivedAt *time.Time) string {
	if archivedAt == nil {
		return entity + ".unarchive"
	}
	return entity + ".archive"
}

// ── Income sources ────────────────────────────────────────────────

func (s *Server) handleListIncomeSources(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID                 int64   `json:"id"`
		Name               string  `json:"name"`
		DefaultAmountCents int64   `json:"defaultAmountCents"`
		IsItemized         bool    `json:"isItemized"`
		IncludeInTemplate  bool    `json:"includeInTemplate"`
		SortOrder          int     `json:"sortOrder"`
		ArchivedAt         *string `json:"archivedAt,omitempty"`
		AutofillActual     bool    `json:"autofillActual"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,default_amount_cents,is_itemized,include_in_template,sort_order,archived_at,autofill_actual FROM income_sources ORDER BY sort_order,id`)
	if err != nil {
		dbError(w, "listIncomeSources", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa *time.Time
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.DefaultAmountCents, &ro.IsItemized, &ro.IncludeInTemplate, &ro.SortOrder, &aa, &ro.AutofillActual); err != nil {
			dbError(w, "listIncomeSources.scan", err)
			return
		}
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listIncomeSources", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateIncomeSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string `json:"name"`
		DefaultAmountCents int64  `json:"defaultAmountCents"`
		IsItemized         bool   `json:"isItemized"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
		AutofillActual     bool   `json:"autofillActual"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	if err := validateAmountCents(body.DefaultAmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO income_sources (name,default_amount_cents,is_itemized,include_in_template,sort_order,autofill_actual) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate, body.SortOrder, body.AutofillActual).Scan(&id); err != nil {
		mapDBError(w, "createIncomeSource", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "defaultAmountCents": body.DefaultAmountCents, "isItemized": body.IsItemized, "includeInTemplate": body.IncludeInTemplate, "sortOrder": body.SortOrder, "autofillActual": body.AutofillActual})
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
		IsItemized         bool   `json:"isItemized"`
		IncludeInTemplate  bool   `json:"includeInTemplate"`
		SortOrder          int    `json:"sortOrder"`
		AutofillActual     bool   `json:"autofillActual"`
	}
	body.IncludeInTemplate = true
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	if err := validateAmountCents(body.DefaultAmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE income_sources SET name=$2,default_amount_cents=$3,is_itemized=$4,include_in_template=$5,sort_order=$6,autofill_actual=$7 WHERE id=$1`, id, name, body.DefaultAmountCents, body.IsItemized, body.IncludeInTemplate, body.SortOrder, body.AutofillActual)
	if err != nil {
		mapDBError(w, "updateIncomeSource", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "income source not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleArchiveIncomeSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var archivedAt *time.Time
	if err := s.pool.QueryRow(r.Context(), `UPDATE income_sources SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1 RETURNING archived_at`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "income source not found")
			return
		}
		dbError(w, "archiveIncomeSource", err)
		return
	}
	s.auditLog(r.Context(), archiveAction("income_source", archivedAt), "income_source", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteIncomeSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM income_sources WHERE id=$1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			Error(w, http.StatusConflict, "in_use", "income source has entries or transactions and cannot be deleted; archive it instead")
			return
		}
		dbError(w, "deleteIncomeSource", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "income source not found")
		return
	}
	s.auditLog(r.Context(), "income_source.delete", "income_source", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ── Pots ─────────────────────────────────────────────────────────

func (s *Server) handleListPots(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID          int64   `json:"id"`
		Name        string  `json:"name"`
		Kind        string  `json:"kind"`
		SortOrder   int     `json:"sortOrder"`
		TargetCents *int64  `json:"targetCents,omitempty"`
		TargetDate  *string `json:"targetDate,omitempty"`
		ArchivedAt  *string `json:"archivedAt,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,name,kind,sort_order,target_cents,target_date,archived_at FROM pots ORDER BY sort_order,id`)
	if err != nil {
		dbError(w, "listPots", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var aa, td *time.Time
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.Kind, &ro.SortOrder, &ro.TargetCents, &td, &aa); err != nil {
			dbError(w, "listPots.scan", err)
			return
		}
		if aa != nil {
			ts := aa.Format(time.RFC3339)
			ro.ArchivedAt = &ts
		}
		if td != nil {
			ds := td.Format("2006-01-02")
			ro.TargetDate = &ds
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listPots", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleGetPotBalances(w http.ResponseWriter, r *http.Request) {
	type row struct {
		PotID        int64   `json:"potId"`
		Name         string  `json:"name"`
		Kind         string  `json:"kind"`
		BalanceCents int64   `json:"balanceCents"`
		TargetCents  *int64  `json:"targetCents,omitempty"`
		TargetDate   *string `json:"targetDate,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT p.id, p.name, p.kind, COALESCE(SUM(pl.amount_cents),0), p.target_cents, p.target_date
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL
		GROUP BY p.id,p.name,p.kind,p.sort_order,p.target_cents,p.target_date
		ORDER BY p.sort_order,p.id`)
	if err != nil {
		dbError(w, "getPotBalances", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var td *time.Time
		if err := rows.Scan(&ro.PotID, &ro.Name, &ro.Kind, &ro.BalanceCents, &ro.TargetCents, &td); err != nil {
			dbError(w, "getPotBalances.scan", err)
			return
		}
		if td != nil {
			ds := td.Format("2006-01-02")
			ro.TargetDate = &ds
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "getPotBalances", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreatePot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string  `json:"name"`
		Kind        string  `json:"kind"`
		SortOrder   int     `json:"sortOrder"`
		TargetCents *int64  `json:"targetCents"`
		TargetDate  *string `json:"targetDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	if body.Kind == "" {
		body.Kind = "normal"
	}
	if body.TargetCents != nil {
		if err := validateAmountCents(*body.TargetCents); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO pots (name,kind,sort_order,target_cents,target_date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		name, body.Kind, body.SortOrder, body.TargetCents, body.TargetDate).Scan(&id); err != nil {
		mapDBError(w, "createPot", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "kind": body.Kind, "sortOrder": body.SortOrder, "targetCents": body.TargetCents, "targetDate": body.TargetDate})
}

func (s *Server) handleUpdatePot(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Kind        string  `json:"kind"`
		SortOrder   int     `json:"sortOrder"`
		TargetCents *int64  `json:"targetCents"`
		TargetDate  *string `json:"targetDate"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	name, ok := requireName(w, body.Name)
	if !ok {
		return
	}
	if body.Kind == "" {
		body.Kind = "normal"
	}
	if body.TargetCents != nil {
		if err := validateAmountCents(*body.TargetCents); err != nil {
			Error(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE pots SET name=$2,kind=$3,sort_order=$4,target_cents=$5,target_date=$6 WHERE id=$1`, id, name, body.Kind, body.SortOrder, body.TargetCents, body.TargetDate)
	if err != nil {
		mapDBError(w, "updatePot", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "pot not found")
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
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		dbError(w, "archivePot.begin", err)
		return
	}
	defer tx.Rollback(ctx)

	var archivedAt *time.Time
	if err := tx.QueryRow(ctx, `UPDATE pots SET archived_at=CASE WHEN archived_at IS NULL THEN now() ELSE NULL END WHERE id=$1 RETURNING archived_at`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "pot not found")
			return
		}
		dbError(w, "archivePot", err)
		return
	}
	if archivedAt != nil && !detachPotSplits(ctx, tx, w, id) {
		return
	}
	if err := tx.Commit(ctx); err != nil {
		dbError(w, "archivePot.commit", err)
		return
	}
	s.auditLog(ctx, archiveAction("pot", archivedAt), "pot", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// detachPotSplits removes an archived pot from every not-yet-closed period's
// split table. Every balance view filters archived pots out but
// handleClosePeriod does not, so leaving the rows in place allocates a share
// of the surplus into a pot no screen ever sums — the money simply vanishes
// from the pots page. The freed percentage goes to the carryover pot, which
// is always 100 − sum(other pots) (same contract as handleReplaceSplits).
// Without a carryover pot there is nowhere to put it, so the archive is
// rejected and the caller has to reassign the percentage first.
func detachPotSplits(ctx context.Context, tx pgx.Tx, w http.ResponseWriter, potID int64) bool {
	type affected struct {
		periodID int64
		label    string
	}
	rows, err := tx.Query(ctx, `
		SELECT ps.period_id, p.year, p.month
		FROM pot_splits ps JOIN periods p ON p.id = ps.period_id
		WHERE ps.pot_id = $1 AND p.status <> 'closed'
		ORDER BY p.year, p.month`, potID)
	if err != nil {
		dbError(w, "detachPotSplits.periods", err)
		return false
	}
	var periods []affected
	for rows.Next() {
		var a affected
		var year, month int
		if err := rows.Scan(&a.periodID, &year, &month); err != nil {
			rows.Close()
			dbError(w, "detachPotSplits.scan", err)
			return false
		}
		a.label = fmt.Sprintf("%04d-%02d", year, month)
		periods = append(periods, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		dbError(w, "detachPotSplits.periods", err)
		return false
	}
	if len(periods) == 0 {
		return true
	}

	// Looked up after the archive UPDATE, so archiving the carryover pot
	// itself correctly reports "no carryover pot" here.
	var carryoverPotID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM pots WHERE kind='carryover' AND archived_at IS NULL`).Scan(&carryoverPotID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			dbError(w, "detachPotSplits.carryover", err)
			return false
		}
		carryoverPotID = 0
	}

	var blocked []string
	for _, a := range periods {
		if _, err := tx.Exec(ctx, `DELETE FROM pot_splits WHERE pot_id=$1 AND period_id=$2`, potID, a.periodID); err != nil {
			dbError(w, "detachPotSplits.delete", err)
			return false
		}
		var otherTotal float64
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(percentage),0)::float8 FROM pot_splits WHERE period_id=$1 AND pot_id <> $2`,
			a.periodID, carryoverPotID).Scan(&otherTotal); err != nil {
			dbError(w, "detachPotSplits.total", err)
			return false
		}
		if carryoverPotID == 0 {
			if otherTotal < 100-0.005 {
				blocked = append(blocked, a.label)
			}
			continue
		}
		remainder := 100 - otherTotal
		if remainder < 0 {
			remainder = 0
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO pot_splits (period_id, pot_id, percentage) VALUES ($1,$2,$3)
			ON CONFLICT (period_id, pot_id) DO UPDATE SET percentage = EXCLUDED.percentage`,
			a.periodID, carryoverPotID, remainder); err != nil {
			dbError(w, "detachPotSplits.carryoverSplit", err)
			return false
		}
	}
	if len(blocked) > 0 {
		Error(w, http.StatusConflict, "in_use",
			"this pot still receives a share of the surplus in "+strings.Join(blocked, ", ")+
				" and there is no carryover pot to absorb it; reassign its percentage to another pot first")
		return false
	}
	return true
}

func (s *Server) handleGetPotLedger(w http.ResponseWriter, r *http.Request) {
	potID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	type row struct {
		ID             int64   `json:"id"`
		PeriodID       *int64  `json:"periodId,omitempty"`
		SourcePeriodID *int64  `json:"sourcePeriodId,omitempty"`
		EntryType      string  `json:"entryType"`
		AmountCents    int64   `json:"amountCents"`
		RunningBalance int64   `json:"runningBalance"`
		Description    string  `json:"description"`
		EntryDate      *string `json:"entryDate,omitempty"`
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, period_id, source_period_id, entry_type, amount_cents,
		  SUM(amount_cents) OVER (ORDER BY entry_date,id ROWS UNBOUNDED PRECEDING) AS running_balance,
		  description, entry_date
		FROM pot_ledger WHERE pot_id=$1 ORDER BY entry_date,id`, potID)
	if err != nil {
		dbError(w, "getPotLedger", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		var ed *time.Time
		if err := rows.Scan(&ro.ID, &ro.PeriodID, &ro.SourcePeriodID, &ro.EntryType, &ro.AmountCents, &ro.RunningBalance, &ro.Description, &ed); err != nil {
			dbError(w, "getPotLedger.scan", err)
			return
		}
		if ed != nil {
			ds := ed.Format("2006-01-02")
			ro.EntryDate = &ds
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "getPotLedger", err)
		return
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
		badRequest(w, err)
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
	if err := validateSignedAmountCents(body.AmountCents); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	// Pot entries carry their own entry_date and have no period linkage, so
	// the locked-year guard has to be applied against that date directly.
	year, ok := entryYear(body.EntryDate)
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "entryDate must be YYYY-MM-DD")
		return
	}
	if !isYearWritable(r.Context(), s.pool, w, year) {
		return
	}
	var id int64
	var storedDate time.Time
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO pot_ledger (pot_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,$4,COALESCE($5,CURRENT_DATE)) RETURNING id, entry_date`,
		potID, body.EntryType, body.AmountCents, body.Description, body.EntryDate).Scan(&id, &storedDate); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			Error(w, http.StatusNotFound, "not_found", "pot not found")
			return
		}
		mapDBError(w, "createPotEntry", err)
		return
	}
	s.auditLog(r.Context(), "pot.entry", "pot", potID, map[string]any{"entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description})
	JSON(w, http.StatusCreated, map[string]any{"id": id, "potId": potID, "entryType": body.EntryType, "amountCents": body.AmountCents, "description": body.Description, "entryDate": storedDate.Format("2006-01-02")})
}

func (s *Server) handleUpdatePotEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		AmountCents int64 `json:"amountCents"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	var potID int64
	var entryType string
	var entryDate time.Time
	if err := s.pool.QueryRow(r.Context(), `SELECT pot_id, entry_type, entry_date FROM pot_ledger WHERE id=$1`, id).Scan(&potID, &entryType, &entryDate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "entry not found")
			return
		}
		dbError(w, "updatePotEntry.lookup", err)
		return
	}
	switch entryType {
	case "allocation", "carryover_out":
		Error(w, http.StatusBadRequest, "bad_request", "cannot edit an automatically generated entry")
		return
	}
	if !isYearWritable(r.Context(), s.pool, w, entryDate.Year()) {
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
	tag, err := s.pool.Exec(r.Context(), `UPDATE pot_ledger SET amount_cents=$2 WHERE id=$1`, id, amount)
	if err != nil {
		mapDBError(w, "updatePotEntry", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	s.auditLog(r.Context(), "pot.entry.update", "pot", potID, map[string]any{"entryId": id, "entryType": entryType, "amountCents": amount})
	JSON(w, http.StatusOK, map[string]any{"id": id, "potId": potID, "entryType": entryType, "amountCents": amount})
}

func (s *Server) handleDeletePotEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var potID int64
	var entryType string
	var entryDate time.Time
	if err := s.pool.QueryRow(r.Context(), `SELECT pot_id, entry_type, entry_date FROM pot_ledger WHERE id=$1`, id).Scan(&potID, &entryType, &entryDate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "entry not found")
			return
		}
		dbError(w, "deletePotEntry.lookup", err)
		return
	}
	switch entryType {
	case "allocation", "carryover_out":
		Error(w, http.StatusBadRequest, "bad_request", "cannot delete an automatically generated entry")
		return
	}
	if !isYearWritable(r.Context(), s.pool, w, entryDate.Year()) {
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM pot_ledger WHERE id=$1`, id)
	if err != nil {
		dbError(w, "deletePotEntry", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	s.auditLog(r.Context(), "pot.entry.delete", "pot", potID, map[string]any{"entryId": id, "entryType": entryType})
	w.WriteHeader(http.StatusNoContent)
}

// ── Description presets ─────────────────────────────────────────
//
// Curated description suggestions per category/income source (e.g.
// "Jumbo"/"Bakker Joost" for Boodschappen), managed in Settings — distinct
// from the auto-derived transaction-description suggestions above, which
// come from typed history and can accumulate typos/one-offs over time.

type descriptionPresetRow struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
}

func (s *Server) handleListCategoryDescriptionPresets(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,description,sort_order FROM category_description_presets WHERE category_id=$1 ORDER BY sort_order,id`, categoryID)
	if err != nil {
		dbError(w, "listCategoryDescriptionPresets", err)
		return
	}
	defer rows.Close()
	out := make([]descriptionPresetRow, 0)
	for rows.Next() {
		var ro descriptionPresetRow
		if err := rows.Scan(&ro.ID, &ro.Description, &ro.SortOrder); err != nil {
			dbError(w, "listCategoryDescriptionPresets.scan", err)
			return
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listCategoryDescriptionPresets", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCategoryDescriptionPreset(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Description string `json:"description"`
		SortOrder   int    `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	body.Description = strings.TrimSpace(body.Description)
	if body.Description == "" {
		Error(w, http.StatusBadRequest, "bad_request", "description is required")
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO category_description_presets (category_id,description,sort_order) VALUES ($1,$2,$3) RETURNING id`,
		categoryID, body.Description, body.SortOrder).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				Error(w, http.StatusConflict, "conflict", "this description already has a preset for this category")
				return
			case pgForeignKeyViolation:
				Error(w, http.StatusNotFound, "not_found", "category not found")
				return
			}
		}
		mapDBError(w, "createCategoryDescriptionPreset", err)
		return
	}
	s.auditLog(r.Context(), "category_description_preset.create", "category", categoryID, map[string]any{"presetId": id, "description": body.Description})
	JSON(w, http.StatusCreated, descriptionPresetRow{ID: id, Description: body.Description, SortOrder: body.SortOrder})
}

func (s *Server) handleDeleteCategoryDescriptionPreset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM category_description_presets WHERE id=$1`, id)
	if err != nil {
		dbError(w, "deleteCategoryDescriptionPreset", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "preset not found")
		return
	}
	s.auditLog(r.Context(), "category_description_preset.delete", "category_description_preset", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListIncomeSourceDescriptionPresets(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,description,sort_order FROM income_source_description_presets WHERE income_source_id=$1 ORDER BY sort_order,id`, sourceID)
	if err != nil {
		dbError(w, "listIncomeSourceDescriptionPresets", err)
		return
	}
	defer rows.Close()
	out := make([]descriptionPresetRow, 0)
	for rows.Next() {
		var ro descriptionPresetRow
		if err := rows.Scan(&ro.ID, &ro.Description, &ro.SortOrder); err != nil {
			dbError(w, "listIncomeSourceDescriptionPresets.scan", err)
			return
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listIncomeSourceDescriptionPresets", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateIncomeSourceDescriptionPreset(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var body struct {
		Description string `json:"description"`
		SortOrder   int    `json:"sortOrder"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	body.Description = strings.TrimSpace(body.Description)
	if body.Description == "" {
		Error(w, http.StatusBadRequest, "bad_request", "description is required")
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO income_source_description_presets (income_source_id,description,sort_order) VALUES ($1,$2,$3) RETURNING id`,
		sourceID, body.Description, body.SortOrder).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				Error(w, http.StatusConflict, "conflict", "this description already has a preset for this income source")
				return
			case pgForeignKeyViolation:
				Error(w, http.StatusNotFound, "not_found", "income source not found")
				return
			}
		}
		mapDBError(w, "createIncomeSourceDescriptionPreset", err)
		return
	}
	s.auditLog(r.Context(), "income_source_description_preset.create", "income_source", sourceID, map[string]any{"presetId": id, "description": body.Description})
	JSON(w, http.StatusCreated, descriptionPresetRow{ID: id, Description: body.Description, SortOrder: body.SortOrder})
}

func (s *Server) handleDeleteIncomeSourceDescriptionPreset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM income_source_description_presets WHERE id=$1`, id)
	if err != nil {
		dbError(w, "deleteIncomeSourceDescriptionPreset", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "preset not found")
		return
	}
	s.auditLog(r.Context(), "income_source_description_preset.delete", "income_source_description_preset", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
