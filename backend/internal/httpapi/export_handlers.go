package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

var dutchMonthNames = []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni",
	"Juli", "Augustus", "September", "Oktober", "November", "December"}

type monthTotal struct {
	Month             int
	Status            *string
	IncomeTotalCents  int64
	ExpenseTotalCents int64
}

// parseMonthsParam parses a comma-separated list of month numbers (1-12).
// An empty input means "all months".
func parseMonthsParam(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		months := make([]int, 12)
		for i := range months {
			months[i] = i + 1
		}
		return months, nil
	}
	seen := map[int]bool{}
	var months []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m, err := strconv.Atoi(part)
		if err != nil || m < 1 || m > 12 {
			return nil, fmt.Errorf("invalid month: %s", part)
		}
		if !seen[m] {
			seen[m] = true
			months = append(months, m)
		}
	}
	if len(months) == 0 {
		return nil, fmt.Errorf("no valid months given")
	}
	sort.Ints(months)
	return months, nil
}

// cellWriter wraps excelize's SetCellValue/SetCellStyle and records the
// first error instead of discarding it. AGENTS.md documents a session lost
// to exactly that failure mode (a swallowed Scan error silently dropped a
// whole export section) — writes after the first error become no-ops, and
// handleExportYear checks Err() before streaming the workbook, so a failed
// write can no longer produce a silently truncated file served as a 200.
type cellWriter struct {
	f   *excelize.File
	err error
}

func newCellWriter(f *excelize.File) *cellWriter {
	return &cellWriter{f: f}
}

func (cw *cellWriter) SetCellValue(sheet, cell string, value interface{}) {
	if cw.err != nil {
		return
	}
	if err := cw.f.SetCellValue(sheet, cell, value); err != nil {
		cw.err = fmt.Errorf("set cell %s!%s: %w", sheet, cell, err)
	}
}

func (cw *cellWriter) SetCellStyle(sheet, topLeftCell, bottomRightCell string, styleID int) {
	if cw.err != nil {
		return
	}
	if err := cw.f.SetCellStyle(sheet, topLeftCell, bottomRightCell, styleID); err != nil {
		cw.err = fmt.Errorf("set cell style %s!%s:%s: %w", sheet, topLeftCell, bottomRightCell, err)
	}
}

func (cw *cellWriter) Err() error {
	return cw.err
}

func (s *Server) handleExportYear(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid year")
		return
	}
	months, err := parseMonthsParam(r.URL.Query().Get("months"))
	if err != nil {
		Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ctx := r.Context()

	f := excelize.NewFile()
	defer f.Close()
	cw := newCellWriter(f)
	headerStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	totals := make([]monthTotal, 0, len(months))

	for _, month := range months {
		var periodID *int64
		var status *string
		if err := s.pool.QueryRow(ctx, `SELECT id, status FROM periods WHERE year=$1 AND month=$2`, year, month).
			Scan(&periodID, &status); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			dbError(w, "export: load period", err)
			return
		}

		mt := monthTotal{Month: month, Status: status}
		if periodID != nil {
			var err error
			mt.IncomeTotalCents, mt.ExpenseTotalCents, err = s.writeMonthSheet(ctx, cw, headerStyle, year, month, *periodID)
			if err != nil {
				dbError(w, "export: write month sheet", err)
				return
			}
		}
		totals = append(totals, mt)
	}

	writeYearOverviewSheet(cw, headerStyle, year, totals)
	if err := s.writePotBalancesSheet(ctx, cw, headerStyle); err != nil {
		dbError(w, "export: write pot balances", err)
		return
	}

	if err := cw.Err(); err != nil {
		dbError(w, "export: write workbook cells", err)
		return
	}

	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(0)

	filename := fmt.Sprintf("home-finance-%d.xlsx", year)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if _, err := f.WriteTo(w); err != nil {
		// The response is already streaming by now, so this can only be logged.
		slog.Error("export: write workbook", "err", err, "year", year)
		return
	}
}

