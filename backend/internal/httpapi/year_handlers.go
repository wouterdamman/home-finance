package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListYears(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT year FROM years ORDER BY year`)
	if err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	defer rows.Close()
	out := make([]int, 0)
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			Error(w, http.StatusInternalServerError, "scan_error", err.Error())
			return
		}
		out = append(out, y)
	}
	if err := rows.Err(); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateYear(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Year int `json:"year"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if body.Year < 2000 || body.Year > 2100 {
		Error(w, http.StatusBadRequest, "bad_request", "year out of range")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, body.Year); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	JSON(w, http.StatusCreated, map[string]any{"year": body.Year})
}

func (s *Server) handleLockYear(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	if !s.requireFreshReauth(w, r) {
		return
	}

	ctx := r.Context()

	if locked, _ := isYearLocked(ctx, s.pool, year); locked {
		Error(w, http.StatusConflict, "year_already_locked", "year is already locked")
		return
	}

	var openCount int
	s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM periods WHERE year=$1 AND status='open'`, year).Scan(&openCount)
	if openCount > 0 {
		Error(w, http.StatusConflict, "open_periods_exist", "close all periods before locking the year")
		return
	}

	if _, err := s.pool.Exec(ctx, `INSERT INTO locked_years (year) VALUES ($1) ON CONFLICT DO NOTHING`, year); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(ctx, "year.lock", "year", int64(year), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnlockYear(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	if !s.requireFreshReauth(w, r) {
		return
	}

	ctx := r.Context()
	if _, err := s.pool.Exec(ctx, `DELETE FROM locked_years WHERE year=$1`, year); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(ctx, "year.unlock", "year", int64(year), nil)
	w.WriteHeader(http.StatusNoContent)
}
