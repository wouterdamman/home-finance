package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/wouterdamman/home-finance/internal/importer"
)

// maxImportUploadBytes caps the uploaded xlsx at 20MB — generous for a
// family budget workbook, tight enough to avoid an accidental huge upload
// tying up the request.
const maxImportUploadBytes = 20 << 20

func (s *Server) handleImportXLSX(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportUploadBytes)
	if err := r.ParseMultipartForm(maxImportUploadBytes); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "file too large or malformed upload")
		return
	}

	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	wipe := r.FormValue("wipe") == "true"
	resetMaster := r.FormValue("resetMaster") == "true"
	closeThrough, _ := strconv.Atoi(r.FormValue("closeThrough"))

	if wipe || resetMaster {
		if !s.requireFreshReauth(w, r) {
			return
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "missing file")
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "import-*.xlsx")
	if err != nil {
		Error(w, http.StatusInternalServerError, "server_error", "could not create temp file")
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, file); err != nil {
		Error(w, http.StatusInternalServerError, "server_error", "could not save upload")
		return
	}
	if err := tmp.Close(); err != nil {
		Error(w, http.StatusInternalServerError, "server_error", "could not save upload")
		return
	}

	// The parser error names the server-side temp file and excelize internals;
	// neither is the uploader's business, and neither helps them fix the file.
	sheets, skipped, err := importer.DetectAndParse(tmp.Name())
	if err != nil {
		slog.Error("import: parse xlsx", "err", err, "filename", header.Filename)
		Error(w, http.StatusBadRequest, "parse_error",
			"could not read this file as a supported Excel workbook")
		return
	}

	ctx := r.Context()
	report, err := importer.Run(ctx, s.pool, sheets, importer.ImportOptions{
		Year:         year,
		Wipe:         wipe,
		ResetMaster:  resetMaster,
		CloseThrough: closeThrough,
	})
	if err != nil {
		slog.Error("import: run", "err", err, "year", year, "filename", header.Filename)
		// A ValidationError is written by the importer and names the month/row
		// at fault; anything else is a driver or constraint error, whose text
		// embeds cell content and raw Postgres detail.
		var ve *importer.ValidationError
		if errors.As(err, &ve) {
			Error(w, http.StatusBadRequest, "import_invalid", ve.Error())
			return
		}
		Error(w, http.StatusInternalServerError, "import_error",
			"import failed and was rolled back; no data was changed")
		return
	}
	report.SkippedSheets = skipped

	// resetMasterdata is not scoped to the requested year: it deletes every
	// period of every year plus pots, categories and income sources. Record it
	// separately, with the row counts, so the trail can't be read as "one year
	// was re-imported".
	if resetMaster {
		s.auditLog(ctx, "import.reset_masterdata", "masterdata", 0, map[string]any{
			"filename":    header.Filename,
			"requestYear": year,
			"scope":       "all years",
			"deletedRows": report.ResetCounts,
			"notCleared":  []string{"kids", "kid_savings_ledger", "years", "locked_years", "users", "audit_log"},
		})
	}

	s.auditLog(ctx, "import.xlsx", "year", int64(year), map[string]any{
		"filename":     header.Filename,
		"wipe":         wipe,
		"resetMaster":  resetMaster,
		"closeThrough": closeThrough,
		"months":       len(report.Months),
		"problems":     len(report.Problems),
	})

	JSON(w, http.StatusOK, report)
}
