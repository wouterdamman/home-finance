package domain

import "math"

// SplitAllocation holds a pot's share of the surplus.
type SplitAllocation struct {
	PotID       int64
	AmountCents int64
}

// PotSplitInput is one pot's percentage share (0-100).
type PotSplitInput struct {
	PotID      int64
	Percentage float64
}

// maxExactCents bounds the intermediate float share to what int64 can hold.
// Go's float->int conversion is undefined past that range and yields the most
// negative int64 on amd64, which is what turns a single corrupt percentage into
// a wrapped leftover.
const maxExactCents = 9.223372036854775e18

// LargestRemainderSplit distributes surplusCents proportionally.
//
// The returned slice is always positionally 1:1 with splits — every caller maps
// allocations back to pots by index — so a percentage this function can't reason
// about yields a zero allocation for that pot rather than being dropped.
//
// For well-formed input (percentages summing to exactly 100, see ValidateSplits)
// the allocations sum exactly to surplusCents. They deliberately do not for a
// broken total: the redistribution is capped at one cent per pot, so a bad total
// leaves a visible shortfall instead of being dumped onto an arbitrary pot.
func LargestRemainderSplit(surplusCents int64, splits []PotSplitInput) []SplitAllocation {
	if len(splits) == 0 {
		return nil
	}
	allocs := make([]SplitAllocation, len(splits))
	remainders := make([]float64, len(splits))
	var assigned int64

	for i, s := range splits {
		allocs[i] = SplitAllocation{PotID: s.PotID}
		exact := float64(surplusCents) * s.Percentage / 100.0
		if math.IsNaN(exact) || exact >= maxExactCents || exact <= -maxExactCents {
			// A NaN/Inf percentage must never reach the arithmetic below: it is
			// rejected at the API boundary, but a row already stored in the DB
			// would otherwise hang a period close inside its open transaction.
			remainders[i] = math.Inf(-1)
			continue
		}
		floor := int64(math.Floor(exact))
		allocs[i].AmountCents = floor
		remainders[i] = exact - float64(floor)
		assigned += floor
	}

	leftover := surplusCents - assigned
	// One pass consumes one pot's remainder, so more passes than there are pots
	// can never improve the distribution — and the cap is what keeps a corrupt
	// percentage from spinning this loop ~9.2e18 times.
	passes := int64(len(splits))
	if n := absInt(leftover); n < passes {
		passes = n
	}
	// leftover is never negative for a true floor with percentages totalling
	// 100 (the sum of floors is an integer that can't exceed the surplus); the
	// sign only matters for a total above 100, which upstream validation rejects.
	step := int64(1)
	if leftover < 0 {
		step = -1
	}
	for i := int64(0); i < passes; i++ {
		best := -1
		var bestRem float64
		for j, r := range remainders {
			if best == -1 || r > bestRem {
				best = j
				bestRem = r
			}
		}
		if best < 0 || math.IsInf(remainders[best], -1) {
			break
		}
		allocs[best].AmountCents += step
		remainders[best] = math.Inf(-1) // consumed
	}
	return allocs
}

func absInt(x int64) int64 {
	if x == math.MinInt64 {
		return math.MaxInt64
	}
	if x < 0 {
		return -x
	}
	return x
}
