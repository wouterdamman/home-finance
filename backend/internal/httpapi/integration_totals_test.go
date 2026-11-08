package httpapi

import "testing"

func TestParsePeriodPath(t *testing.T) {
	for in, want := range map[string][2]int{"2026-11": {2026, 11}, "2026-01": {2026, 1}, "2000-12": {2000, 12}} {
		y, m, ok := parsePeriodPath(in)
		if !ok || y != want[0] || m != want[1] {
			t.Errorf("%q: got %d %d %v", in, y, m, ok)
		}
	}
	for _, in := range []string{"", "2026-13", "2026-00", "2026-1", "202-11", "2026/11", "2026-111", "abc", " 2026-11", "2026-11\n", "current", "２０２６-11"} {
		if _, _, ok := parsePeriodPath(in); ok {
			t.Errorf("%q should be rejected", in)
		}
	}
}

func TestParseYearPath(t *testing.T) {
	for in, want := range map[string]int{"2026": 2026, "2000": 2000, "2100": 2100} {
		if y, ok := parseYearPath(in); !ok || y != want {
			t.Errorf("%q: got %d %v", in, y, ok)
		}
	}
	for _, in := range []string{"", "1999", "2101", "202", "20260", "abcd", "2026-11", "-202", "current", "2026\n"} {
		if _, ok := parseYearPath(in); ok {
			t.Errorf("%q should be rejected", in)
		}
	}
}

func TestPickCurrentPeriod(t *testing.T) {
	p := func(y, m int, st string) periodRef { return periodRef{y, m, st} }
	for name, c := range map[string]struct {
		in   []periodRef
		want periodRef
		ok   bool
	}{
		"none":                  {nil, periodRef{}, false},
		"current month wins":    {[]periodRef{p(2026, 12, "open"), p(2026, 11, "closed"), p(2026, 10, "open")}, p(2026, 11, "closed"), true},
		"latest open fallback":  {[]periodRef{p(2026, 8, "closed"), p(2026, 9, "open"), p(2026, 10, "closed"), p(2025, 12, "open")}, p(2026, 9, "open"), true},
		"latest overall":        {[]periodRef{p(2026, 8, "closed"), p(2026, 10, "closed"), p(2025, 12, "closed")}, p(2026, 10, "closed"), true},
		"order independent":     {[]periodRef{p(2025, 12, "open"), p(2026, 9, "open")}, p(2026, 9, "open"), true},
		"future open preferred": {[]periodRef{p(2027, 1, "open"), p(2026, 10, "open")}, p(2027, 1, "open"), true},
		"same month other year": {[]periodRef{p(2025, 11, "open"), p(2026, 10, "closed")}, p(2025, 11, "open"), true},
	} {
		got, ok := pickCurrentPeriod(c.in, 2026, 11)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got %+v %v, want %+v %v", name, got, ok, c.want, c.ok)
		}
	}
}

func TestAverageMonthlySurplus(t *testing.T) {
	for name, c := range map[string]struct {
		surplus int64
		n       int
		want    int64
	}{
		"no periods":         {1000, 0, 0},
		"exact":              {1200, 12, 100},
		"round down":         {1204, 12, 100},
		"half up":            {1206, 12, 101},
		"half away positive": {5, 2, 3},
		"half away negative": {-5, 2, -3},
		"negative down":      {-1204, 12, -100},
		"negative exact":     {-1200, 12, -100},
		"zero":               {0, 5, 0},
		"one period":         {-7, 1, -7},
		"third":              {100, 3, 33},
		"two thirds":         {200, 3, 67},
	} {
		if got := averageMonthlySurplus(c.surplus, c.n); got != c.want {
			t.Errorf("%s: got %d, want %d", name, got, c.want)
		}
	}
}
