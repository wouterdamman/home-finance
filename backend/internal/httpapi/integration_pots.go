package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Fine-grained savings-pot endpoints of the integration API. Like the kids
// endpoints they expose no ids (pot, period) and no ledger descriptions.

type potPeriod struct {
	AllocationsCents  int64 `json:"allocationsCents"`
	DepositsCents     int64 `json:"depositsCents"`
	WithdrawalsCents  int64 `json:"withdrawalsCents"`
	CarryoverOutCents int64 `json:"carryoverOutCents"`
	AdjustmentsCents  int64 `json:"adjustmentsCents"`
	NetCents          int64 `json:"netCents"`
}

func (p *potPeriod) setNet() {
	p.NetCents = p.AllocationsCents + p.DepositsCents - p.WithdrawalsCents - p.CarryoverOutCents + p.AdjustmentsCents
}

type potFlows struct {
	OpeningBalanceCents int64     `json:"openingBalanceCents"`
	AllocationsCents    int64     `json:"allocationsCents"`
	DepositsCents       int64     `json:"depositsCents"`
	WithdrawalsCents    int64     `json:"withdrawalsCents"`
	CarryoverOutCents   int64     `json:"carryoverOutCents"`
	AdjustmentsCents    int64     `json:"adjustmentsCents"`
	ThisMonth           potPeriod `json:"thisMonth"`
	ThisYear            potPeriod `json:"thisYear"`
}

type potTarget struct {
	TargetCents         int64    `json:"targetCents"`
	TargetDate          *string  `json:"targetDate"`
	RemainingCents      int64    `json:"remainingCents"`
	ProgressPercent     *float64 `json:"progressPercent"`
	Reached             bool     `json:"reached"`
	MonthsLeft          *int     `json:"monthsLeft"`
	NeededPerMonthCents *int64   `json:"neededPerMonthCents"`
}

type potSplit struct {
	Percentage   float64 `json:"percentage"`
	PeriodYear   int     `json:"periodYear"`
	PeriodMonth  int     `json:"periodMonth"`
	PeriodStatus string  `json:"periodStatus"`
}

type potLastEntry struct {
	Date        string `json:"date"`
	Type        string `json:"type"`
	AmountCents int64  `json:"amountCents"`
}

type potRow struct {
	ID           int64
	Name         string
	Kind         string
	BalanceCents int64
	TargetCents  *int64
	TargetDate   *time.Time
}

// potProgressPercent is balance/target*100 rounded to one decimal and not
// capped at 100. A target of zero or less has no meaningful ratio, so the
// result is nil (the pot counts as reached through potTargetInfo instead).
func potProgressPercent(balance, target int64) *float64 {
	if target <= 0 {
		return nil
	}
	v := math.Round(float64(balance)/float64(target)*1000) / 10
	return &v
}

// potTargetInfo does the savings-goal maths. Months are whole calendar months
// from now's month (Europe/Amsterdam) to the target date's month, never
// negative. Needed-per-month is the remaining amount spread over the months
// left, rounded up; with the target month already reached or past it is the
// whole remainder, and with no date it is null.
func potTargetInfo(balance, target int64, targetDate *time.Time, now time.Time) potTarget {
	t := potTarget{
		TargetCents:     target,
		RemainingCents:  max(target-balance, 0),
		ProgressPercent: potProgressPercent(balance, target),
		Reached:         balance >= target,
	}
	if targetDate == nil {
		return t
	}
	ds := targetDate.Format("2006-01-02")
	t.TargetDate = &ds
	n := now.In(amsterdam())
	left := max((targetDate.Year()-n.Year())*12+int(targetDate.Month())-int(n.Month()), 0)
	t.MonthsLeft = &left
	var need int64
	if !t.Reached {
		div := int64(max(left, 1))
		need = (t.RemainingCents + div - 1) / div
	}
	t.NeededPerMonthCents = &need
	return t
}

