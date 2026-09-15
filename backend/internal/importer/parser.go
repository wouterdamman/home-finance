package importer

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

var sheetRe = regexp.MustCompile(`(?i)^(\d+)[-\s]+(\d{2})[-\s]+(overview|details)$`)

var sheetKindNormalize = map[string]string{"overview": "Overview", "details": "Details"}

// openOpts bounds what excelize inflates from an uploaded workbook. Its
// default UnzipSizeLimit is ~16GB and readFile pre-allocates an entry's
// *declared* uncompressed size, so a 15MB upload declaring a 15GB entry would
// OOM the process long before the 20MB upload cap ever mattered.
var openOpts = excelize.Options{UnzipSizeLimit: 100 << 20, UnzipXMLSizeLimit: 16 << 20}

func openWorkbook(path string) (*excelize.File, error) {
	f, err := excelize.OpenFile(path, openOpts)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	return f, nil
}

type SheetData struct {
	Year      int
	Month     int
	Kind      string // "Overview" or "Details"
	Incomes   []IncomeRow
	Lines     []BudgetLineRow
	Splits    []SplitRow
	Txs       []TxRow
	IncomeTxs []IncomeTxRow
	// Problems lists cells this sheet carried that could not be imported
	// (unreadable or out-of-range amounts). They are surfaced in the import
	// Report instead of being silently turned into a zero or dropped.
	Problems []string
}

type IncomeRow struct {
	Label       string
	AmountCents int64
}

type BudgetLineRow struct {
	Label       string
	AmountCents int64
}

type SplitRow struct {
	PotName    string
	Percentage float64 // 0–100
}

type TxRow struct {
	CategoryLabel string
	AmountCents   int64
	Description   string
	// Date is the transaction date in YYYY-MM-DD form. Only the export-format
	// parser populates it — the legacy Fam_Finance layout never carried a
	// per-transaction date, so mapper.Run() falls back to day 1 of the month
	// when it's empty.
	Date string
}

// IncomeTxRow is one line item of an itemized income source, the income-side
// mirror of TxRow. Only this app's own export carries them.
type IncomeTxRow struct {
	SourceLabel string
	AmountCents int64
	Description string
	Date        string
}

func ParseXLSX(path string) ([]SheetData, []string, error) {
	f, err := openWorkbook(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return parseLegacyWorkbook(f)
}

func parseLegacyWorkbook(f *excelize.File) ([]SheetData, []string, error) {
	var sheets []SheetData
	var skipped []string
	for _, sheet := range f.GetSheetList() {
		m := sheetRe.FindStringSubmatch(sheet)
		if m == nil {
			skipped = append(skipped, sheet)
			continue
		}
		monthNum, _ := strconv.Atoi(m[1])
		year, _ := strconv.Atoi(m[2])
		if year < 100 {
			year += 2000
		}
		kind := sheetKindNormalize[strings.ToLower(m[3])]

		sd := SheetData{Year: year, Month: monthNum, Kind: kind}
		switch kind {
		case "Overview":
			sd.Incomes, sd.Lines, sd.Splits, sd.Problems = parseOverview(f, sheet)
		case "Details":
			sd.Txs, sd.Problems = parseDetails(f, sheet)
		}
		sheets = append(sheets, sd)
	}
	return sheets, skipped, nil
}

func cell(f *excelize.File, sheet, col string, row int) string {
	v, _ := f.GetCellValue(sheet, fmt.Sprintf("%s%d", col, row), excelize.Options{RawCellValue: true})
	return strings.TrimSpace(v)
}

// isCarryoverLabel reports whether an income label is one of the
// "Doorlopen maand <maand>" entries that closing a period generates. They must
// never be re-imported as ordinary income: close writes its own copy, so an
// imported one double-counts the carryover and invents an income source for it.
func isCarryoverLabel(label string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(label)), "doorlopen maand")
}

func parseOverview(f *excelize.File, sheet string) ([]IncomeRow, []BudgetLineRow, []SplitRow, []string) {
	var incomes []IncomeRow
	var lines []BudgetLineRow
	var splits []SplitRow
	var problems []string

	// A legacy sheet is a fixed-column scan over rows that are mostly not data,
	// so an unreadable cell is normal and stays silent; only a value the app
	// cannot store is worth reporting.
	report := func(col string, row int, err error) {
		if errors.Is(err, errAmountRange) {
			problems = append(problems, fmt.Sprintf("%s!%s%d: %v", sheet, col, row, err))
		}
	}

	incomeDone := false
	for row := 2; row <= 60; row++ {
		a := cell(f, sheet, "A", row)
		b := cell(f, sheet, "B", row)
		c := cell(f, sheet, "C", row)
		d := cell(f, sheet, "D", row)
		h := cell(f, sheet, "H", row)
		ii := cell(f, sheet, "I", row)

		// ── Income: col A = label, col B = value ──────────────────
		if !incomeDone {
			if strings.Contains(strings.ToLower(a), "totale inkomsten") {
				incomeDone = true
			} else if a != "" && b != "" {
				label := strings.TrimSpace(a)
				if !isCarryoverLabel(label) {
					cents, err := parseCents(b)
					switch {
					case err != nil:
						report("B", row, err)
					case cents > 0:
						incomes = append(incomes, IncomeRow{Label: label, AmountCents: cents})
					}
				}
			}
		}

		// ── Budget lines: col C = value, col D = label ────────────
		if c != "" && d != "" {
			dl := strings.ToLower(d)
			if !strings.Contains(dl, "totaal af") && !strings.Contains(dl, "totaal over") {
				cents, err := parseCents(c)
				switch {
				case err != nil:
					report("C", row, err)
				case cents > 0:
					lines = append(lines, BudgetLineRow{Label: strings.TrimSpace(d), AmountCents: cents})
				}
			}
		}

		// ── Pot splits: col H = fraction (0-1), col I = pot name ──
		if h != "" && ii != "" {
			name := strings.TrimSpace(ii)
			if !strings.EqualFold(name, "totaal") {
				pct := parseFloat(h)
				switch {
				case math.IsNaN(pct) || math.IsInf(pct, 0):
					problems = append(problems, fmt.Sprintf("%s!H%d: pot split percentage %q is not a finite number", sheet, row, h))
				case pct > 0:
					splits = append(splits, SplitRow{
						PotName:    normalizePotName(name),
						Percentage: pct * 100,
					})
				}
			}
		}
	}
	return incomes, lines, splits, problems
}

