package domain

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

// LargestRemainderSplit distributes surplusCents proportionally.
// The returned allocations sum exactly to surplusCents.
func LargestRemainderSplit(surplusCents int64, splits []PotSplitInput) []SplitAllocation {
	if len(splits) == 0 {
		return nil
	}
	allocs := make([]SplitAllocation, len(splits))
	remainders := make([]float64, len(splits))
	var assigned int64

	for i, s := range splits {
		exact := float64(surplusCents) * s.Percentage / 100.0
		var floor int64
		if exact >= 0 {
			floor = int64(exact)
		} else {
			floor = int64(exact)
			if float64(floor) > exact {
				floor--
			}
		}
		allocs[i] = SplitAllocation{PotID: s.PotID, AmountCents: floor}
		remainders[i] = exact - float64(floor)
		assigned += floor
	}

	leftover := surplusCents - assigned
	for step := int64(0); step < absInt(leftover); step++ {
		best := -1
		var bestRem float64
		for i, r := range remainders {
			if best == -1 || r > bestRem {
				best = i
				bestRem = r
			}
		}
		if best >= 0 {
			if leftover > 0 {
				allocs[best].AmountCents++
			} else {
				allocs[best].AmountCents--
			}
			remainders[best] = -1e18 // consumed
		}
	}
	return allocs
}

func absInt(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
