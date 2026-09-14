package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	// Authenticated API responses carry household financial data. Nothing
	// about them is safe for a shared/intermediary cache to retain or replay,
	// and without an explicit directive RFC 9111 heuristic freshness applies.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

// dbError logs the real driver error server-side and returns a generic message
// to the caller. Raw pgx errors name tables, columns, constraints and — on a
// unique violation — the conflicting value itself, so they must never reach a
// client. Use this for any error the caller can't act on; keep an explicit
// Error(...) call for cases that have been deliberately mapped to a meaningful
// code (not_found, in_use, period_closed, ...).
func dbError(w http.ResponseWriter, op string, err error) {
	slog.Error("db error", "op", op, "err", err)
	Error(w, http.StatusInternalServerError, "db_error", "internal error")
}

// errBodyTooLarge is returned by DecodeJSON when the request body exceeded the
// limit imposed by the bodyLimit middleware.
var errBodyTooLarge = errors.New("request body too large")

func DecodeJSON(r *http.Request, v any) error {
	// r.Body is wrapped by the bodyLimit middleware, so a hostile body is cut
	// off at the socket rather than buffered whole by the decoder.
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errBodyTooLarge
		}
		return err
	}
	return nil
}

// badRequest reports a decode failure without echoing the decoder's internal
// error text back to the caller.
func badRequest(w http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		Error(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body too large")
		return
	}
	Error(w, http.StatusBadRequest, "bad_request", "invalid request body")
}
