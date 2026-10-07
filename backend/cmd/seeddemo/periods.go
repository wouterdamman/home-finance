package main

import "time"

// seedYearAndPeriods creates the years row and all 12 periods for demoYear.
// Months 1-10 are closed (closed_at set to the 2nd of the following month,
// a plausible close date); 11-12 stay open.
func (s *seeder) seedYearAndPeriods() error {
	if _, err := s.tx.Exec(s.ctx, `INSERT INTO years (year) VALUES ($1) ON CONFLICT DO NOTHING`, demoYear); err != nil {
		return err
	}

	s.periodID = make(map[int]int64, 12)
	for month := 1; month <= 12; month++ {
		closed := month <= 10
		var closedAt *time.Time
		if closed {
			t := time.Date(demoYear, time.Month(month)+1, 2, 9, 0, 0, 0, time.UTC)
			closedAt = &t
		}
		status := "open"
		if closed {
			status = "closed"
		}
		id, err := s.scanID(
			`INSERT INTO periods (year, month, status, closed_at) VALUES ($1,$2,$3,$4) RETURNING id`,
			demoYear, month, status, closedAt,
		)
		if err != nil {
			return err
		}
		s.periodID[month] = id
	}
	return nil
}
