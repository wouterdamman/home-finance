package httpapi

import (
	"context"
	"errors"
	"fmt"
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
	headerStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	totals := make([]monthTotal, 0, len(months))

	for _, month := range months {
		var periodID *int64
		var status *string
		if err := s.pool.QueryRow(ctx, `SELECT id, status FROM periods WHERE year=$1 AND month=$2`, year, month).
			Scan(&periodID, &status); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}

		mt := monthTotal{Month: month, Status: status}
		if periodID != nil {
			var err error
			mt.IncomeTotalCents, mt.ExpenseTotalCents, err = s.writeMonthSheet(ctx, f, headerStyle, year, month, *periodID)
			if err != nil {
				Error(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		}
		totals = append(totals, mt)
	}

	writeYearOverviewSheet(f, headerStyle, year, totals)
	if err := s.writePotBalancesSheet(ctx, f, headerStyle); err != nil {
		Error(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}

	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(0)

	filename := fmt.Sprintf("home-finance-%d.xlsx", year)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if _, err := f.WriteTo(w); err != nil {
		Error(w, http.StatusInternalServerError, "export_error", err.Error())
		return
	}
}

// writeMonthSheet writes one sheet for the given period: income entries,
// budget lines (effective amount — transaction sum for tracked lines,
// otherwise the budgeted amount), and the underlying transactions for
// tracked lines. Returns the income/expense totals for the year-overview sheet.
func (s *Server) writeMonthSheet(ctx context.Context, f *excelize.File, headerStyle int, year, month int, periodID int64) (incomeTotal, expenseTotal int64, err error) {
	sheetName := fmt.Sprintf("%d-%02d", year, month)
	f.NewSheet(sheetName)
	row := 1

	f.SetCellValue(sheetName, cellRef("A", row), fmt.Sprintf("%s %d", dutchMonthNames[month], year))
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row += 2

	// ── Income ───────────────────────────────────────────────
	f.SetCellValue(sheetName, cellRef("A", row), "Inkomsten")
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row++
	f.SetCellValue(sheetName, cellRef("A", row), "Bron")
	f.SetCellValue(sheetName, cellRef("B", row), "Bedrag")
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row++

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
		f.SetCellValue(sheetName, cellRef("A", row), label)
		f.SetCellValue(sheetName, cellRef("B", row), float64(effective)/100)
		incomeTotal += effective
		row++
	}
	incRows.Close()
	if err := incRows.Err(); err != nil {
		return 0, 0, err
	}
	f.SetCellValue(sheetName, cellRef("A", row), "Totaal inkomsten")
	f.SetCellValue(sheetName, cellRef("B", row), float64(incomeTotal)/100)
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	// ── Budget lines ─────────────────────────────────────────
	f.SetCellValue(sheetName, cellRef("A", row), "Uitgaven")
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
	row++
	f.SetCellValue(sheetName, cellRef("A", row), "Categorie")
	f.SetCellValue(sheetName, cellRef("B", row), "Bedrag")
	f.SetCellValue(sheetName, cellRef("C", row), "Type")
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("C", row), headerStyle)
	row++

	type trackedCat struct {
		label string
	}
	var trackedCats []trackedCat

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
			trackedCats = append(trackedCats, trackedCat{label: label})
		}
		f.SetCellValue(sheetName, cellRef("A", row), label)
		f.SetCellValue(sheetName, cellRef("B", row), float64(effective)/100)
		f.SetCellValue(sheetName, cellRef("C", row), typeLabel)
		expenseTotal += effective
		row++
	}
	blRows.Close()
	if err := blRows.Err(); err != nil {
		return 0, 0, err
	}
	f.SetCellValue(sheetName, cellRef("A", row), "Totaal uitgaven")
	f.SetCellValue(sheetName, cellRef("B", row), float64(expenseTotal)/100)
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	f.SetCellValue(sheetName, cellRef("A", row), "Surplus")
	f.SetCellValue(sheetName, cellRef("B", row), float64(incomeTotal-expenseTotal)/100)
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("B", row), headerStyle)
	row += 2

	// ── Transactions for tracked categories ─────────────────
	if len(trackedCats) > 0 {
		f.SetCellValue(sheetName, cellRef("A", row), "Transacties")
		f.SetCellStyle(sheetName, cellRef("A", row), cellRef("A", row), headerStyle)
		row++
		f.SetCellValue(sheetName, cellRef("A", row), "Datum")
		f.SetCellValue(sheetName, cellRef("B", row), "Categorie")
		f.SetCellValue(sheetName, cellRef("C", row), "Omschrijving")
		f.SetCellValue(sheetName, cellRef("D", row), "Bedrag")
		f.SetCellStyle(sheetName, cellRef("A", row), cellRef("D", row), headerStyle)
		row++

		txRows, err := s.pool.Query(ctx, `
			SELECT COALESCE(c.name,''), t.description, t.amount_cents, t.tx_date
			FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
			WHERE t.period_id=$1 ORDER BY t.tx_date NULLS LAST, t.id`, periodID)
		if err != nil {
			return 0, 0, err
		}
		for txRows.Next() {
			var catLabel, desc string
			var cents int64
			var txDate *time.Time
			if err := txRows.Scan(&catLabel, &desc, &cents, &txDate); err != nil {
				txRows.Close()
				return 0, 0, err
			}
			date := ""
			if txDate != nil {
				date = txDate.Format("2006-01-02")
			}
			f.SetCellValue(sheetName, cellRef("A", row), date)
			f.SetCellValue(sheetName, cellRef("B", row), catLabel)
			f.SetCellValue(sheetName, cellRef("C", row), desc)
			f.SetCellValue(sheetName, cellRef("D", row), float64(cents)/100)
			row++
		}
		txRows.Close()
		if err := txRows.Err(); err != nil {
			return 0, 0, err
		}
	}

	f.SetColWidth(sheetName, "A", "A", 28)
	f.SetColWidth(sheetName, "B", "D", 16)

	return incomeTotal, expenseTotal, nil
}

