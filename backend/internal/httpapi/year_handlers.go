package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleLockYear(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
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
	var body struct {
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
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
	if _, err := s.pool.Exec(ctx, `DELETE FROM locked_years WHERE year=$1`, year); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	s.auditLog(ctx, "year.unlock", "year", int64(year), nil)
	w.WriteHeader(http.StatusNoContent)
}
