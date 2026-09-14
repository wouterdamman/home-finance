package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/wouterdamman/home-finance/internal/domain"
)

func sumAllocs(allocs []domain.SplitAllocation) int64 {
	var s int64
	for _, a := range allocs {
		s += a.AmountCents
	}
	return s
}

func TestLargestRemainderSplit_SumsExactly(t *testing.T) {
	cases := []struct {
		name    string
		surplus int64
		splits  []domain.PotSplitInput
	}{
		{"67.5/30/2.5", 100000, []domain.PotSplitInput{{1, 67.5}, {2, 30.0}, {3, 2.5}}},
		{"zero surplus", 0, []domain.PotSplitInput{{1, 67.5}, {2, 30.0}, {3, 2.5}}},
		{"negative surplus", -5000, []domain.PotSplitInput{{1, 67.5}, {2, 30.0}, {3, 2.5}}},
		{"single pot 100%", 99999, []domain.PotSplitInput{{1, 100.0}}},
		{"prime surplus", 100003, []domain.PotSplitInput{{1, 50.0}, {2, 50.0}}},
		{"three equal", 100, []domain.PotSplitInput{{1, 33.33}, {2, 33.33}, {3, 33.34}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := domain.LargestRemainderSplit(tc.surplus, tc.splits)
			got := sumAllocs(allocs)
			if got != tc.surplus {
				t.Errorf("sum %d != surplus %d", got, tc.surplus)
			}
		})
	}
}

func TestLargestRemainderSplit_Proportional(t *testing.T) {
	splits := []domain.PotSplitInput{{1, 67.5}, {2, 30.0}, {3, 2.5}}
	allocs := domain.LargestRemainderSplit(100000, splits)
	if allocs[0].AmountCents != 67500 {
		t.Errorf("want 67500 got %d", allocs[0].AmountCents)
	}
	if allocs[1].AmountCents != 30000 {
		t.Errorf("want 30000 got %d", allocs[1].AmountCents)
	}
	if allocs[2].AmountCents != 2500 {
		t.Errorf("want 2500 got %d", allocs[2].AmountCents)
	}
}

func TestLargestRemainderSplit_Empty(t *testing.T) {
	allocs := domain.LargestRemainderSplit(1000, nil)
	if allocs != nil {
		t.Errorf("expected nil, got %v", allocs)
	}
}

// Every call site (handleGetPeriodOverview, handleClosePeriod, importer.mapper)
// maps allocations back onto its own split slice by index, so a reordered or
// short result would silently credit the wrong pot.
func TestLargestRemainderSplit_PositionalCorrespondence(t *testing.T) {
	splits := []domain.PotSplitInput{{7, 33.33}, {3, 33.33}, {9, 33.34}, {5, 0}}
	allocs := domain.LargestRemainderSplit(100, splits)
	if len(allocs) != len(splits) {
		t.Fatalf("len %d != %d", len(allocs), len(splits))
	}
	for i := range splits {
		if allocs[i].PotID != splits[i].PotID {
			t.Errorf("allocs[%d].PotID = %d, want %d", i, allocs[i].PotID, splits[i].PotID)
		}
	}
}

func TestLargestRemainderSplit_NegativeSurplusProportional(t *testing.T) {
	splits := []domain.PotSplitInput{{1, 67.5}, {2, 30.0}, {3, 2.5}}
	allocs := domain.LargestRemainderSplit(-5000, splits)
	want := []int64{-3375, -1500, -125}
	for i, w := range want {
		if allocs[i].AmountCents != w {
			t.Errorf("allocs[%d] = %d, want %d", i, allocs[i].AmountCents, w)
		}
	}
}

// A tie between equal remainders resolves to the earliest pot — pinned so the
// allocation of an odd cent stays reproducible across runs.
func TestLargestRemainderSplit_RemainderTie(t *testing.T) {
	allocs := domain.LargestRemainderSplit(1, []domain.PotSplitInput{{1, 50.0}, {2, 50.0}})
	if allocs[0].AmountCents != 1 || allocs[1].AmountCents != 0 {
		t.Errorf("got %d/%d, want 1/0", allocs[0].AmountCents, allocs[1].AmountCents)
	}
}

func TestLargestRemainderSplit_ZeroPercentPot(t *testing.T) {
	allocs := domain.LargestRemainderSplit(12345, []domain.PotSplitInput{{1, 100.0}, {2, 0}})
	if allocs[0].AmountCents != 12345 || allocs[1].AmountCents != 0 {
		t.Errorf("got %d/%d, want 12345/0", allocs[0].AmountCents, allocs[1].AmountCents)
	}
}

// Pins the deliberate fail-safe behaviour for input the caller's 100%-total
// invariant does not hold for: it terminates, keeps its positions, and leaves a
// visible shortfall instead of dumping the undistributable rest on one pot.
func TestLargestRemainderSplit_MalformedInput(t *testing.T) {
	cases := []struct {
		name    string
		surplus int64
		splits  []domain.PotSplitInput
		want    []int64
	}{
		{"NaN percentage", 1000, []domain.PotSplitInput{{1, math.NaN()}, {2, 50.0}}, []int64{0, 501}},
		{"+Inf percentage", 1000, []domain.PotSplitInput{{1, math.Inf(1)}, {2, 50.0}}, []int64{0, 501}},
		{"-Inf percentage", 1000, []domain.PotSplitInput{{1, math.Inf(-1)}, {2, 50.0}}, []int64{0, 501}},
		{"all non-finite", 1000, []domain.PotSplitInput{{1, math.NaN()}, {2, math.NaN()}}, []int64{0, 0}},
		{"total below 100", 1000, []domain.PotSplitInput{{1, 50.0}, {2, 25.0}}, []int64{501, 251}},
		{"total above 100", 1000, []domain.PotSplitInput{{1, 100.0}, {2, 100.0}}, []int64{999, 999}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan []domain.SplitAllocation, 1)
			go func() { done <- domain.LargestRemainderSplit(tc.surplus, tc.splits) }()
			var allocs []domain.SplitAllocation
			select {
			case allocs = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("did not terminate — redistribution loop is unbounded again")
			}
			if len(allocs) != len(tc.splits) {
				t.Fatalf("len %d != %d", len(allocs), len(tc.splits))
			}
			for i := range tc.splits {
				if allocs[i].PotID != tc.splits[i].PotID {
					t.Errorf("allocs[%d].PotID = %d, want %d", i, allocs[i].PotID, tc.splits[i].PotID)
				}
				if allocs[i].AmountCents != tc.want[i] {
					t.Errorf("allocs[%d] = %d, want %d", i, allocs[i].AmountCents, tc.want[i])
				}
			}
		})
	}
}