func writeYearOverviewSheet(f *excelize.File, headerStyle int, year int, totals []monthTotal) {
	sheetName := "Jaaroverzicht"
	f.NewSheet(sheetName)
	f.SetCellValue(sheetName, "A1", fmt.Sprintf("Jaaroverzicht %d", year))
	f.SetCellStyle(sheetName, "A1", "A1", headerStyle)

	headers := []string{"Maand", "Status", "Inkomsten", "Uitgaven", "Surplus"}
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetCellValue(sheetName, cellRef(col, 3), h)
	}
	f.SetCellStyle(sheetName, "A3", "E3", headerStyle)

	row := 4
	var yearIncome, yearExpense int64
	for _, mt := range totals {
		status := "—"
		if mt.Status != nil {
			status = *mt.Status
		}
		surplus := mt.IncomeTotalCents - mt.ExpenseTotalCents
		f.SetCellValue(sheetName, cellRef("A", row), dutchMonthNames[mt.Month])
		f.SetCellValue(sheetName, cellRef("B", row), status)
		f.SetCellValue(sheetName, cellRef("C", row), float64(mt.IncomeTotalCents)/100)
		f.SetCellValue(sheetName, cellRef("D", row), float64(mt.ExpenseTotalCents)/100)
		f.SetCellValue(sheetName, cellRef("E", row), float64(surplus)/100)
		yearIncome += mt.IncomeTotalCents
		yearExpense += mt.ExpenseTotalCents
		row++
	}
	f.SetCellValue(sheetName, cellRef("A", row), "Totaal")
	f.SetCellValue(sheetName, cellRef("C", row), float64(yearIncome)/100)
	f.SetCellValue(sheetName, cellRef("D", row), float64(yearExpense)/100)
	f.SetCellValue(sheetName, cellRef("E", row), float64(yearIncome-yearExpense)/100)
	f.SetCellStyle(sheetName, cellRef("A", row), cellRef("E", row), headerStyle)

	f.SetColWidth(sheetName, "A", "A", 14)
	f.SetColWidth(sheetName, "B", "E", 14)
}

func (s *Server) writePotBalancesSheet(ctx context.Context, f *excelize.File, headerStyle int) error {
	sheetName := "Potbalansen"
	f.NewSheet(sheetName)
	f.SetCellValue(sheetName, "A1", "Naam")
	f.SetCellValue(sheetName, "B1", "Type")
	f.SetCellValue(sheetName, "C1", "Saldo")
	f.SetCellStyle(sheetName, "A1", "C1", headerStyle)

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
		f.SetCellValue(sheetName, cellRef("A", row), name)
		f.SetCellValue(sheetName, cellRef("B", row), kindLabel)
		f.SetCellValue(sheetName, cellRef("C", row), float64(balance)/100)
		row++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	f.SetColWidth(sheetName, "A", "A", 24)
	f.SetColWidth(sheetName, "B", "C", 16)
	return nil
}

func cellRef(col string, row int) string {
	return fmt.Sprintf("%s%d", col, row)
}
