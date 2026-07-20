package importer

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSheetRegexAcceptsFormatVariants(t *testing.T) {
	matches := []string{
		"7-26-Overview",
		"7-26-Details",
		"7-26 details",
		"7-26-details",
		"7 26 Overview",
		"07-26-Overview",
	}
	for _, name := range matches {
		if !sheetRe.MatchString(name) {
			t.Errorf("expected %q to match sheetRe", name)
		}
	}

	nonMatches := []string{"Summary", "7-26", "2026-07"}
	for _, name := range nonMatches {
		if sheetRe.MatchString(name) {
			t.Errorf("expected %q not to match sheetRe", name)
		}
	}
}

func TestParseXLSXReportsSkippedSheets(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "7-26-Overview")
	f.SetCellValue("7-26-Overview", "A2", "Salaris")
	f.SetCellValue("7-26-Overview", "B2", "1000")

	f.NewSheet("7-26 details")
	f.SetCellValue("7-26 details", "A1", "Boodschappen")
	f.SetCellValue("7-26 details", "A2", "50")
	f.SetCellValue("7-26 details", "B2", "Albert Heijn")

	f.NewSheet("Notes")
	f.SetCellValue("Notes", "A1", "just some unrelated sheet")

	path := t.TempDir() + "/test.xlsx"
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}

	sheets, skipped, err := ParseXLSX(path)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}
	if len(sheets) != 2 {
		t.Fatalf("expected 2 parsed sheets, got %d", len(sheets))
	}
	if len(skipped) != 1 || skipped[0] != "Notes" {
		t.Fatalf("expected skipped=[Notes], got %v", skipped)
	}

	var sawDetails bool
	for _, sd := range sheets {
		if sd.Kind == "Details" {
			sawDetails = true
			if len(sd.Txs) != 1 || sd.Txs[0].CategoryLabel != "Boodschappen" {
				t.Errorf("expected 1 tx under Boodschappen, got %+v", sd.Txs)
			}
		}
	}
	if !sawDetails {
		t.Error("expected a Details sheet to be parsed from '7-26 details'")
	}
}
