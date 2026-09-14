package importer

import (
	"errors"
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

const (
	modeNone     = ""
	modeIncome   = "income"
	modeExpense  = "expense"
	modeTx       = "tx"
	modeIncomeTx = "incomeTx"
)

// sectionHeaders are the column-header rows the export writes directly under a
// section title. They are only skipped in that position, so a category or
// source that happens to be named "Categorie" is still imported as data.
var sectionHeaders = map[string][]string{
	modeIncome:   {"bron", "source"},
	modeExpense:  {"categorie", "category"},
	modeTx:       {"datum", "date"},
	modeIncomeTx: {"datum", "date"},
}

// ParseXLSXExportFormat reads sheets produced by this app's own export
// (one sheet per month: Inkomsten/Uitgaven/Transacties/Inkomsten transacties
// sections) and returns the same SheetData shape ParseXLSX produces, so Run()
// needs no changes to consume either source. Pot splits are not present in the
// export format, so periods parsed this way import without splits — a close on
// such a period allocates nothing to pots (see mapper.go).
func ParseXLSXExportFormat(path string) ([]SheetData, []string, error) {
	f, err := openWorkbook(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return parseExportWorkbook(f)
}

func parseExportWorkbook(f *excelize.File) ([]SheetData, []string, error) {
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
			return nil, nil, fmt.Errorf("read sheet %q: %w", sheet, err)
		}

		c := parseExportSheet(sheet, rows)
		sheets = append(sheets,
			SheetData{Year: year, Month: month, Kind: "Overview", Incomes: c.incomes, Lines: c.lines, Problems: c.problems},
			SheetData{Year: year, Month: month, Kind: "Details", Txs: c.txs, IncomeTxs: c.incomeTxs},
		)
	}
	return sheets, skipped, nil
}

type exportSheetContent struct {
	incomes   []IncomeRow
	lines     []BudgetLineRow
	txs       []TxRow
	incomeTxs []IncomeTxRow
	problems  []string
}

func parseExportSheet(sheet string, rows [][]string) exportSheetContent {
	var c exportSheetContent
	mode := modeNone
	expectHeader := false

	col := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	problem := func(colLetter string, rowNum int, format string, args ...any) {
		c.problems = append(c.problems,
			fmt.Sprintf("%s!%s%d: %s", sheet, colLetter, rowNum, fmt.Sprintf(format, args...)))
	}
	// amount reads one amount cell, reporting anything unreadable. Blank is
	// distinguished from unreadable so a genuinely empty row stays silent.
	amount := func(raw, colLetter string, rowNum int) (int64, bool) {
		cents, err := parseCents(raw)
		if err != nil {
			if errors.Is(err, errEmptyAmount) {
				problem(colLetter, rowNum, "row has no amount, skipped")
			} else {
				problem(colLetter, rowNum, "%v", err)
			}
			return 0, false
		}
		return cents, true
	}

	for i, row := range rows {
		rowNum := i + 1
		a := col(row, 0)
		al := strings.ToLower(a)

		switch al {
		case "inkomsten", "income":
			mode, expectHeader = modeIncome, true
			continue
		case "inkomsten transacties", "income transactions":
			mode, expectHeader = modeIncomeTx, true
			continue
		case "uitgaven", "expenses":
			mode, expectHeader = modeExpense, true
			continue
		case "transacties", "transactions":
			mode, expectHeader = modeTx, true
			continue
		// Anchored on the exact strings the export writes: a prefix match ends
		// the section on a real category named "Totaal onderhoud" too, silently
		// discarding every row after it.
		case "totaal inkomsten", "total income",
			"totaal uitgaven", "total expenses",
			"surplus":
			mode, expectHeader = modeNone, false
			continue
		}

		if expectHeader {
			expectHeader = false
			if isSectionHeader(mode, al) {
				continue
			}
		}

		switch mode {
		case modeIncome:
			amt := col(row, 1)
			if a == "" && amt == "" {
				continue
			}
			// close() writes its own carryover entry; re-importing the exported
			// one double-counts it and invents an income source named after it.
			if isCarryoverLabel(a) {
				continue
			}
			cents, ok := amount(amt, "B", rowNum)
			if !ok {
				continue
			}
			if a == "" {
				problem("A", rowNum, "income row has no source name, skipped")
				continue
			}
			c.incomes = append(c.incomes, IncomeRow{Label: unescapeExportCell(a), AmountCents: cents})

		case modeExpense:
			amt := col(row, 1)
			if a == "" && amt == "" {
				continue
			}
			cents, ok := amount(amt, "B", rowNum)
			if !ok {
				continue
			}
			if a == "" {
				problem("A", rowNum, "expense row has no category name, skipped")
				continue
			}
			c.lines = append(c.lines, BudgetLineRow{Label: unescapeExportCell(a), AmountCents: cents})

		case modeTx:
			// The Datum column is nullable and the export writes it empty for a
			// dateless transaction, so the row's identity comes from the
			// category/amount columns, never from column A being non-empty.
			catLabel, desc, amt := col(row, 1), col(row, 2), col(row, 3)
			if catLabel == "" && amt == "" {
				continue
			}
			cents, ok := amount(amt, "D", rowNum)
			if !ok {
				continue
			}
			if catLabel == "" {
				problem("B", rowNum, "transaction row has no category, skipped")
				continue
			}
			c.txs = append(c.txs, TxRow{CategoryLabel: unescapeExportCell(catLabel), AmountCents: cents, Description: unescapeExportCell(desc), Date: a})

		case modeIncomeTx:
			srcLabel, desc, amt := col(row, 1), col(row, 2), col(row, 3)
			if srcLabel == "" && amt == "" {
				continue
			}
			cents, ok := amount(amt, "D", rowNum)
			if !ok {
				continue
			}
			if srcLabel == "" {
				problem("B", rowNum, "income transaction row has no source, skipped")
				continue
			}
			c.incomeTxs = append(c.incomeTxs, IncomeTxRow{SourceLabel: unescapeExportCell(srcLabel), AmountCents: cents, Description: unescapeExportCell(desc), Date: a})
		}
	}
	return c
}

// unescapeExportCell undoes the apostrophe the export prefixes to any text that
// a spreadsheet would otherwise read back as a formula (sanitizeExportCell in
// export_handlers.go), so a category literally named "-Onderhoud" round-trips
// to itself instead of creating a second category named "'-Onderhoud".
func unescapeExportCell(s string) string {
	if len(s) >= 2 && s[0] == '\'' {
		switch s[1] {
		case '=', '+', '-', '@', '\t', '\r':
			return s[1:]
		}
	}
	return s
}

func isSectionHeader(mode, labelLower string) bool {
	for _, h := range sectionHeaders[mode] {
		if labelLower == h {
			return true
		}
	}
	return false
}

// DetectAndParse picks the legacy Fam_Finance parser or this app's own
// export-format parser based on which sheet-naming pattern the workbook
// uses, so callers don't need to know the format in advance.
func DetectAndParse(path string) ([]SheetData, []string, error) {
	f, err := openWorkbook(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	sheetList := f.GetSheetList()
	for _, s := range sheetList {
		if sheetRe.MatchString(s) {
			return parseLegacyWorkbook(f)
		}
	}
	for _, s := range sheetList {
		if exportSheetRe.MatchString(s) {
			return parseExportWorkbook(f)
		}
	}
	return nil, nil, errors.New("unrecognized workbook format: no legacy or export sheet names found")
}
