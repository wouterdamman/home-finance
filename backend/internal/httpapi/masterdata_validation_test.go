package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func decodeErrorBody(t *testing.T, rec *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code, body.Error.Message
}

func TestRequireName(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantOK     bool
		wantStored string
	}{
		{name: "plain", in: "Boodschappen", wantOK: true, wantStored: "Boodschappen"},
		{name: "trimmed", in: "  Boodschappen \t", wantOK: true, wantStored: "Boodschappen"},
		{name: "empty", in: "", wantOK: false},
		{name: "whitespace only", in: "   ", wantOK: false},
		{name: "newline only", in: "\n\t ", wantOK: false},
		{name: "too long", in: strings.Repeat("a", maxNameLen+1), wantOK: false},
		{name: "at the cap", in: strings.Repeat("a", maxNameLen), wantOK: true, wantStored: strings.Repeat("a", maxNameLen)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			got, ok := requireName(rec, tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400", rec.Code)
				}
				if code, _ := decodeErrorBody(t, rec); code != "bad_request" {
					t.Fatalf("code = %q, want bad_request", code)
				}
				return
			}
			if got != tc.wantStored {
				t.Fatalf("name = %q, want %q", got, tc.wantStored)
			}
			if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
				t.Fatalf("unexpected response written: %d %q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestEntryYear(t *testing.T) {
	ptr := func(s string) *string { return &s }
	thisYear := time.Now().Year()
	cases := []struct {
		name     string
		in       *string
		wantYear int
		wantOK   bool
	}{
		{name: "nil defaults to today", in: nil, wantYear: thisYear, wantOK: true},
		{name: "empty defaults to today", in: ptr(""), wantYear: thisYear, wantOK: true},
		{name: "explicit date", in: ptr("2023-07-04"), wantYear: 2023, wantOK: true},
		{name: "first of january", in: ptr("2024-01-01"), wantYear: 2024, wantOK: true},
		{name: "last of december", in: ptr("2024-12-31"), wantYear: 2024, wantOK: true},
		{name: "not a date", in: ptr("yesterday"), wantOK: false},
		{name: "wrong layout", in: ptr("04-07-2023"), wantOK: false},
		{name: "timestamp", in: ptr("2023-07-04T00:00:00Z"), wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			year, ok := entryYear(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK && year != tc.wantYear {
				t.Fatalf("year = %d, want %d", year, tc.wantYear)
			}
		})
	}
}

// The whole point of mapDBError is that the driver's own text — which names
// tables, constraints, columns and the conflicting value — never reaches the
// client, and that a transient failure isn't reported as a 409 the caller
// would retry differently.
func TestMapDBError(t *testing.T) {
	leaky := "duplicate key value violates unique constraint \"categories_name_key\" DETAIL: Key (name)=(Boodschappen) already exists."
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unique violation",
			err:        &pgconn.PgError{Code: pgUniqueViolation, Message: leaky, TableName: "categories", ConstraintName: "categories_name_key"},
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
		},
		{
			name:       "foreign key violation",
			err:        &pgconn.PgError{Code: pgForeignKeyViolation, Message: leaky, TableName: "pot_ledger"},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "check violation",
			err:        &pgconn.PgError{Code: pgCheckViolation, Message: leaky, ConstraintName: "pots_kind_check"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:       "numeric out of range",
			err:        &pgconn.PgError{Code: pgNumericOutOfRange, Message: leaky},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name:       "connection failure is not a conflict",
			err:        errors.New("failed to connect to `host=db`: dial error"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "db_error",
		},
		{
			name:       "unmapped pg code falls through to db_error",
			err:        &pgconn.PgError{Code: "40001", Message: leaky},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "db_error",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mapDBError(rec, "testOp", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			code, message := decodeErrorBody(t, rec)
			if code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
			for _, leak := range []string{"categories_name_key", "Boodschappen", "pot_ledger", "pots_kind_check", "host=db"} {
				if strings.Contains(message, leak) {
					t.Fatalf("message %q leaks %q", message, leak)
				}
			}
		})
	}
}

func TestArchiveAction(t *testing.T) {
	now := time.Now()
	if got := archiveAction("pot", &now); got != "pot.archive" {
		t.Fatalf("archived action = %q, want pot.archive", got)
	}
	if got := archiveAction("pot", nil); got != "pot.unarchive" {
		t.Fatalf("unarchived action = %q, want pot.unarchive", got)
	}
}
