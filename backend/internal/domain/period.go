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

// EffectiveIncomeCentsSQL and EffectiveExpenseCentsSQL are the correlated-subquery
// snippets for a period's effective income/expense total. Itemized income sources
// (migration 0010) sum their income_transactions rows instead of using
// income_entries.amount_cents directly, mirroring how tracked budget lines sum
// their transactions rows instead of using budget_lines.amount_cents. Every query
// that totals a period's income or expenses must use these exact snippets — a plain
// SUM(amount_cents) silently drops itemized/tracked amounts.
//
// EffectiveIncomeCentsSQL expects the query to alias income_entries as "ie" and
// (left-)join income_sources as "isrc" on isrc.id = ie.source_id.
const EffectiveIncomeCentsSQL = `COALESCE(SUM(CASE WHEN isrc.is_itemized
	THEN COALESCE((SELECT SUM(it.amount_cents) FROM income_transactions it WHERE it.period_id=ie.period_id AND it.source_id=ie.source_id),0)
	ELSE ie.amount_cents END),0)`

// EffectiveExpenseCentsSQL expects the query to alias budget_lines as "bl".
const EffectiveExpenseCentsSQL = `COALESCE(SUM(CASE WHEN bl.tracks_transactions
	THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0)
	ELSE bl.amount_cents END),0)`

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
