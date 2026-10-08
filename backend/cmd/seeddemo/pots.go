package main

import (
	"fmt"
	"math"
	"time"
)

// noLedgerMonth is the one month that deliberately gets zero pot_ledger
// activity of any kind (no allocation, no deposit/withdrawal) — the edge
// case a balance-over-time chart needs to handle.
const noLedgerMonth = 9

// allocationDate is the single, non-meaningful date every monthly
// 'allocation' entry carries, mirroring a real import-style batch job that
// stamps all its rows with the date it ran rather than the date they're
// "for" (that's what period_id is for).
var allocationDate = time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

type potSpec struct {
	name        string
	kind        string // 'normal' | 'carryover'
	targetCents *int64
	targetDate  *time.Time

	depositMonths     []int
	depositAmountMin  int64
	depositAmountMax  int64
	withdrawalMonths  []int // amounts computed from running balance at insert time
	withdrawalFracMin float64
	withdrawalFracMax float64
}

func ptr[T any](v T) *T { return &v }

func potSpecs() []potSpec {
	return []potSpec{
		{
			name: "Buffer", kind: "normal",
			depositMonths: []int{2, 5, 10}, depositAmountMin: 8000, depositAmountMax: 30000,
		},
		{
			name: "Vakantiepot", kind: "normal",
			targetCents: ptr[int64](300000), targetDate: ptr(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
			depositMonths: []int{3, 8}, depositAmountMin: 15000, depositAmountMax: 40000,
			// Withdrawal scheduled after the (fixed, non-meaningful)
			// allocation date of 2026-07-23 and after both deposits, so the
			// chronological balance simulation always has funds to draw
			// from — see seedPotLedgerForPot.
			withdrawalMonths: []int{10}, withdrawalFracMin: 0.3, withdrawalFracMax: 0.5,
		},
		{
			name: "Autofonds", kind: "normal",
			targetCents: ptr[int64](500000), targetDate: ptr(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)),
			depositMonths: []int{4, 11}, depositAmountMin: 10000, depositAmountMax: 25000,
			withdrawalMonths: []int{12}, withdrawalFracMin: 0.2, withdrawalFracMax: 0.4,
		},
		{
			name: "Verbouwpot", kind: "normal",
			// Many withdrawals (8), all dated after the fixed 2026-07-23
			// allocation date — Verbouwpot has no deposits of its own, so
			// before that date it has only whatever partial-month
			// allocations it already received, which isn't a safe base to
			// withdraw from.
			withdrawalMonths:  []int{8, 8, 10, 10, 11, 11, 12, 12},
			withdrawalFracMin: 0.1, withdrawalFracMax: 0.3,
		},
		{
			name: "Cadeaupot", kind: "normal",
			depositMonths: []int{6, 12}, depositAmountMin: 5000, depositAmountMax: 15000,
			withdrawalMonths: []int{12}, withdrawalFracMin: 0.2, withdrawalFracMax: 0.35,
		},
		{
			name: "Overschot", kind: "carryover",
			depositMonths: []int{5}, depositAmountMin: 10000, depositAmountMax: 20000,
		},
	}
}

func (s *seeder) seedPots() error {
	specs := potSpecs()
	s.potID = make(map[string]int64, len(specs))
	for order, spec := range specs {
		id, err := s.scanID(
			`INSERT INTO pots (name, kind, sort_order, target_cents, target_date)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			spec.name, spec.kind, order, spec.targetCents, spec.targetDate,
		)
		if err != nil {
			return fmt.Errorf("insert pot %q: %w", spec.name, err)
		}
		s.potID[spec.name] = id
	}

	// unitsByMonth[month] holds, per pot (same order as specs), the
	// hundredths-of-a-percent each pot got that month — reused both for the
	// pot_splits percentage and, proportionally, for that month's
	// allocation cents, so the two stay consistent with each other.
	unitsByMonth := make(map[int][]int64, 12)
	for month := 1; month <= 12; month++ {
		weights := make([]float64, len(specs))
		for i := range weights {
			base := seedRand.Float64()*10 + 0.5
			weights[i] = math.Pow(base, 2) // exaggerates month-to-month variance
		}
		units := largestRemainderInts(10000, weights)
		unitsByMonth[month] = units

		periodID := s.periodID[month]
		for i, spec := range specs {
			pct := float64(units[i]) / 100.0
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO pot_splits (period_id, pot_id, percentage) VALUES ($1,$2,$3)`,
				periodID, s.potID[spec.name], pct,
			); err != nil {
				return fmt.Errorf("insert pot_split %q month %d: %w", spec.name, month, err)
			}
		}
	}

	// Per-period "surplus" available to allocate to pots. Deliberately
	// decoupled from the actual income/expense totals generated elsewhere —
	// this only needs to be a plausible, comfortably positive figure so
	// allocations are realistic and never risk pushing a pot negative.
	surplusByMonth := make(map[int]int64, 10)
	for month := 1; month <= 10; month++ {
		surplusByMonth[month] = randRange(150000, 450000)
	}

	for i, spec := range specs {
		if err := s.seedPotLedgerForPot(i, spec, unitsByMonth, surplusByMonth); err != nil {
			return fmt.Errorf("pot ledger %q: %w", spec.name, err)
		}
	}
	return nil
}

