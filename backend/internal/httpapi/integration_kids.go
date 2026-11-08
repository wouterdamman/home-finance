package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Fine-grained kids-savings endpoints of the integration API. Everything here
// is derived from the same ledger as the session API but deliberately exposes
// no ids and no ledger descriptions (free text).

type kidPeriod struct {
	DepositsCents    int64 `json:"depositsCents"`
	WithdrawalsCents int64 `json:"withdrawalsCents"`
	AdjustmentsCents int64 `json:"adjustmentsCents"`
	NetCents         int64 `json:"netCents"`
}

type kidFlows struct {
	OpeningBalanceCents int64     `json:"openingBalanceCents"`
	DepositsCents       int64     `json:"depositsCents"`
	WithdrawalsCents    int64     `json:"withdrawalsCents"`
	AdjustmentsCents    int64     `json:"adjustmentsCents"`
	ThisMonth           kidPeriod `json:"thisMonth"`
	ThisYear            kidPeriod `json:"thisYear"`
}

type kidLastEntry struct {
	Date        string `json:"date"`
	Type        string `json:"type"`
	Owner       string `json:"owner"`
	AmountCents int64  `json:"amountCents"`
	id          int64  // tie-breaker only, never serialised
}

type kidSubPot struct {
	Owner        string `json:"owner"`
	BalanceCents int64  `json:"balanceCents"`
	kidFlows
	LastEntry *kidLastEntry `json:"lastEntry"`
}

// amsterdam is the household's time zone for "this month" / "this year".
// time/tzdata is imported by cmd/server because the release image has no zoneinfo.
func amsterdam() *time.Location {
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		return time.UTC
	}
	return loc
}

type periodBounds struct{ monthStart, nextMonthStart, yearStart, nextYearStart time.Time }

