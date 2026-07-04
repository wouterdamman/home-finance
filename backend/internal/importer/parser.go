package importer

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

var sheetRe = regexp.MustCompile(`^(\d+)-(\d{2})-(Overview|Details)$`)

type SheetData struct {
	Year    int
	Month   int
	Kind    string // "Overview" or "Details"
	Incomes []IncomeRow
	Lines   []BudgetLineRow
	Splits  []SplitRow
	Txs     []TxRow
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
}

func ParseXLSX(path string) ([]SheetData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var sheets []SheetData
	for _, sheet := range f.GetSheetList() {
		m := sheetRe.FindStringSubmatch(sheet)
		if m == nil {
			continue
		}
		monthNum, _ := strconv.Atoi(m[1])
		year, _ := strconv.Atoi(m[2])
		if year < 100 {
			year += 2000
		}
		kind := m[3]

		sd := SheetData{Year: year, Month: monthNum, Kind: kind}
		switch kind {
		case "Overview":
			sd.Incomes, sd.Lines, sd.Splits = parseOverview(f, sheet)
		case "Details":
			sd.Txs = parseDetails(f, sheet)
		}
		sheets = append(sheets, sd)
	}
	return sheets, nil
}

func cell(f *excelize.File, sheet, col string, row int) string {
	v, _ := f.GetCellValue(sheet, fmt.Sprintf("%s%d", col, row), excelize.Options{RawCellValue: true})
	return strings.TrimSpace(v)
}

func parseOverview(f *excelize.File, sheet string) ([]IncomeRow, []BudgetLineRow, []SplitRow) {
	var incomes []IncomeRow
	var lines []BudgetLineRow
	var splits []SplitRow

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
				if !strings.HasPrefix(strings.ToLower(label), "doorlopen maand") {
					if cents := parseCents(b); cents > 0 {
						incomes = append(incomes, IncomeRow{Label: label, AmountCents: cents})
					}
				}
			}
		}

		// ── Budget lines: col C = value, col D = label ────────────
		if c != "" && d != "" {
			dl := strings.ToLower(d)
			if !strings.Contains(dl, "totaal af") && !strings.Contains(dl, "totaal over") {
				if cents := parseCents(c); cents > 0 {
					lines = append(lines, BudgetLineRow{Label: strings.TrimSpace(d), AmountCents: cents})
				}
			}
		}

		// ── Pot splits: col H = fraction (0-1), col I = pot name ──
		if h != "" && ii != "" {
			name := strings.TrimSpace(ii)
			if !strings.EqualFold(name, "totaal") {
				pct := parseFloat(h)
				if pct > 0 {
					splits = append(splits, SplitRow{
						PotName:    normalizePotName(name),
						Percentage: pct * 100,
					})
				}
			}
		}
	}
	return incomes, lines, splits
}

func parseDetails(f *excelize.File, sheet string) []TxRow {
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
			cents := parseCents(valStr)
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
	return txs
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

func parseCents(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	// Handle scientific notation like "2.5e-02"
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Try comma as decimal
		s2 := strings.ReplaceAll(s, ".", "")
		s2 = strings.ReplaceAll(s2, ",", ".")
		f, err = strconv.ParseFloat(s2, 64)
		if err != nil {
			return 0
		}
	}
	return int64(math.Round(f * 100))
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