// writeMonthSheet writes one sheet for the given period: income entries,
// budget lines (effective amount — transaction sum for tracked lines,
// otherwise the budgeted amount), and the underlying transactions for
// tracked lines. Returns the income/expense totals for the year-overview sheet.
func (s *Server) writeMonthSheet(ctx context.Context, cw *cellWriter, headerStyle int, year, month int, periodID int64) (incomeTotal, expenseTotal int64, err error) {
	sheetName := fmt.Sprintf("%d-%02d", year, month)
	cw.f.NewSheet(sheetName)
	row := 1

	cw.SetCellValue(sheetName, cellRef("A", row), fmt.Sprintf("%s %d", dutchMonthNames[month], year))
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row += 2

	// ── Income ───────────────────────────────────────────────
	cw.SetCellValue(sheetName, cellRef("A", row), "Inkomsten")
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row++
	cw.SetCellValue(sheetName, cellRef("A", row), "Bron")
	cw.SetCellValue(sheetName, cellRef("B", row), "Bedrag")
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row++

	// Carryover entries are included deliberately, so the sheet's Totaal
	// inkomsten and Surplus match what the app shows for the month and the
	// listed rows actually add up to the printed total. They must not be
	// re-imported as ordinary income, though — closing the previous period
	// generates them, so a reimport would write a second copy on top of the
	// one close creates. That is handled on the read side instead: both
	// parsers skip rows whose label matches isCarryoverLabel, and the label
	// ("Doorlopen maand <maand>", period_handlers.go) is generated server-side
	// and never user-editable, so the guard cannot be dodged by renaming.
	incRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(src.name, ie.label, ''), ie.amount_cents, COALESCE(src.is_itemized,false),
		  COALESCE((SELECT SUM(it.amount_cents) FROM income_transactions it WHERE it.period_id=ie.period_id AND it.source_id=ie.source_id),0)
		FROM income_entries ie LEFT JOIN income_sources src ON src.id = ie.source_id
		WHERE ie.period_id=$1 ORDER BY ie.sort_order, ie.id`, periodID)
	if err != nil {
		return 0, 0, err
	}
	for incRows.Next() {
		var label string
		var cents, txCents int64
		var itemized bool
		if err := incRows.Scan(&label, &cents, &itemized, &txCents); err != nil {
			incRows.Close()
			return 0, 0, err
		}
		effective := cents
		if itemized {
			effective = txCents
		}
		cw.SetCellValue(sheetName, cellRef("A", row), sanitizeExportCell(label))
		cw.SetCellValue(sheetName, cellRef("B", row), float64(effective)/100)
		incomeTotal += effective
		row++
	}
	incRows.Close()
	if err := incRows.Err(); err != nil {
		return 0, 0, err
	}
	cw.SetCellValue(sheetName, cellRef("A", row), "Totaal inkomsten")
	cw.SetCellValue(sheetName, cellRef("B", row), float64(incomeTotal)/100)
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	// ── Budget lines ─────────────────────────────────────────
	cw.SetCellValue(sheetName, cellRef("A", row), "Uitgaven")
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row++
	cw.SetCellValue(sheetName, cellRef("A", row), "Categorie")
	cw.SetCellValue(sheetName, cellRef("B", row), "Bedrag")
	cw.SetCellValue(sheetName, cellRef("C", row), "Type")
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("C", row), headerStyle)
	row++

	blRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(c.name, bl.label, ''), bl.amount_cents, bl.tracks_transactions,
		  COALESCE((SELECT SUM(t.amount_cents) FROM transactions t JOIN category_rollup cr ON cr.member_id=t.category_id WHERE t.period_id=bl.period_id AND cr.category_id=bl.category_id),0)
		FROM budget_lines bl LEFT JOIN categories c ON c.id = bl.category_id
		WHERE bl.period_id=$1 ORDER BY bl.sort_order, bl.id`, periodID)
	if err != nil {
		return 0, 0, err
	}
	for blRows.Next() {
		var label string
		var amountCents, txCents int64
		var tracks bool
		if err := blRows.Scan(&label, &amountCents, &tracks, &txCents); err != nil {
			blRows.Close()
			return 0, 0, err
		}
		effective := amountCents
		typeLabel := "Vast"
		if tracks {
			effective = txCents
			typeLabel = "Boekingen"
		}
		cw.SetCellValue(sheetName, cellRef("A", row), sanitizeExportCell(label))
		cw.SetCellValue(sheetName, cellRef("B", row), float64(effective)/100)
		cw.SetCellValue(sheetName, cellRef("C", row), typeLabel)
		expenseTotal += effective
		row++
	}
	blRows.Close()
	if err := blRows.Err(); err != nil {
		return 0, 0, err
	}
	cw.SetCellValue(sheetName, cellRef("A", row), "Totaal uitgaven")
	cw.SetCellValue(sheetName, cellRef("B", row), float64(expenseTotal)/100)
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	cw.SetCellValue(sheetName, cellRef("A", row), "Surplus")
	cw.SetCellValue(sheetName, cellRef("B", row), float64(incomeTotal-expenseTotal)/100)
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	// ── Transactions ────────────────────────────────────────
	// Written whenever the period has any, regardless of whether a budget
	// line tracks them: the Settings UI pairs this export with
	// import-with-wipe, so skipping the section for a period with no tracked
	// line meant the wipe deleted transactions the workbook never carried.
	txRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(c.name,''), t.description, t.amount_cents, t.tx_date
		FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.period_id=$1 ORDER BY t.tx_date NULLS LAST, t.id`, periodID)
	if err != nil {
		return 0, 0, err
	}
	type txLine struct {
		category, desc, date string
		cents                int64
	}
	var txLines []txLine
	for txRows.Next() {
		var line txLine
		var txDate *time.Time
		if err := txRows.Scan(&line.category, &line.desc, &line.cents, &txDate); err != nil {
			txRows.Close()
			return 0, 0, err
		}
		if txDate != nil {
			line.date = txDate.Format("2006-01-02")
		}
		txLines = append(txLines, line)
	}
	txRows.Close()
	if err := txRows.Err(); err != nil {
		return 0, 0, err
	}

	if len(txLines) > 0 {
		cw.SetCellValue(sheetName, cellRef("A", row), "Transacties")
		cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
		row++
		cw.SetCellValue(sheetName, cellRef("A", row), "Datum")
		cw.SetCellValue(sheetName, cellRef("B", row), "Categorie")
		cw.SetCellValue(sheetName, cellRef("C", row), "Omschrijving")
		cw.SetCellValue(sheetName, cellRef("D", row), "Bedrag")
		cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("D", row), headerStyle)
		row++
		for _, line := range txLines {
			cw.SetCellValue(sheetName, cellRef("A", row), line.date)
			cw.SetCellValue(sheetName, cellRef("B", row), sanitizeExportCell(line.category))
			cw.SetCellValue(sheetName, cellRef("C", row), sanitizeExportCell(line.desc))
			cw.SetCellValue(sheetName, cellRef("D", row), float64(line.cents)/100)
			row++
		}
	}

	// ── Line items of itemized income sources ───────────────
	// The income section above collapses an itemized source to its summed
	// total; without these rows a reimport recreates the source as itemized
	// with zero line items, and EffectiveIncomeCentsSQL then values it at 0.
	itxRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(src.name,''), it.description, it.amount_cents, it.tx_date
		FROM income_transactions it LEFT JOIN income_sources src ON src.id = it.source_id
		WHERE it.period_id=$1 ORDER BY it.tx_date NULLS LAST, it.id`, periodID)
	if err != nil {
		return 0, 0, err
	}
	type incomeTxLine struct {
		source, desc, date string
		cents              int64
	}
	var incomeTxLines []incomeTxLine
	for itxRows.Next() {
		var line incomeTxLine
		var txDate *time.Time
		if err := itxRows.Scan(&line.source, &line.desc, &line.cents, &txDate); err != nil {
			itxRows.Close()
			return 0, 0, err
		}
		if txDate != nil {
			line.date = txDate.Format("2006-01-02")
		}
		incomeTxLines = append(incomeTxLines, line)
	}
	itxRows.Close()
	if err := itxRows.Err(); err != nil {
		return 0, 0, err
	}

	if len(incomeTxLines) > 0 {
		row++ // blank separator row
		cw.SetCellValue(sheetName, cellRef("A", row), "Inkomsten transacties")
		cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
		row++
		cw.SetCellValue(sheetName, cellRef("A", row), "Datum")
		cw.SetCellValue(sheetName, cellRef("B", row), "Bron")
		cw.SetCellValue(sheetName, cellRef("C", row), "Omschrijving")
		cw.SetCellValue(sheetName, cellRef("D", row), "Bedrag")
		cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("D", row), headerStyle)
		row++
		for _, line := range incomeTxLines {
			cw.SetCellValue(sheetName, cellRef("A", row), line.date)
			cw.SetCellValue(sheetName, cellRef("B", row), sanitizeExportCell(line.source))
			cw.SetCellValue(sheetName, cellRef("C", row), sanitizeExportCell(line.desc))
			cw.SetCellValue(sheetName, cellRef("D", row), float64(line.cents)/100)
			row++
		}
	}

	cw.f.SetColWidth(sheetName, "A", "A", 28)
	cw.f.SetColWidth(sheetName, "B", "D", 16)

	return incomeTotal, expenseTotal, nil
}

