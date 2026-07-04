package domain

import (
	"errors"
	"fmt"
)

var (
	ErrPeriodClosed          = errors.New("period_closed")
	ErrSplitPercentageNot100 = errors.New("split_percentage_not_100")
	ErrPeriodAlreadyClosed   = errors.New("period_already_closed")
	ErrNextPeriodClosed      = errors.New("next_period_closed")
)

// ComputeSurplus returns income minus expenses.
func ComputeSurplus(incomeCents, expenseCents int64) int64 {
	return incomeCents - expenseCents
}

// ValidateSplits checks that percentages sum to 100.00 (±0.01 epsilon).
func ValidateSplits(splits []PotSplitInput) error {
	var total float64
	for _, s := range splits {
		total += s.Percentage
	}
	if total < 99.99 || total > 100.01 {
		return fmt.Errorf("%w: got %.2f", ErrSplitPercentageNot100, total)
	}
	return nil
}
