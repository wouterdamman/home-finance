package httpapi

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/wouterdamman/home-finance/internal/domain"
)

// Year-totals endpoints of the integration API (scope years:read). Same
// effective-amount SQL as the month endpoints. No ids, no names.

var yearPathRe = regexp.MustCompile(`^\d{4}$`)

// parseYearPath parses a strict "yyyy" path segment in 2000-2100 (the range
// the years table allows).
func parseYearPath(s string) (int, bool) {
	if !yearPathRe.MatchString(s) {
		return 0, false
	}
	y, _ := strconv.Atoi(s)
	if y < 2000 || y > 2100 {
		return 0, false
	}
	return y, true
}

// averageMonthlySurplus is surplus / periodCount rounded half away from zero;
// 0 when there are no periods.
func averageMonthlySurplus(surplus int64, periodCount int) int64 {
	if periodCount <= 0 {
		return 0
	}
	n := int64(periodCount)
	abs := surplus
	if abs < 0 {
		abs = -abs
	}
	q := (2*abs + n) / (2 * n)
	if surplus < 0 {
		return -q
	}
	return q
}

type yearTotals struct {
	Year                       int   `json:"year"`
	Locked                     bool  `json:"locked"`
	PeriodsOpen                int   `json:"periodsOpen"`
	PeriodsClosed              int   `json:"periodsClosed"`
	IncomeCents                int64 `json:"incomeCents"`
	ExpenseCents               int64 `json:"expenseCents"`
	SurplusCents               int64 `json:"surplusCents"`
	AverageMonthlySurplusCents int64 `json:"averageMonthlySurplusCents"`
}

// queryYearTotals aggregates every year in `years` ∪ `periods.year`, oldest
// first. A non-nil only restricts the result to that year.
func (s *Server) queryYearTotals(ctx context.Context, only *int) ([]yearTotals, error) {
	rows, err := s.pool.Query(ctx, `
		WITH ys AS (
		  SELECT year FROM years UNION SELECT year FROM periods
		), pt AS (
		  SELECT p.year, p.status,
		    (SELECT `+domain.EffectiveIncomeCentsSQL+` FROM income_entries ie LEFT JOIN income_sources isrc ON isrc.id=ie.source_id WHERE ie.period_id=p.id) AS inc,
		    (SELECT `+domain.EffectiveExpenseCentsSQL+` FROM budget_lines bl WHERE bl.period_id=p.id) AS exp
		  FROM periods p WHERE ($1::int IS NULL OR p.year=$1)
		)
		SELECT ys.year,
		  EXISTS(SELECT 1 FROM locked_years ly WHERE ly.year=ys.year),
		  (COUNT(pt.year) FILTER (WHERE pt.status='open'))::int,
		  (COUNT(pt.year) FILTER (WHERE pt.status='closed'))::int,
		  COALESCE(SUM(pt.inc),0)::BIGINT,
		  COALESCE(SUM(pt.exp),0)::BIGINT
		FROM ys LEFT JOIN pt ON pt.year=ys.year
		WHERE ($1::int IS NULL OR ys.year=$1)
		GROUP BY ys.year ORDER BY ys.year`, only)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]yearTotals, 0)
	for rows.Next() {
		var y yearTotals
		if err := rows.Scan(&y.Year, &y.Locked, &y.PeriodsOpen, &y.PeriodsClosed, &y.IncomeCents, &y.ExpenseCents); err != nil {
			return nil, err
		}
		y.SurplusCents = y.IncomeCents - y.ExpenseCents
		y.AverageMonthlySurplusCents = averageMonthlySurplus(y.SurplusCents, y.PeriodsOpen+y.PeriodsClosed)
		out = append(out, y)
	}
	return out, rows.Err()
}

func (s *Server) handleIntegrationYears(w http.ResponseWriter, r *http.Request) {
	ys, err := s.queryYearTotals(r.Context(), nil)
	if err != nil {
		dbError(w, "integrationYears", err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"years":       ys,
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

// yearResponse is YearTotals plus generatedAt.
type yearResponse struct {
	yearTotals
	GeneratedAt string `json:"generatedAt"`
}

func (s *Server) handleIntegrationYearCurrent(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year := now.In(amsterdam()).Year()
	ys, err := s.queryYearTotals(r.Context(), &year)
	if err != nil {
		dbError(w, "integrationYearCurrent", err)
		return
	}
	yt := yearTotals{Year: year} // zeros when the year has no data yet
	if len(ys) == 1 {
		yt = ys[0]
	}
	JSON(w, http.StatusOK, yearResponse{yt, now.UTC().Format(time.RFC3339)})
}

func (s *Server) handleIntegrationYear(w http.ResponseWriter, r *http.Request) {
	year, ok := parseYearPath(chi.URLParam(r, "yyyy"))
	if !ok {
		Error(w, http.StatusNotFound, "not_found", "year not found")
		return
	}
	ys, err := s.queryYearTotals(r.Context(), &year)
	if err != nil {
		dbError(w, "integrationYear", err)
		return
	}
	if len(ys) == 0 {
		Error(w, http.StatusNotFound, "not_found", "year not found")
		return
	}
	JSON(w, http.StatusOK, yearResponse{ys[0], time.Now().UTC().Format(time.RFC3339)})
}
