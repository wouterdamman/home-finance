package httpapi

import (
	"math"
	"testing"
)

func TestParseSplitPercentageRejectsNonFinite(t *testing.T) {
	// strconv.ParseFloat happily returns these, and every downstream guard is
	// a comparison — all false for a NaN — so they used to reach the
	// allocation math intact.
	for _, raw := range []string{"NaN", "nan", "+Inf", "-Inf", "inf", "Infinity"} {
		if _, err := parseSplitPercentage(raw); err == nil {
			t.Errorf("parseSplitPercentage(%q): want error, got none", raw)
		}
	}
}

func TestParseSplitPercentageRejectsOutOfRange(t *testing.T) {
	for _, raw := range []string{"-0.01", "-1", "100.01", "1000", "1e9"} {
		if _, err := parseSplitPercentage(raw); err == nil {
			t.Errorf("parseSplitPercentage(%q): want error, got none", raw)
		}
	}
}

func TestParseSplitPercentageRejectsNonNumeric(t *testing.T) {
	for _, raw := range []string{"", " ", "abc", "50%", "1,5"} {
		if _, err := parseSplitPercentage(raw); err == nil {
			t.Errorf("parseSplitPercentage(%q): want error, got none", raw)
		}
	}
}

func TestParseSplitPercentageAcceptsValid(t *testing.T) {
	cases := map[string]float64{
		"0":     0,
		"0.00":  0,
		"33.33": 33.33,
		"100":   100,
		"1e2":   100,
	}
	for raw, want := range cases {
		got, err := parseSplitPercentage(raw)
		if err != nil {
			t.Errorf("parseSplitPercentage(%q): unexpected error %v", raw, err)
			continue
		}
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("parseSplitPercentage(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestNextPeriodOf(t *testing.T) {
	cases := []struct {
		year, month         int
		wantYear, wantMonth int
	}{
		{2026, 1, 2026, 2},
		{2026, 11, 2026, 12},
		{2026, 12, 2027, 1},
	}
	for _, c := range cases {
		gotYear, gotMonth := nextPeriodOf(c.year, c.month)
		if gotYear != c.wantYear || gotMonth != c.wantMonth {
			t.Errorf("nextPeriodOf(%d, %d) = (%d, %d), want (%d, %d)",
				c.year, c.month, gotYear, gotMonth, c.wantYear, c.wantMonth)
		}
	}
}
