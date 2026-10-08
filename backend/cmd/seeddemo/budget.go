package main

import (
	"fmt"
	"time"
)

// overigRecurringGroups are spelling-variant groups for the Overig catch-all
// category: the same real-world merchant, typed differently each time, so
// grouping/normalization logic downstream has something realistic to chew
// on.
var overigRecurringGroups = [][]string{
	{"Bol.com", "bol.com ", "BOL.COM"},
	{"Albert Heijn", "albert heijn", "ALBERT HEIJN "},
	{"Action", "action", "ACTION "},
	{"Kruidvat", "kruidvat ", "KRUIDVAT"},
	{"Etos", "etos ", "ETOS"},
}

// overigOneOffDescriptions are one-off, non-recurring Overig transactions.
var overigOneOffDescriptions = []string{
	"Verjaardagscadeau buurman", "Reparatie fiets", "Planten tuincentrum",
	"Koffie onderweg", "Parkeergarage centrum", "Schoolspullen", "Boek",
	"App store aankoop", "Snack onderweg", "Taxi", "Fotoafdruk", "Postzegels",
	"Cadeaubon", "Statiegeld automaat", "Speelgoedwinkel", "Verf klussen",
	"Markt kraam", "Tweedehands aankoop", "Reparatie telefoon", "Batterijen",
	"Sleutel bijmaken", "Kaarsen", "Puzzelboek", "Donatie goed doel",
	"Lunch onderweg", "Treinkaartje los", "Fietsreparatie", "Horeca fooi",
	"Diverse aankoop", "Winkelcentrum overig",
}

// seedBudgetLinesAndTransactions inserts a budget_lines row for every
// (period, category) pair, then the ~900 transactions: a handful per
// category per month, plus the ~300 that pile into "Overig".
func (s *seeder) seedBudgetLinesAndTransactions() error {
	for month := 1; month <= 12; month++ {
		periodID := s.periodID[month]
		for order, spec := range s.catSpecs {
			catID := s.categoryID[spec.name]
			// Child categories get transactions but no budget line of their
			// own: effective spend resolves a budget line's amount through
			// category_rollup, which already folds a child's transactions
			// into its parent's line. Giving the child its own line too
			// counted those transactions twice, inflating every expense
			// total the trends widgets are read against.
			if spec.parent == "" {
				variance := randRange(-spec.monthlyBudget/10, spec.monthlyBudget/10)
				amount := spec.monthlyBudget + variance
				if _, err := s.tx.Exec(s.ctx,
					`INSERT INTO budget_lines (period_id, category_id, amount_cents, tracks_transactions, sort_order)
					 VALUES ($1,$2,$3,$4,$5)`,
					periodID, catID, amount, spec.tracksTransactions, order,
				); err != nil {
					return fmt.Errorf("insert budget_line %q month %d: %w", spec.name, month, err)
				}
			}

			if spec.name == "Overig" {
				continue // handled below, once we know the per-month share
			}
			n := spec.txPerMonthMin
			if spec.txPerMonthMax > spec.txPerMonthMin {
				n += seedRand.Intn(spec.txPerMonthMax - spec.txPerMonthMin + 1)
			}
			for i := 0; i < n; i++ {
				desc := spec.descriptions[seedRand.Intn(len(spec.descriptions))]
				// Expense amounts are stored POSITIVE: the app derives direction
				// from the table (transactions/budget_lines are outflow), and
				// surplus is income - expense. Negative rows here produced a
				// surplus larger than income and a savings rate above 100%.
				amount := randRange(500, spec.monthlyBudget/2+500)
				if err := s.insertTransaction(periodID, catID, amount, desc, month); err != nil {
					return err
				}
			}
		}
	}
	return s.seedOverigTransactions()
}

// seedOverigTransactions spreads ~300 transactions across all 12 months
// into the Overig category: a mix of inconsistently-spelled recurring
// merchants and one-off purchases.
func (s *seeder) seedOverigTransactions() error {
	const target = 300
	catID := s.categoryID["Overig"]
	perMonth := target / 12
	remaining := target - perMonth*12

	for month := 1; month <= 12; month++ {
		periodID := s.periodID[month]
		count := perMonth
		if remaining > 0 {
			count++
			remaining--
		}
		for i := 0; i < count; i++ {
			var desc string
			if seedRand.Intn(3) == 0 { // ~1/3 recurring-but-inconsistent merchants
				group := overigRecurringGroups[seedRand.Intn(len(overigRecurringGroups))]
				desc = group[seedRand.Intn(len(group))]
			} else {
				desc = overigOneOffDescriptions[seedRand.Intn(len(overigOneOffDescriptions))]
			}
			amount := randRange(300, 12000)
			if err := s.insertTransaction(periodID, catID, amount, desc, month); err != nil {
				return err
			}
		}
	}
	return nil
}

// insertTransaction inserts one transaction row, sprinkling in a NULL
// tx_date every so often (a handful out of ~900 total) to exercise that
// edge case downstream.
func (s *seeder) insertTransaction(periodID, categoryID int64, amountCents int64, description string, month int) error {
	s.txCount++

	var txDate *time.Time
	if s.txCount%73 != 0 {
		day := 1 + seedRand.Intn(28)
		d := time.Date(demoYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		txDate = &d
	}

	_, err := s.tx.Exec(s.ctx,
		`INSERT INTO transactions (period_id, category_id, amount_cents, description, tx_date)
		 VALUES ($1,$2,$3,$4,$5)`,
		periodID, categoryID, amountCents, description, txDate,
	)
	if err != nil {
		return fmt.Errorf("insert transaction (%q, month %d): %w", description, month, err)
	}
	return nil
}
