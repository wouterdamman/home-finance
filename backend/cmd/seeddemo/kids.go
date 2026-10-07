package main

import (
	"fmt"
	"time"
)

// seedKidSavings adds ledger activity for the two existing kids rows,
// covering both owners ('ours'/'theirs') and a reported balance snapshot.
// It does not create or rename kid rows — those predate this tool and are
// real data, not something this seeder invents.
func (s *seeder) seedKidSavings() error {
	rows, err := s.tx.Query(s.ctx,
		`SELECT id FROM kids WHERE archived_at IS NULL ORDER BY sort_order, id LIMIT 2`)
	if err != nil {
		return fmt.Errorf("query kids: %w", err)
	}
	var kidIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		kidIDs = append(kidIDs, id)
	}
	rows.Close()
	if len(kidIDs) < 2 {
		return fmt.Errorf("expected at least 2 kids rows to seed savings for, found %d", len(kidIDs))
	}

	type entry struct {
		owner       string
		entryType   string
		amountCents int64
		description string
		date        time.Time
	}
	entries := []entry{
		{"ours", "opening_balance", 50000, "Startsaldo", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"ours", "deposit", 2500, "Maandelijkse inleg januari", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"ours", "deposit", 2500, "Maandelijkse inleg april", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)},
		{"ours", "deposit", 2500, "Maandelijkse inleg augustus", time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)},
		{"ours", "withdrawal", -3000, "Nieuwe fiets", time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)},
		{"theirs", "opening_balance", 30000, "Startsaldo kinderrekening", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"theirs", "deposit", 1500, "Zakgeld opa en oma", time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)},
		{"theirs", "deposit", 2000, "Verjaardagsgeld", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
		{"theirs", "adjustment", -200, "Correctie rente", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, kidID := range kidIDs {
		for _, e := range entries {
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO kid_savings_ledger (kid_id, owner, entry_type, amount_cents, description, entry_date)
				 VALUES ($1,$2,$3,$4,$5,$6)`,
				kidID, e.owner, e.entryType, e.amountCents, e.description, e.date,
			); err != nil {
				return fmt.Errorf("insert kid_savings_ledger kid=%d: %w", kidID, err)
			}
		}
		reportedBalance := int64(0)
		for _, e := range entries {
			reportedBalance += e.amountCents
		}
		reportedDate := time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)
		if _, err := s.tx.Exec(s.ctx,
			`UPDATE kids SET reported_balance_cents=$1, reported_balance_date=$2 WHERE id=$3`,
			reportedBalance, reportedDate, kidID,
		); err != nil {
			return fmt.Errorf("update kid reported balance kid=%d: %w", kidID, err)
		}
	}
	return nil
}
