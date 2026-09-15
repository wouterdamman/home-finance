package importer

import (
	"errors"
	"testing"
)

func TestParseCentsLocaleAndCurrencyFormats(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		// Raw values as the export writes them.
		{"1000", 100000},
		{"1234.56", 123456},
		{"0", 0},
		{"-50.25", -5025},
		{"2.5e-02", 3},
		// Re-saved through Excel with a locale/currency number format, which is
		// what GetRows hands back once a user opens and saves the export.
		{"1,234.56", 123456},
		{"1.234,56", 123456},
		{"€ 1.234,56", 123456},
		{"€1,234.56", 123456},
		{"1 234,56", 123456},
		{"1 234,56", 123456},
		{"-€ 1.234,56", -123456},
		{"€ -1.234,56", -123456},
		{"(1.234,56)", -123456},
		{"123.456,78", 12345678},
		{"10,00", 1000},
		{"10.00", 1000},
	}
	for _, tc := range cases {
		got, err := parseCents(tc.in)
		if err != nil {
			t.Errorf("parseCents(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseCents(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseCentsRejectsOutOfRangeAndGarbage(t *testing.T) {
	// int64(math.Round(1e300*100)) silently saturates; the month then fails
	// every aggregate with "bigint out of range" and can't be loaded at all.
	rangeCases := []string{"1e300", "-1e300", "1000000.01", "1e400", "1.234.567,89"}
	for _, in := range rangeCases {
		got, err := parseCents(in)
		if !errors.Is(err, errAmountRange) {
			t.Errorf("parseCents(%q) = (%d, %v), want errAmountRange", in, got, err)
		}
	}

	if _, err := parseCents(""); !errors.Is(err, errEmptyAmount) {
		t.Errorf("parseCents(\"\") should report errEmptyAmount, got %v", err)
	}
	if _, err := parseCents("   "); !errors.Is(err, errEmptyAmount) {
		t.Errorf("parseCents(spaces) should report errEmptyAmount, got %v", err)
	}
	for _, in := range []string{"n.v.t.", "Totaal", "€", "NaN", "Inf"} {
		if _, err := parseCents(in); err == nil {
			t.Errorf("parseCents(%q) should fail rather than silently yield 0", in)
		}
	}
}

func TestParseCentsAcceptsTheBoundary(t *testing.T) {
	got, err := parseCents("1000000.00")
	if err != nil || got != maxImportAmountCents {
		t.Fatalf("parseCents(1000000.00) = (%d, %v), want (%d, nil)", got, err, maxImportAmountCents)
	}
}