func parseDetails(f *excelize.File, sheet string) ([]TxRow, []string) {
	type catDef struct {
		name    string
		valCol  string
		descCol string
	}

	// Row 1: category headers at odd column positions (A, C, E, G, ...)
	var cats []catDef
	for colNum := 1; colNum <= 30; colNum += 2 {
		colName, err := excelize.ColumnNumberToName(colNum)
		if err != nil {
			break
		}
		nextColName, _ := excelize.ColumnNumberToName(colNum + 1)
		header := cell(f, sheet, colName, 1)
		if header == "" {
			break
		}
		cats = append(cats, catDef{
			name:    strings.TrimSpace(header),
			valCol:  colName,
			descCol: nextColName,
		})
	}

	var txs []TxRow
	var problems []string
	for _, cat := range cats {
		for row := 2; row <= 300; row++ {
			valStr := cell(f, sheet, cat.valCol, row)
			descStr := cell(f, sheet, cat.descCol, row)
			if strings.EqualFold(strings.TrimSpace(descStr), "totaal") {
				break
			}
			if valStr == "" {
				continue
			}
			cents, err := parseCents(valStr)
			if err != nil {
				if errors.Is(err, errAmountRange) {
					problems = append(problems, fmt.Sprintf("%s!%s%d: %v", sheet, cat.valCol, row, err))
				}
				continue
			}
			if cents > 0 {
				desc := strings.TrimSpace(descStr)
				if desc == "" {
					desc = cat.name
				}
				txs = append(txs, TxRow{
					CategoryLabel: cat.name,
					AmountCents:   cents,
					Description:   desc,
				})
			}
		}
	}
	return txs, problems
}

func normalizePotName(name string) string {
	// "Doorlopen (speling 2.5%)" → "Doorlopen"
	// "Trouwen * -720" → "Trouwen"
	for _, sep := range []string{" (", " *", " -"} {
		if idx := strings.Index(name, sep); idx > 0 {
			name = name[:idx]
		}
	}
	return strings.TrimSpace(name)
}

// maxImportAmountCents mirrors httpapi.maxAmountCents (1,000,000.00 in the
// app's currency). The importer cannot reference that constant directly
// (httpapi imports this package), but a cell must obey the same bound as an
// amount typed into the UI: "1e300" otherwise saturates int64, and every
// aggregate over that month then fails with "bigint out of range", leaving it
// permanently unloadable with no UI path to delete the row.
const maxImportAmountCents = 100_000_000

var (
	// errEmptyAmount means the cell was blank — usually "not a data row",
	// which callers distinguish from a cell that held something unreadable.
	errEmptyAmount = errors.New("empty amount cell")
	// errAmountRange means the cell held a number the app cannot store.
	errAmountRange = errors.New("amount out of range")
)

// amountRe finds the numeric core of a cell, skipping any currency prefix
// ("€ 1.234,56") or suffix ("1 234,56 EUR").
var amountRe = regexp.MustCompile(`[0-9][0-9.,]*(?:[eE][-+]?[0-9]+)?`)

// parseCents converts a spreadsheet cell to integer cents. GetRows returns
// *formatted* values, so the same amount reaches us as "1234.56", "1.234,56"
// or "€ 1 234,56" depending on what the user's Excel locale wrote: whichever
// of "." or "," comes last is the decimal separator, every other one is a
// thousands separator. Returns an error rather than 0 for anything it cannot
// read, so a bad cell is reported instead of silently importing as zero — and
// bounds the result, so an absurd value can't poison the month's aggregates.
func parseCents(s string) (int64, error) {
	t := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	if t == "" {
		return 0, errEmptyAmount
	}

	loc := amountRe.FindStringIndex(t)
	if loc == nil {
		return 0, fmt.Errorf("could not read %q as an amount", s)
	}
	num := t[loc[0]:loc[1]]
	// A minus can sit before the currency symbol ("-€ 5,00") as easily as
	// after it, and accountants' parentheses mean the same thing.
	neg := strings.ContainsAny(t[:loc[0]], "-−") ||
		(strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")"))

	mantissa, exp := num, ""
	if i := strings.IndexAny(num, "eE"); i >= 0 {
		mantissa, exp = num[:i], num[i:]
	}
	intPart, frac := mantissa, ""
	if i := strings.LastIndexAny(mantissa, ".,"); i >= 0 {
		intPart, frac = mantissa[:i], mantissa[i+1:]
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	if intPart == "" {
		intPart = "0"
	}
	normalized := intPart
	if frac != "" {
		normalized += "." + frac
	}

	f, err := strconv.ParseFloat(normalized+exp, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%w: %q", errAmountRange, s)
	}
	cents := math.Round(f * 100)
	if math.Abs(cents) > maxImportAmountCents {
		return 0, fmt.Errorf("%w: %q", errAmountRange, s)
	}
	if neg {
		cents = -cents
	}
	return int64(cents), nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