// sanitizeExportCell defuses spreadsheet formula injection in user-entered
// text. excelize writes these as string-typed cells, so Excel itself won't
// evaluate them — but "Save As → CSV", the usual way this workbook gets shared
// with an accountant, drops the type and a leading =/+/-/@ (or a leading tab /
// carriage return) turns the cell back into a formula in whatever opens it
// next. A leading apostrophe is the standard inert prefix.
func sanitizeExportCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func writeYearOverviewSheet(cw *cellWriter, headerStyle int, year int, totals []monthTotal) {
	sheetName := "Jaaroverzicht"
	cw.f.NewSheet(sheetName)
	cw.SetCellValue(sheetName, "A1", fmt.Sprintf("Jaaroverzicht %d", year))
	cw.SetCellStyle(sheetName, "A1", "A1", headerStyle)

	headers := []string{"Maand", "Status", "Inkomsten", "Uitgaven", "Surplus"}
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		cw.SetCellValue(sheetName, cellRef(col, 3), h)
	}
	cw.SetCellStyle(sheetName, "A3", "E3", headerStyle)

	row := 4
	var yearIncome, yearExpense int64
	for _, mt := range totals {
		status := "—"
		if mt.Status != nil {
			status = *mt.Status
		}
		surplus := mt.IncomeTotalCents - mt.ExpenseTotalCents
		cw.SetCellValue(sheetName, cellRef("A", row), dutchMonthNames[mt.Month])
		cw.SetCellValue(sheetName, cellRef("B", row), status)
		cw.SetCellValue(sheetName, cellRef("C", row), float64(mt.IncomeTotalCents)/100)
		cw.SetCellValue(sheetName, cellRef("D", row), float64(mt.ExpenseTotalCents)/100)
		cw.SetCellValue(sheetName, cellRef("E", row), float64(surplus)/100)
		yearIncome += mt.IncomeTotalCents
		yearExpense += mt.ExpenseTotalCents
		row++
	}
	cw.SetCellValue(sheetName, cellRef("A", row), "Totaal")
	cw.SetCellValue(sheetName, cellRef("C", row), float64(yearIncome)/100)
	cw.SetCellValue(sheetName, cellRef("D", row), float64(yearExpense)/100)
	cw.SetCellValue(sheetName, cellRef("E", row), float64(yearIncome-yearExpense)/100)
	cw.SetCellStyle(sheetName, cellRef("A", row), cellRef("E", row), headerStyle)

	cw.f.SetColWidth(sheetName, "A", "A", 14)
	cw.f.SetColWidth(sheetName, "B", "E", 14)
}

