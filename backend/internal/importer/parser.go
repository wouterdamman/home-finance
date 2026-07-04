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

type MonthData struct {
	Year    int
	Month   int
	Sheet   string // "Overview" or "Details"
	Incomes []IncomeRow
	Lines   []BudgetLineRow
	Txs     []TxRow
}

type IncomeRow struct {
	Label       string
	AmountCents int64
}

type BudgetLineRow struct {
	Label              string
	AmountCents        int64
	TracksTransactions bool
}

type TxRow struct {
	CategoryLabel string
	AmountCents   int64
	Description   string
}

func ParseXLSX(path string) ([]MonthData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var months []MonthData
	for _, sheet := range f.GetSheetList() {
		m := sheetRe.FindStringSubmatch(sheet)
		if m == nil {
			continue
		}
		monthNum, _ := strconv.Atoi(m[1])
		year, _ := strconv.Atoi(m[2])
		kind := m[3]

		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("get rows %s: %w", sheet, err)
		}

		md := MonthData{Year: year, Month: monthNum, Sheet: kind}
		switch kind {
		case "Overview":
			md.Incomes, md.Lines = parseOverview(rows)
		case "Details":
			md.Txs = parseDetails(rows)
		}
		months = append(months, md)
	}
	return months, nil
}

func parseOverview(rows [][]string) ([]IncomeRow, []BudgetLineRow) {
	var incomes []IncomeRow
	var lines []BudgetLineRow
	inIncome := false
	inExpense := false

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		label := strings.TrimSpace(row[0])
		if label == "" {
			continue
		}
		labelLower := strings.ToLower(label)

		if strings.Contains(labelLower, "inkomsten") || strings.Contains(labelLower, "inkomen") {
			inIncome = true
			inExpense = false
			continue
		}
		if strings.Contains(labelLower, "uitgaven") || strings.Contains(labelLower, "kosten") || strings.Contains(labelLower, "vaste") {
			inIncome = false
			inExpense = true
			continue
		}
		if strings.Contains(labelLower, "totaal") {
			inIncome = false
			inExpense = false
			continue
		}

		val := extractAmount(row)
		if val == 0 {
			continue
		}

		if inIncome {
			incomes = append(incomes, IncomeRow{Label: label, AmountCents: val})
		} else if inExpense {
			lines = append(lines, BudgetLineRow{Label: label, AmountCents: val})
		}
	}
	return incomes, lines
}

func parseDetails(rows [][]string) []TxRow {
	var txs []TxRow
	var currentCategory string

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		label := strings.TrimSpace(row[0])
		if label == "" {
			continue
		}
		labelLower := strings.ToLower(label)

		if strings.Contains(labelLower, "totaal") {
			continue
		}

		val := extractAmount(row)
		desc := ""
		if len(row) > 1 {
			desc = strings.TrimSpace(row[1])
		}

		if val == 0 && desc == "" {
			currentCategory = label
			continue
		}

		if val != 0 {
			cat := currentCategory
			if cat == "" {
				cat = label
			}
			d := desc
			if d == "" {
				d = label
			}
			txs = append(txs, TxRow{CategoryLabel: cat, AmountCents: val, Description: d})
		}
	}
	return txs
}

func extractAmount(row []string) int64 {
	for i := len(row) - 1; i >= 0; i-- {
		cell := strings.TrimSpace(row[i])
		if cell == "" {
			continue
		}
		cell = strings.ReplaceAll(cell, "€", "")
		cell = strings.TrimSpace(cell)
		cell = strings.ReplaceAll(cell, ".", "")
		cell = strings.ReplaceAll(cell, ",", ".")
		f, err := strconv.ParseFloat(cell, 64)
		if err == nil && f != 0 {
			return int64(math.Round(f * 100))
		}
	}
	return 0
}
