package httpapi

import (
	"fmt"
	"io"
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

	sheets, skipped, err := importer.DetectAndParse(tmp.Name())
	if err != nil {
		Error(w, http.StatusBadRequest, "parse_error", fmt.Sprintf("could not parse xlsx: %v", err))
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
		Error(w, http.StatusInternalServerError, "import_error", err.Error())
		return
	}
	report.SkippedSheets = skipped

	s.auditLog(ctx, "import.xlsx", "year", int64(year), map[string]any{
		"filename":     header.Filename,
		"wipe":         wipe,
		"resetMaster":  resetMaster,
		"closeThrough": closeThrough,
		"months":       len(report.Months),
	})

	JSON(w, http.StatusOK, report)
}