func (s *Server) writePotBalancesSheet(ctx context.Context, cw *cellWriter, headerStyle int) error {
	sheetName := "Potbalansen"
	cw.f.NewSheet(sheetName)
	cw.SetCellValue(sheetName, "A1", "Naam")
	cw.SetCellValue(sheetName, "B1", "Type")
	cw.SetCellValue(sheetName, "C1", "Saldo")
	cw.SetCellStyle(sheetName, "A1", "C1", headerStyle)

	rows, err := s.pool.Query(ctx, `
		SELECT p.name, p.kind, COALESCE(SUM(pl.amount_cents),0)
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL GROUP BY p.id,p.name,p.kind,p.sort_order ORDER BY p.sort_order,p.id`)
	if err != nil {
		return err
	}
	row := 2
	for rows.Next() {
		var name, kind string
		var balance int64
		if err := rows.Scan(&name, &kind, &balance); err != nil {
			rows.Close()
			return err
		}
		kindLabel := "Normaal"
		if kind == "carryover" {
			kindLabel = "Doorlopend"
		}
		cw.SetCellValue(sheetName, cellRef("A", row), sanitizeExportCell(name))
		cw.SetCellValue(sheetName, cellRef("B", row), kindLabel)
		cw.SetCellValue(sheetName, cellRef("C", row), float64(balance)/100)
		row++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	cw.f.SetColWidth(sheetName, "A", "A", 24)
	cw.f.SetColWidth(sheetName, "B", "C", 16)
	return nil
}

func cellRef(col string, row int) string {
	return fmt.Sprintf("%s%d", col, row)
}