type ledgerEvent struct {
	date        time.Time
	entryType   string
	amount      int64 // 0 for a pending withdrawal, computed during simulation
	pending     bool
	periodID    *int64
	description string
}

// seedPotLedgerForPot builds and inserts one pot's full ledger: at most one
// opening_balance (Buffer only), that pot's share of each closed period's
// allocation (skipping noLedgerMonth entirely), and its deposits/pending
// withdrawals. Withdrawal amounts are computed as a random fraction of the
// running balance at the moment they occur, during a single chronological
// simulation pass, which is what guarantees no pot ever goes negative.
func (s *seeder) seedPotLedgerForPot(potIdx int, spec potSpec, unitsByMonth map[int][]int64, surplusByMonth map[int]int64) error {
	potID := s.potID[spec.name]
	var events []ledgerEvent

	if spec.name == "Buffer" {
		events = append(events, ledgerEvent{
			date:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			entryType: "opening_balance", amount: 150000,
			description: "Startsaldo buffer",
		})
	}
	if spec.name == "Autofonds" {
		// Dated after the April deposit's random day (max day 24, see the
		// depositMonths loop below), so the real chronological order always
		// has something to adjust against.
		d := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
		events = append(events, ledgerEvent{
			date:      d,
			entryType: "adjustment", amount: -500,
			description: "Correctie afronding",
		})
	}

	for month := 1; month <= 10; month++ {
		if month == noLedgerMonth {
			continue
		}
		surplus := surplusByMonth[month]
		cents := largestRemainderInts(surplus, unitsToWeights(unitsByMonth[month]))[potIdx]
		periodID := s.periodID[month]
		events = append(events, ledgerEvent{
			date:      allocationDate,
			entryType: "allocation", amount: cents, periodID: &periodID,
			description: fmt.Sprintf("Automatische verdeling %s", monthName(month)),
		})
	}

	for _, month := range spec.depositMonths {
		if month == noLedgerMonth {
			continue
		}
		day := 5 + seedRand.Intn(20)
		d := time.Date(2026, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		amount := randRange(spec.depositAmountMin, spec.depositAmountMax)
		events = append(events, ledgerEvent{
			date:      d,
			entryType: "deposit", amount: amount,
			description: "Handmatige inleg",
		})
	}
	for _, month := range spec.withdrawalMonths {
		if month == noLedgerMonth {
			continue
		}
		day := 5 + seedRand.Intn(20)
		d := time.Date(2026, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		events = append(events, ledgerEvent{
			date:      d,
			entryType: "withdrawal", pending: true,
			description: "Onttrekking",
		})
	}

	// Chronological order (real entry_date, ties broken by the order events
	// were appended above) is what the app's own balance queries use
	// (`ORDER BY entry_date, id`) — the simulation below MUST match that
	// exactly, or a withdrawal could be judged "safe" here while the real
	// app sees it drawing against a balance that, on that actual calendar
	// date, doesn't exist yet (e.g. all allocations post on the same fixed
	// date, so a withdrawal dated earlier in the year can't draw on a
	// later month's allocation no matter which month it's "for").
	sortEventsByDate(events)

	var balance int64
	for _, ev := range events {
		amount := ev.amount
		if ev.pending {
			if balance < 1000 {
				continue // not enough left to safely withdraw from, skip this one
			}
			frac := spec.withdrawalFracMin + seedRand.Float64()*(spec.withdrawalFracMax-spec.withdrawalFracMin)
			amount = -int64(float64(balance) * frac)
			if amount == 0 {
				continue
			}
		}
		balance += amount
		if balance < 0 {
			// Shouldn't happen given the fraction-of-balance withdrawal
			// design, but never write a negative-balance-inducing row.
			balance -= amount
			continue
		}
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO pot_ledger (pot_id, period_id, entry_type, amount_cents, description, entry_date)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			potID, ev.periodID, ev.entryType, amount, ev.description, ev.date,
		); err != nil {
			return err
		}
		s.potLedgerCount++
	}
	return nil
}

func unitsToWeights(units []int64) []float64 {
	w := make([]float64, len(units))
	for i, u := range units {
		w[i] = float64(u)
	}
	return w
}

// sortEventsByDate orders by real entry_date ascending, stably preserving
// the original (append) order for equal dates — insertion-sort is fine
// here, the slice is always tiny (at most a dozen events per pot).
func sortEventsByDate(events []ledgerEvent) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].date.Before(events[j-1].date); j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}
