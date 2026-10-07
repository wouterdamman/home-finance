package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// demoKidNames are this seeder's own kid rows. It used to borrow whatever
// kids already existed, which both required a non-empty database and left a
// fabricated reported balance on a real child's row that the wipe never
// undid — the savings screen then showed a permanent reported-vs-ledger
// mismatch. Owning the rows keeps every write reversible.
var demoKidNames = []string{"Mees", "Fenna"}

func kidNames() []string { return demoKidNames }

// seedKidSavings creates this seeder's kid rows (if they are not there yet)
// and gives each one ledger activity for both owners ('ours'/'theirs') plus
// a reported balance snapshot. Kid rows it does not own are never touched.
func (s *seeder) seedKidSavings() error {
	kidIDs := make([]int64, 0, len(demoKidNames))
	for order, name := range demoKidNames {
		// kids.name has no unique constraint, so ON CONFLICT cannot be used:
		// look the row up first and only insert when it is really missing,
		// otherwise every run would add another duplicate child.
		var id int64
		err := s.tx.QueryRow(s.ctx,
			`SELECT id FROM kids WHERE name=$1 ORDER BY id LIMIT 1`, name,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := s.tx.QueryRow(s.ctx,
				`INSERT INTO kids (name, sort_order) VALUES ($1,$2) RETURNING id`,
				name, 100+order,
			).Scan(&id); err != nil {
				return fmt.Errorf("insert kid %q: %w", name, err)
			}
		} else if err != nil {
			return fmt.Errorf("resolve kid %q: %w", name, err)
		}
		kidIDs = append(kidIDs, id)
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