// queryActivePots returns the non-archived pots with their balances, in
// display order.
func (s *Server) queryActivePots(ctx context.Context) ([]potRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.kind, COALESCE(SUM(pl.amount_cents),0)::BIGINT, p.target_cents, p.target_date
		FROM pots p LEFT JOIN pot_ledger pl ON pl.pot_id=p.id
		WHERE p.archived_at IS NULL
		GROUP BY p.id, p.name, p.kind, p.sort_order, p.target_cents, p.target_date
		ORDER BY p.sort_order, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []potRow
	for rows.Next() {
		var r potRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.BalanceCents, &r.TargetCents, &r.TargetDate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func potSlugs(rows []potRow) map[int64]string {
	in := make([]slugInput, len(rows))
	for i, r := range rows {
		in[i] = slugInput{r.ID, r.Name}
	}
	return assignSlugs(in, nil, "pot")
}

// queryPotFlows aggregates one pot's ledger. A row counts in its period's
// (year, month) when it has a period_id and in its entry_date's otherwise,
// the same convention as /api/trends/pot-balances. Withdrawals and
// carryover_out are stored negative and returned as positive magnitudes.
func (s *Server) queryPotFlows(ctx context.Context, potID int64, now time.Time) (int64, potFlows, error) {
	n := now.In(amsterdam())
	var bal int64
	var f potFlows
	err := s.pool.QueryRow(ctx, `
		WITH l AS (
		  SELECT pl.entry_type AS t, pl.amount_cents AS a,
		    COALESCE(p.year, EXTRACT(YEAR FROM pl.entry_date)::int) AS y,
		    COALESCE(p.month, EXTRACT(MONTH FROM pl.entry_date)::int) AS m
		  FROM pot_ledger pl LEFT JOIN periods p ON p.id = pl.period_id
		  WHERE pl.pot_id = $1
		)
		SELECT
		  COALESCE(SUM(a), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'opening_balance'), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'allocation'), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'deposit'), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'withdrawal'), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'carryover_out'), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'adjustment'), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'allocation' AND y = $2 AND m = $3), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'deposit' AND y = $2 AND m = $3), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'withdrawal' AND y = $2 AND m = $3), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'carryover_out' AND y = $2 AND m = $3), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'adjustment' AND y = $2 AND m = $3), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'allocation' AND y = $2), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'deposit' AND y = $2), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'withdrawal' AND y = $2), 0)::BIGINT,
		  COALESCE(-SUM(a) FILTER (WHERE t = 'carryover_out' AND y = $2), 0)::BIGINT,
		  COALESCE(SUM(a) FILTER (WHERE t = 'adjustment' AND y = $2), 0)::BIGINT
		FROM l`, potID, n.Year(), int(n.Month())).Scan(
		&bal, &f.OpeningBalanceCents, &f.AllocationsCents, &f.DepositsCents, &f.WithdrawalsCents, &f.CarryoverOutCents, &f.AdjustmentsCents,
		&f.ThisMonth.AllocationsCents, &f.ThisMonth.DepositsCents, &f.ThisMonth.WithdrawalsCents, &f.ThisMonth.CarryoverOutCents, &f.ThisMonth.AdjustmentsCents,
		&f.ThisYear.AllocationsCents, &f.ThisYear.DepositsCents, &f.ThisYear.WithdrawalsCents, &f.ThisYear.CarryoverOutCents, &f.ThisYear.AdjustmentsCents)
	if err != nil {
		return 0, f, err
	}
	f.ThisMonth.setNet()
	f.ThisYear.setNet()
	return bal, f, nil
}

// queryPotSplit finds the pot's reference split: the latest open period if
// the pot has a split row there, otherwise the latest period (any status)
// with a split row for this pot. nil when the pot has no split row at all.
func (s *Server) queryPotSplit(ctx context.Context, potID int64) (*potSplit, error) {
	var sp potSplit
	err := s.pool.QueryRow(ctx, `
		SELECT ps.percentage::float8, p.year, p.month, p.status
		FROM pot_splits ps JOIN periods p ON p.id = ps.period_id
		WHERE ps.pot_id = $1
		ORDER BY (p.id = (SELECT id FROM periods WHERE status = 'open' ORDER BY year DESC, month DESC LIMIT 1)) DESC,
		         p.year DESC, p.month DESC
		LIMIT 1`, potID).Scan(&sp.Percentage, &sp.PeriodYear, &sp.PeriodMonth, &sp.PeriodStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Server) queryPotLastEntry(ctx context.Context, potID int64) (*potLastEntry, error) {
	var e potLastEntry
	var d time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT entry_date, entry_type, amount_cents FROM pot_ledger
		WHERE pot_id = $1 ORDER BY entry_date DESC, id DESC LIMIT 1`, potID).Scan(&d, &e.Type, &e.AmountCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.Date = d.Format("2006-01-02")
	return &e, nil
}

func (s *Server) handleIntegrationPots(w http.ResponseWriter, r *http.Request) {
	type pot struct {
		Slug            string   `json:"slug"`
		Name            string   `json:"name"`
		Kind            string   `json:"kind"`
		BalanceCents    int64    `json:"balanceCents"`
		TargetCents     *int64   `json:"targetCents"`
		ProgressPercent *float64 `json:"progressPercent"`
	}
	rows, err := s.queryActivePots(r.Context())
	if err != nil {
		dbError(w, "integrationPots", err)
		return
	}
	slugs := potSlugs(rows)
	pots := make([]pot, 0, len(rows))
	var totalBal, totalTarget int64
	for _, ro := range rows {
		p := pot{Slug: slugs[ro.ID], Name: ro.Name, Kind: ro.Kind, BalanceCents: ro.BalanceCents, TargetCents: ro.TargetCents}
		if ro.TargetCents != nil {
			p.ProgressPercent = potProgressPercent(ro.BalanceCents, *ro.TargetCents)
			totalTarget += *ro.TargetCents
		}
		totalBal += ro.BalanceCents
		pots = append(pots, p)
	}
	JSON(w, http.StatusOK, map[string]any{
		"pots":        pots,
		"totals":      map[string]int64{"balanceCents": totalBal, "targetCents": totalTarget},
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleIntegrationPotDetail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !validKidSlugPath(slug) {
		Error(w, http.StatusNotFound, "not_found", "pot not found")
		return
	}
	ctx := r.Context()
	rows, err := s.queryActivePots(ctx)
	if err != nil {
		dbError(w, "integrationPot.lookup", err)
		return
	}
	slugs := potSlugs(rows)
	var ro *potRow
	for i := range rows {
		if slugs[rows[i].ID] == slug {
			ro = &rows[i]
			break
		}
	}
	if ro == nil {
		Error(w, http.StatusNotFound, "not_found", "pot not found")
		return
	}
	now := time.Now()
	bal, flows, err := s.queryPotFlows(ctx, ro.ID, now)
	if err != nil {
		dbError(w, "integrationPot.flows", err)
		return
	}
	split, err := s.queryPotSplit(ctx, ro.ID)
	if err != nil {
		dbError(w, "integrationPot.split", err)
		return
	}
	last, err := s.queryPotLastEntry(ctx, ro.ID)
	if err != nil {
		dbError(w, "integrationPot.last", err)
		return
	}
	var target *potTarget
	if ro.TargetCents != nil {
		t := potTargetInfo(bal, *ro.TargetCents, ro.TargetDate, now)
		target = &t
	}
	JSON(w, http.StatusOK, map[string]any{
		"slug":         slug,
		"name":         ro.Name,
		"kind":         ro.Kind,
		"balanceCents": bal,
		"flows":        flows,
		"target":       target,
		"split":        split,
		"lastEntry":    last,
		"generatedAt":  now.UTC().Format(time.RFC3339),
	})
}
