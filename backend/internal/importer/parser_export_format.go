package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// exportSheetRe matches month sheets written by this app's own Excel export
// (backend/internal/httpapi/export_handlers.go), named e.g. "2026-07" —
// distinct from the legacy Fam_Finance sheet naming ("7-26-Overview").
var exportSheetRe = regexp.MustCompile(`^(\d{4})-(\d{2})$`)

// ParseXLSXExportFormat reads sheets produced by this app's own export
// (one sheet per month: Inkomsten/Uitgaven/Transacties sections) and
// returns the same SheetData shape ParseXLSX produces, so Run() needs no
// changes to consume either source. Pot splits are not present in the
// export format, so periods parsed this way import without splits — a
// close on such a period allocates nothing to pots (see mapper.go).
func ParseXLSXExportFormat(path string) ([]SheetData, []string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var sheets []SheetData
	var skipped []string
	for _, sheet := range f.GetSheetList() {
		m := exportSheetRe.FindStringSubmatch(sheet)
		if m == nil {
			skipped = append(skipped, sheet)
			continue
		}
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])

		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}

		incomes, lines, txs := parseExportSheet(rows)
		sheets = append(sheets,
			SheetData{Year: year, Month: month, Kind: "Overview", Incomes: incomes, Lines: lines},
			SheetData{Year: year, Month: month, Kind: "Details", Txs: txs},
		)
	}
	return sheets, skipped, nil
}

func parseExportSheet(rows [][]string) ([]IncomeRow, []BudgetLineRow, []TxRow) {
	var incomes []IncomeRow
	var lines []BudgetLineRow
	var txs []TxRow
	mode := ""

	col := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	for _, row := range rows {
		a := col(row, 0)
		al := strings.ToLower(a)

		switch {
		case al == "inkomsten" || al == "income":
			mode = "income"
			continue
		case al == "uitgaven" || al == "expenses":
			mode = "expense"
			continue
		case al == "transacties" || al == "transactions":
			mode = "tx"
			continue
		case al == "bron" || al == "source" || al == "categorie" || al == "category" || al == "datum" || al == "date":
			continue // section header row
		case strings.HasPrefix(al, "totaal") || strings.HasPrefix(al, "total") || al == "surplus":
			mode = ""
			continue
		case a == "":
			continue
		}

		switch mode {
		case "income":
			if cents := parseCents(col(row, 1)); cents > 0 {
				incomes = append(incomes, IncomeRow{Label: a, AmountCents: cents})
			}
		case "expense":
			if cents := parseCents(col(row, 1)); cents > 0 {
				lines = append(lines, BudgetLineRow{Label: a, AmountCents: cents})
			}
		case "tx":
			date := col(row, 0)
			catLabel := col(row, 1)
			desc := col(row, 2)
			if cents := parseCents(col(row, 3)); cents > 0 && catLabel != "" {
				txs = append(txs, TxRow{CategoryLabel: catLabel, AmountCents: cents, Description: desc, Date: date})
			}
		}
	}
	return incomes, lines, txs
}

// DetectAndParse picks the legacy Fam_Finance parser or this app's own
// export-format parser based on which sheet-naming pattern the workbook
// uses, so callers don't need to know the format in advance.
func DetectAndParse(path string) ([]SheetData, []string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open xlsx: %w", err)
	}
	sheetList := f.GetSheetList()
	f.Close()

	for _, s := range sheetList {
		if sheetRe.MatchString(s) {
			return ParseXLSX(path)
		}
	}
	for _, s := range sheetList {
		if exportSheetRe.MatchString(s) {
			return ParseXLSXExportFormat(path)
		}
	}
	return nil, nil, fmt.Errorf("unrecognized workbook format: no legacy or export sheet names found")
}
