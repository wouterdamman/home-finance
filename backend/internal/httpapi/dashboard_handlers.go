package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/wouterdamman/home-finance/internal/auth"
)

// maxDashboardWidgets bounds a stored board; the UI has no natural limit, but
// an unbounded array is just a free JSONB store.
const maxDashboardWidgets = 200

// The widget schema is owned by the frontend (it sanitises on load), so the
// server stores the array opaquely and only enforces shape and size.
func (s *Server) handleGetTrendsDashboard(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var widgets json.RawMessage
	err := s.pool.QueryRow(r.Context(), `SELECT widgets FROM trends_dashboards WHERE user_id = $1`, uid).Scan(&widgets)
	if errors.Is(err, pgx.ErrNoRows) {
		JSON(w, http.StatusOK, map[string]any{"widgets": nil})
		return
	}
	if err != nil {
		dbError(w, "getTrendsDashboard", err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"widgets": widgets})
}

func (s *Server) handlePutTrendsDashboard(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var body struct {
		Widgets []json.RawMessage `json:"widgets"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		badRequest(w, err)
		return
	}
	if body.Widgets == nil {
		Error(w, http.StatusBadRequest, "bad_request", "widgets is required")
		return
	}
	if len(body.Widgets) > maxDashboardWidgets {
		Error(w, http.StatusBadRequest, "bad_request", "too many widgets")
		return
	}
	for _, raw := range body.Widgets {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
			Error(w, http.StatusBadRequest, "bad_request", "every widget must be an object")
			return
		}
	}
	widgets, err := json.Marshal(body.Widgets)
	if err != nil {
		badRequest(w, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(),
		`INSERT INTO trends_dashboards (user_id, widgets) VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET widgets = EXCLUDED.widgets, updated_at = now()`,
		uid, widgets); err != nil {
		dbError(w, "putTrendsDashboard", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
