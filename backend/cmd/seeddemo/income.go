package main

import (
	"fmt"
	"time"
)

// incomeSourceSpec describes one of the 8 invented income sources. Exactly
// two (Belastingdienst, Energieleverancier) are itemized: for those, amounts
// come from income_transactions line items instead of a flat monthly
// amount, matching domain.EffectiveIncomeCentsSQL.
type incomeSourceSpec struct {
	name        string
	isItemized  bool
	months      []int // active months; nil/empty = all 12
	monthAmount func(month int) int64

	// itemized only
	itemDescriptions []string
	itemAmountMin    int64
	itemAmountMax    int64
	itemsPerMonthMin int
	itemsPerMonthMax int
}

func activeMonths(spec incomeSourceSpec) []int {
	if len(spec.months) > 0 {
		return spec.months
	}
	all := make([]int, 12)
	for i := range all {
		all[i] = i + 1
	}
	return all
}

func incomeSourceSpecs() []incomeSourceSpec {
	return []incomeSourceSpec{
		{
			name: "Salaris Mila",
			monthAmount: func(month int) int64 {
				base := randRange(290000, 312000)
				if month == 5 { // vakantiegeld spike
					base += randRange(170000, 195000)
				}
				return base
			},
		},
		{
			name: "Salaris Joris",
			monthAmount: func(month int) int64 {
				return randRange(258000, 284000)
			},
		},
		{
			name: "Kinderbijslag",
			monthAmount: func(month int) int64 {
				return randRange(24800, 26200)
			},
		},
		{
			name: "Laadpaal vergoeding",
			monthAmount: func(month int) int64 {
				return randRange(3800, 9200)
			},
		},
		{
			name: "Cashback creditcard",
			monthAmount: func(month int) int64 {
				return randRange(900, 4100)
			},
		},
		{
			name: "Verhuur garagebox",
			monthAmount: func(month int) int64 {
				return 12000
			},
		},
		{
			name:             "Belastingdienst",
			isItemized:       true,
			months:           []int{1, 2, 3, 4}, // stops after April
			itemDescriptions: []string{"Kinderopvangtoeslag", "Hypotheekrenteaftrek", "IACK", "Teruggave inkomstenbelasting", "Zorgtoeslag correctie"},
			itemAmountMin:    4500,
			itemAmountMax:    38000,
			itemsPerMonthMin: 2,
			itemsPerMonthMax: 4,
		},
		{
			name:             "Energieleverancier",
			isItemized:       true,
			months:           []int{6, 7, 8, 9, 10, 11, 12}, // only starts in June
			itemDescriptions: []string{"Teruggave voorschot", "Terugbetaling heffingskorting", "Teruggave jaarafrekening", "Correctie meterstand"},
			itemAmountMin:    1500,
			itemAmountMax:    16000,
			itemsPerMonthMin: 1,
			itemsPerMonthMax: 3,
		},
	}
}

func (s *seeder) seedIncome() error {
	s.sourceID = make(map[string]int64)
	specs := incomeSourceSpecs()

	for order, spec := range specs {
		id, err := s.scanID(
			`INSERT INTO income_sources (name, default_amount_cents, sort_order, is_itemized)
			 VALUES ($1,$2,$3,$4) RETURNING id`,
			spec.name, 0, order, spec.isItemized,
		)
		if err != nil {
			return fmt.Errorf("insert income_source %q: %w", spec.name, err)
		}
		s.sourceID[spec.name] = id

		for _, month := range activeMonths(spec) {
			periodID := s.periodID[month]
			if !spec.isItemized {
				amount := spec.monthAmount(month)
				if _, err := s.tx.Exec(s.ctx,
					`INSERT INTO income_entries (period_id, source_id, amount_cents, entry_type, sort_order)
					 VALUES ($1,$2,$3,'normal',$4)`,
					periodID, id, amount, order,
				); err != nil {
					return fmt.Errorf("insert income_entry %q month %d: %w", spec.name, month, err)
				}
				continue
			}

			// Itemized: a handful of income_transactions line items, plus a
			// matching income_entries row (its amount_cents is cosmetic —
			// EffectiveIncomeCentsSQL ignores it for itemized sources and
			// sums income_transactions instead — but a row must exist for the
			// period/source to be picked up at all).
			n := spec.itemsPerMonthMin
			if spec.itemsPerMonthMax > spec.itemsPerMonthMin {
				n += seedRand.Intn(spec.itemsPerMonthMax - spec.itemsPerMonthMin + 1)
			}
			var total int64
			for i := 0; i < n; i++ {
				desc := spec.itemDescriptions[seedRand.Intn(len(spec.itemDescriptions))]
				amount := randRange(spec.itemAmountMin, spec.itemAmountMax)
				total += amount
				day := 3 + seedRand.Intn(24)
				date := time.Date(demoYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
				if _, err := s.tx.Exec(s.ctx,
					`INSERT INTO income_transactions (period_id, source_id, amount_cents, description, tx_date)
					 VALUES ($1,$2,$3,$4,$5)`,
					periodID, id, amount, desc, date,
				); err != nil {
					return fmt.Errorf("insert income_transaction %q month %d: %w", spec.name, month, err)
				}
			}
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO income_entries (period_id, source_id, amount_cents, entry_type, sort_order)
				 VALUES ($1,$2,$3,'normal',$4)`,
				periodID, id, total, order,
			); err != nil {
				return fmt.Errorf("insert income_entry (itemized) %q month %d: %w", spec.name, month, err)
			}
		}
	}

	return s.seedCarryoverIncome()
}

// seedCarryoverIncome adds carryover income entries (source_id NULL,
// source_period_id set, entry_type='carryover') in 3 months, each carrying
// forward a small leftover from the previous period.
func (s *seeder) seedCarryoverIncome() error {
	carryoverMonths := []int{2, 6, 11}
	for _, month := range carryoverMonths {
		prevMonth := month - 1
		amount := randRange(3000, 22000)
		label := fmt.Sprintf("Overschot %s", monthName(prevMonth))
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO income_entries (period_id, source_id, label, amount_cents, entry_type, source_period_id)
			 VALUES ($1,NULL,$2,$3,'carryover',$4)`,
			s.periodID[month], label, amount, s.periodID[prevMonth],
		); err != nil {
			return fmt.Errorf("insert carryover income_entry month %d: %w", month, err)
		}
	}
	return nil
}

var dutchMonthNames = []string{"", "januari", "februari", "maart", "april", "mei", "juni", "juli", "augustus", "september", "oktober", "november", "december"}

func monthName(month int) string {
	if month < 1 || month > 12 {
		return ""
	}
	return dutchMonthNames[month]
}

// randRange returns a uniformly random int64 in [min, max], drawing from the
// single shared seedRand source.
func randRange(min, max int64) int64 {
	if max <= min {
		return min
	}
	return min + seedRand.Int63n(max-min+1)
}