// kidPeriodBounds returns the calendar month/year containing now in
// Europe/Amsterdam as UTC-midnight dates, ready to bind to DATE parameters.
func kidPeriodBounds(now time.Time) periodBounds {
	n := now.In(amsterdam())
	d := func(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
	return periodBounds{
		monthStart:     d(n.Year(), n.Month()),
		nextMonthStart: d(n.Year(), n.Month()+1),
		yearStart:      d(n.Year(), time.January),
		nextYearStart:  d(n.Year()+1, time.January),
	}
}

// querySubPots returns the sub-pots of one kid, always both owners.
func (s *Server) querySubPots(ctx context.Context, kidID int64, now time.Time) (map[string]*kidSubPot, error) {
	b := kidPeriodBounds(now)
	pots := map[string]*kidSubPot{"ours": {Owner: "ours"}, "theirs": {Owner: "theirs"}}

	rows, err := s.pool.Query(ctx, `
		SELECT owner,
		  COALESCE(SUM(amount_cents), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'opening_balance'), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'deposit'), 0)::BIGINT,
		  COALESCE(-SUM(amount_cents) FILTER (WHERE entry_type = 'withdrawal'), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'adjustment'), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'deposit' AND entry_date >= $2 AND entry_date < $3), 0)::BIGINT,
		  COALESCE(-SUM(amount_cents) FILTER (WHERE entry_type = 'withdrawal' AND entry_date >= $2 AND entry_date < $3), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'adjustment' AND entry_date >= $2 AND entry_date < $3), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'deposit' AND entry_date >= $4 AND entry_date < $5), 0)::BIGINT,
		  COALESCE(-SUM(amount_cents) FILTER (WHERE entry_type = 'withdrawal' AND entry_date >= $4 AND entry_date < $5), 0)::BIGINT,
		  COALESCE(SUM(amount_cents) FILTER (WHERE entry_type = 'adjustment' AND entry_date >= $4 AND entry_date < $5), 0)::BIGINT
		FROM kid_savings_ledger
		WHERE kid_id = $1
		GROUP BY owner`, kidID, b.monthStart, b.nextMonthStart, b.yearStart, b.nextYearStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner string
		var bal int64
		var f kidFlows
		if err := rows.Scan(&owner, &bal, &f.OpeningBalanceCents, &f.DepositsCents, &f.WithdrawalsCents, &f.AdjustmentsCents,
			&f.ThisMonth.DepositsCents, &f.ThisMonth.WithdrawalsCents, &f.ThisMonth.AdjustmentsCents,
			&f.ThisYear.DepositsCents, &f.ThisYear.WithdrawalsCents, &f.ThisYear.AdjustmentsCents); err != nil {
			return nil, err
		}
		f.ThisMonth.NetCents = f.ThisMonth.DepositsCents - f.ThisMonth.WithdrawalsCents + f.ThisMonth.AdjustmentsCents
		f.ThisYear.NetCents = f.ThisYear.DepositsCents - f.ThisYear.WithdrawalsCents + f.ThisYear.AdjustmentsCents
		if p := pots[owner]; p != nil {
			p.BalanceCents, p.kidFlows = bal, f
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	le, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (owner) owner, entry_date, entry_type, amount_cents, id
		FROM kid_savings_ledger
		WHERE kid_id = $1
		ORDER BY owner, entry_date DESC, id DESC`, kidID)
	if err != nil {
		return nil, err
	}
	defer le.Close()
	for le.Next() {
		var e kidLastEntry
		var d time.Time
		if err := le.Scan(&e.Owner, &d, &e.Type, &e.AmountCents, &e.id); err != nil {
			return nil, err
		}
		e.Date = d.Format("2006-01-02")
		if p := pots[e.Owner]; p != nil {
			p.LastEntry = &e
		}
	}
	return pots, le.Err()
}

// combine adds the flows of both sub-pots.
func combineFlows(a, b kidFlows) kidFlows {
	add := func(x, y kidPeriod) kidPeriod {
		return kidPeriod{x.DepositsCents + y.DepositsCents, x.WithdrawalsCents + y.WithdrawalsCents,
			x.AdjustmentsCents + y.AdjustmentsCents, x.NetCents + y.NetCents}
	}
	return kidFlows{
		OpeningBalanceCents: a.OpeningBalanceCents + b.OpeningBalanceCents,
		DepositsCents:       a.DepositsCents + b.DepositsCents,
		WithdrawalsCents:    a.WithdrawalsCents + b.WithdrawalsCents,
		AdjustmentsCents:    a.AdjustmentsCents + b.AdjustmentsCents,
		ThisMonth:           add(a.ThisMonth, b.ThisMonth),
		ThisYear:            add(a.ThisYear, b.ThisYear),
	}
}

func latestEntry(a, b *kidLastEntry) *kidLastEntry {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	// Dates are YYYY-MM-DD so string order is chronological; ties go to the higher id.
	if b.Date > a.Date || (b.Date == a.Date && b.id > a.id) {
		return b
	}
	return a
}

// lookupKid resolves the {slug} path value to an active kid, writing the 404
// itself when it cannot.
func (s *Server) lookupKid(w http.ResponseWriter, r *http.Request) (kidBalanceRow, bool) {
	slug := chi.URLParam(r, "slug")
	if !validKidSlugPath(slug) {
		Error(w, http.StatusNotFound, "not_found", "kid not found")
		return kidBalanceRow{}, false
	}
	rows, err := s.queryKidBalances(r.Context())
	if err != nil {
		dbError(w, "integrationKid.lookup", err)
		return kidBalanceRow{}, false
	}
	in := make([]slugInput, len(rows))
	for i, ro := range rows {
		in[i] = slugInput{ro.KidID, ro.Name}
	}
	slugs := assignKidSlugs(in)
	for _, ro := range rows {
		if slugs[ro.KidID] == slug {
			return ro, true
		}
	}
	Error(w, http.StatusNotFound, "not_found", "kid not found")
	return kidBalanceRow{}, false
}

func (s *Server) handleIntegrationKids(w http.ResponseWriter, r *http.Request) {
	type kid struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		TotalCents  int64  `json:"totalCents"`
		OursCents   int64  `json:"oursCents"`
		TheirsCents int64  `json:"theirsCents"`
	}
	type totals struct {
		TotalCents  int64 `json:"totalCents"`
		OursCents   int64 `json:"oursCents"`
		TheirsCents int64 `json:"theirsCents"`
	}
	rows, err := s.queryKidBalances(r.Context())
	if err != nil {
		dbError(w, "integrationKids", err)
		return
	}
	in := make([]slugInput, len(rows))
	for i, ro := range rows {
		in[i] = slugInput{ro.KidID, ro.Name}
	}
	slugs := assignKidSlugs(in)
	kids := make([]kid, 0, len(rows))
	var t totals
	for _, ro := range rows {
		kids = append(kids, kid{slugs[ro.KidID], ro.Name, ro.TotalCents, ro.OursCents, ro.TheirsCents})
		t.TotalCents += ro.TotalCents
		t.OursCents += ro.OursCents
		t.TheirsCents += ro.TheirsCents
	}
	JSON(w, http.StatusOK, map[string]any{
		"kids":        kids,
		"totals":      t,
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleIntegrationKidDetail(w http.ResponseWriter, r *http.Request) {
	ro, ok := s.lookupKid(w, r)
	if !ok {
		return
	}
	now := time.Now()
	pots, err := s.querySubPots(r.Context(), ro.KidID, now)
	if err != nil {
		dbError(w, "integrationKidDetail", err)
		return
	}
	ours, theirs := pots["ours"], pots["theirs"]
	total := ours.BalanceCents + theirs.BalanceCents

	bank := map[string]any{"reportedBalanceCents": nil, "reportedBalanceDate": nil, "differenceCents": nil}
	if ro.ReportedBalanceCents != nil {
		bank["reportedBalanceCents"] = *ro.ReportedBalanceCents
		bank["reportedBalanceDate"] = ro.ReportedBalanceDate
		bank["differenceCents"] = total - *ro.ReportedBalanceCents
	}
	JSON(w, http.StatusOK, map[string]any{
		"slug":        chi.URLParam(r, "slug"),
		"name":        ro.Name,
		"totalCents":  total,
		"combined":    combineFlows(ours.kidFlows, theirs.kidFlows),
		"pots":        map[string]*kidSubPot{"ours": ours, "theirs": theirs},
		"bank":        bank,
		"lastEntry":   latestEntry(ours.LastEntry, theirs.LastEntry),
		"generatedAt": now.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleIntegrationKidSubPot(owner string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ro, ok := s.lookupKid(w, r)
		if !ok {
			return
		}
		now := time.Now()
		pots, err := s.querySubPots(r.Context(), ro.KidID, now)
		if err != nil {
			dbError(w, "integrationKidSubPot", err)
			return
		}
		JSON(w, http.StatusOK, struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
			*kidSubPot
			GeneratedAt string `json:"generatedAt"`
		}{chi.URLParam(r, "slug"), ro.Name, pots[owner], now.UTC().Format(time.RFC3339)})
	}
}
