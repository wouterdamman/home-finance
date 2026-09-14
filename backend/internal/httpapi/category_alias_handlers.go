package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ── Category aliases ─────────────────────────────────────────────
//
// Maps an import Details-table header (e.g. "Bunq") to the parent category
// it should nest under (e.g. "Boodschappen") instead of becoming its own
// disconnected top-level category — see mapper.go's alias lookup. No
// archive/soft-delete needed: an alias has no historical dependents of its
// own, it's just a lookup rule consulted at import time.

func (s *Server) handleListCategoryAliases(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID               int64  `json:"id"`
		AliasName        string `json:"aliasName"`
		ParentCategoryID int64  `json:"parentCategoryId"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT id,alias_name,parent_category_id FROM category_aliases ORDER BY alias_name`)
	if err != nil {
		dbError(w, "listCategoryAliases", err)
		return
	}
	defer rows.Close()
	out := make([]row, 0)
	for rows.Next() {
		var ro row
		if err := rows.Scan(&ro.ID, &ro.AliasName, &ro.ParentCategoryID); err != nil {
			dbError(w, "listCategoryAliases.scan", err)
			return
		}
		out = append(out, ro)
	}
	if err := rows.Err(); err != nil {
		dbError(w, "listCategoryAliases", err)
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCategoryAlias(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AliasName        string `json:"aliasName"`
		ParentCategoryID int64  `json:"parentCategoryId"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	aliasName := strings.TrimSpace(body.AliasName)
	if aliasName == "" || body.ParentCategoryID == 0 {
		Error(w, http.StatusBadRequest, "bad_request", "aliasName and parentCategoryId are required")
		return
	}
	if len(aliasName) > maxNameLen {
		Error(w, http.StatusBadRequest, "bad_request", "aliasName is too long")
		return
	}
	var hasParent bool
	if err := s.pool.QueryRow(r.Context(), `SELECT parent_id IS NOT NULL FROM categories WHERE id=$1`, body.ParentCategoryID).Scan(&hasParent); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusBadRequest, "bad_request", "invalid parent category")
			return
		}
		dbError(w, "createCategoryAlias.parent", err)
		return
	}
	if hasParent {
		Error(w, http.StatusBadRequest, "bad_request", "parent category must not itself be a child")
		return
	}
	var id int64
	if err := s.pool.QueryRow(r.Context(),
		`INSERT INTO category_aliases (alias_name, parent_category_id) VALUES ($1,$2) RETURNING id`,
		aliasName, body.ParentCategoryID).Scan(&id); err != nil {
		mapDBError(w, "createCategoryAlias", err)
		return
	}
	s.auditLog(r.Context(), "category_alias.create", "category_alias", id, map[string]any{"aliasName": aliasName, "parentCategoryId": body.ParentCategoryID})
	JSON(w, http.StatusCreated, map[string]any{"id": id, "aliasName": aliasName, "parentCategoryId": body.ParentCategoryID})
}

func (s *Server) handleDeleteCategoryAlias(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM category_aliases WHERE id=$1`, id)
	if err != nil {
		dbError(w, "deleteCategoryAlias", err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "alias not found")
		return
	}
	s.auditLog(r.Context(), "category_alias.delete", "category_alias", id, nil)
	w.WriteHeader(http.StatusNoContent)
}
