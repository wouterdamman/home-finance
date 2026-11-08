package httpapi

import (
	"encoding/json"
	"testing"
	"time"
)

func ptrEq[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestPotTargetInfo(t *testing.T) {
	ams := amsterdam()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, ams)
	date := func(y int, m time.Month, d int) *time.Time {
		v := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		return &v
	}
	i64 := func(v int64) *int64 { return &v }
	i := func(v int) *int { return &v }
	f := func(v float64) *float64 { return &v }
	ds := func(v string) *string { return &v }

	for name, c := range map[string]struct {
		balance, target int64
		date            *time.Time
		want            potTarget
	}{
		"in progress with date, ceil": {
			245000, 300000, date(2027, time.June, 1),
			potTarget{300000, ds("2027-06-01"), 55000, f(81.7), false, i(7), i64(7858)}, // 55000/7 = 7857.14 -> 7858
		},
		"exact division": {
			1000, 4000, date(2027, time.February, 15),
			potTarget{4000, ds("2027-02-15"), 3000, f(25), false, i(3), i64(1000)},
		},
		"reached": {
			6000, 5000, date(2027, time.January, 1),
			potTarget{5000, ds("2027-01-01"), 0, f(120), true, i(2), i64(0)},
		},
		"exactly reached": {
			5000, 5000, nil,
			potTarget{5000, nil, 0, f(100), true, nil, nil},
		},
		"no date": {
			1000, 3000, nil,
			potTarget{3000, nil, 2000, f(33.3), false, nil, nil},
		},
		"date in the past": {
			2500, 10000, date(2026, time.March, 1),
			potTarget{10000, ds("2026-03-01"), 7500, f(25), false, i(0), i64(7500)},
		},
		"target month is this month": {
			100, 1000, date(2026, time.November, 30),
			potTarget{1000, ds("2026-11-30"), 900, f(10), false, i(0), i64(900)},
		},
		"next month": {
			0, 1001, date(2026, time.December, 1),
			potTarget{1001, ds("2026-12-01"), 1001, f(0), false, i(1), i64(1001)},
		},
		"across a year boundary": {
			0, 1000, date(2027, time.November, 1),
			potTarget{1000, ds("2027-11-01"), 1000, f(0), false, i(12), i64(84)}, // ceil(1000/12)
		},
		"negative balance": {
			-500, 1000, nil,
			potTarget{1000, nil, 1500, f(-50), false, nil, nil},
		},
		"zero target has no ratio": {
			500, 0, nil,
			potTarget{0, nil, 0, nil, true, nil, nil},
		},
	} {
		got := potTargetInfo(c.balance, c.target, c.date, now)
		w := c.want
		if got.TargetCents != w.TargetCents || !ptrEq(got.TargetDate, w.TargetDate) || got.RemainingCents != w.RemainingCents ||
			!ptrEq(got.ProgressPercent, w.ProgressPercent) || got.Reached != w.Reached ||
			!ptrEq(got.MonthsLeft, w.MonthsLeft) || !ptrEq(got.NeededPerMonthCents, w.NeededPerMonthCents) {
			t.Errorf("%s:\n got %s\nwant %s", name, fmtTarget(got), fmtTarget(w))
		}
	}
}

func fmtTarget(p potTarget) string {
	b, _ := json.Marshal(p)
	return string(b)
}
