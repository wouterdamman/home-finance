package main

// largestRemainderInts distributes `total` units across len(weights) buckets
// proportionally to weights, using the largest-remainder method so the
// buckets always sum to exactly `total` (used both for pot_splits
// percentages, in hundredths of a percent, and for pot_ledger allocation
// cents).
func largestRemainderInts(total int64, weights []float64) []int64 {
	n := len(weights)
	result := make([]int64, n)
	if n == 0 || total == 0 {
		return result
	}
	var sumWeights float64
	for _, w := range weights {
		sumWeights += w
	}
	if sumWeights <= 0 {
		// Degenerate: split evenly.
		for i := range weights {
			weights[i] = 1
		}
		sumWeights = float64(n)
	}

	type remainder struct {
		idx int
		rem float64
	}
	remainders := make([]remainder, n)
	var assigned int64
	for i, w := range weights {
		exact := float64(total) * w / sumWeights
		floor := int64(exact)
		result[i] = floor
		remainders[i] = remainder{idx: i, rem: exact - float64(floor)}
		assigned += floor
	}
	leftover := total - assigned
	// Simple selection sort for the `leftover` largest remainders — n is
	// always tiny here (pot count), so O(n^2) is fine and keeps this
	// dependency-free and deterministic without needing a stable sort tie
	// rule beyond "lowest index first" for equal remainders.
	for leftover > 0 {
		best := -1
		for i, r := range remainders {
			if r.rem < 0 {
				continue
			}
			if best == -1 || r.rem > remainders[best].rem {
				best = i
			}
		}
		if best == -1 {
			break
		}
		result[remainders[best].idx]++
		remainders[best].rem = -1
		leftover--
	}
	return result
}
