package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/wouterdamman/home-finance/internal/domain"
)

// Month-totals endpoints of the integration API (scope periods:read). Totals
// use the same effective-amount SQL as /api/years/{y}/summary, so itemized
// income and tracked budget lines count correctly. No ids, no names.

var periodPathRe = regexp.MustCompile(`^\d{4}-\d{2}$`)

// parsePeriodPath parses a strict "yyyy-mm" path segment (month 01-12).
func parsePeriodPath(s string) (year, month int, ok bool) {
	if !periodPathRe.MatchString(s) {
		return 0, 0, false
	}
	year, _ = strconv.Atoi(s[:4])
	month, _ = strconv.Atoi(s[5:])
	if month < 1 || month > 12 {
		return 0, 0, false
	}
	return year, month, true
}

type periodRef struct {
	Year, Month int
	Status      string
}

// pickCurrentPeriod chooses the period "current" refers to: the one for the
// given calendar month, else the latest open one, else the latest of all.
func pickCurrentPeriod(periods []periodRef, nowYear, nowMonth int) (periodRef, bool) {
	later := func(a, b periodRef) bool { return a.Year > b.Year || (a.Year == b.Year && a.Month > b.Month) }
	var latest, latestOpen periodRef
	var haveAny, haveOpen bool
	for _, p := range periods {
		if p.Year == nowYear && p.Month == nowMonth {
			return p, true
		}
		if !haveAny || later(p, latest) {
			latest, haveAny = p, true
		}
		if p.Status == "open" && (!haveOpen || later(p, latestOpen)) {
			latestOpen, haveOpen = p, true
		}
	}
	if haveOpen {
		return latestOpen, true
	}
	return latest, haveAny
}

type periodTotals struct {
	Year           int        `json:"year"`
	Month          int        `json:"month"`
	Status         string     `json:"status"`
	ClosedAt       *time.Time `json:"closedAt"`
	IsCurrentMonth bool       `json:"isCurrentMonth"`
	IncomeCents    int64      `json:"incomeCents"`
	ExpenseCents   int64      `json:"expenseCents"`
	SurplusCents   int64      `json:"surplusCents"`
	GeneratedAt    string     `json:"generatedAt"`
}

// queryPeriodTotals returns nil when the period does not exist.
func (s *Server) queryPeriodTotals(ctx context.Context, year, month int, now time.Time) (*periodTotals, error) {
	pt := periodTotals{Year: year, Month: month}
	err := s.pool.QueryRow(ctx, `
		SELECT p.status, p.closed_at,
		  (SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=p.id),
		  (SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=p.id)
		FROM periods p WHERE p.year=$1 AND p.month=$2`, year, month).
		Scan(&pt.Status, &pt.ClosedAt, &pt.IncomeCents, &pt.ExpenseCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if pt.ClosedAt != nil {
		u := pt.ClosedAt.UTC()
		pt.ClosedAt = &u
	}
	n := now.In(amsterdam())
	pt.IsCurrentMonth = year == n.Year() && month == int(n.Month())
	pt.SurplusCents = pt.IncomeCents - pt.ExpenseCents
	pt.GeneratedAt = now.UTC().Format(time.RFC3339)
	return &pt, nil
}

func (s *Server) handleIntegrationPeriodCurrent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.pool.Query(ctx, `SELECT year, month, status FROM periods`)
	if err != nil {
		dbError(w, "integrationPeriodCurrent.list", err)
		return
	}
	defer rows.Close()
	var all []periodRef
	for rows.Next() {
		var p periodRef
		if err := rows.Scan(&p.Year, &p.Month, &p.Status); err != nil {
			dbError(w, "integrationPeriodCurrent.scan", err)
			return
		}
		all = append(all, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		dbError(w, "integrationPeriodCurrent.list", err)
		return
	}
	now := time.Now()
	n := now.In(amsterdam())
	p, ok := pickCurrentPeriod(all, n.Year(), int(n.Month()))
	if !ok {
		Error(w, http.StatusNotFound, "not_found", "no periods")
		return
	}
	pt, err := s.queryPeriodTotals(ctx, p.Year, p.Month, now)
	if err != nil {
		dbError(w, "integrationPeriodCurrent", err)
		return
	}
	if pt == nil { // deleted between the two queries
		Error(w, http.StatusNotFound, "not_found", "no periods")
		return
	}
	JSON(w, http.StatusOK, pt)
}

func (s *Server) handleIntegrationPeriod(w http.ResponseWriter, r *http.Request) {
	year, month, ok := parsePeriodPath(chi.URLParam(r, "ym"))
	if !ok {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	pt, err := s.queryPeriodTotals(r.Context(), year, month, time.Now())
	if err != nil {
		dbError(w, "integrationPeriod", err)
		return
	}
	if pt == nil {
		Error(w, http.StatusNotFound, "not_found", "period not found")
		return
	}
	JSON(w, http.StatusOK, pt)
}
