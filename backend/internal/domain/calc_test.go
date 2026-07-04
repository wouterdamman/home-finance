package domain_test

import (
	"testing"

	"github.com/TheIronRock95/home-finance/internal/domain"
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
