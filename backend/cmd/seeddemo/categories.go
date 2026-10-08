package main

import "fmt"

// categorySpec describes one of the ~15 invented expense categories.
// "Overig" is the deliberately overstuffed catch-all and is handled
// separately in budget.go (its transactions use inconsistent spellings
// rather than this pool).
type categorySpec struct {
	name               string
	parent             string // "" if top-level
	isItemized         bool
	tracksTransactions bool
	monthlyBudget      int64 // baseline for budget_lines.amount_cents
	descriptions       []string
	txPerMonthMin      int
	txPerMonthMax      int
}

func categorySpecs() []categorySpec {
	return []categorySpec{
		{name: "Boodschappen", tracksTransactions: true, monthlyBudget: 55000,
			descriptions:  []string{"Albert Heijn Boodschappen", "Jumbo", "Lidl", "Plus supermarkt", "Dirk van den Broek"},
			txPerMonthMin: 5, txPerMonthMax: 10},
		{name: "Boodschappen Online", parent: "Boodschappen", tracksTransactions: true, monthlyBudget: 15000,
			descriptions:  []string{"Picnic", "Albert Heijn bezorgd", "Gorillas", "Flink"},
			txPerMonthMin: 2, txPerMonthMax: 6},
		{name: "Woonlasten", monthlyBudget: 140000,
			descriptions:  []string{"Huur", "Hypotheek", "Waterschapsbelasting", "Gemeentelijke belastingen"},
			txPerMonthMin: 1, txPerMonthMax: 3},
		{name: "Energie", monthlyBudget: 18000,
			descriptions:  []string{"Energieleverancier voorschot", "Gasrekening", "Stroomrekening"},
			txPerMonthMin: 1, txPerMonthMax: 3},
		{name: "Verzekeringen", monthlyBudget: 22000,
			descriptions:  []string{"Zorgverzekering", "Inboedelverzekering", "Autoverzekering", "Reisverzekering"},
			txPerMonthMin: 1, txPerMonthMax: 3},
		{name: "Abonnementen", isItemized: true, tracksTransactions: true, monthlyBudget: 9000,
			descriptions:  []string{"Netflix", "Spotify", "Disney+", "Videoland", "Krant abonnement"},
			txPerMonthMin: 3, txPerMonthMax: 7},
		{name: "Verzorging", isItemized: true, tracksTransactions: true, monthlyBudget: 12000,
			descriptions:  []string{"Kapper", "Drogist", "Tandarts", "Fysiotherapeut"},
			txPerMonthMin: 2, txPerMonthMax: 6},
		{name: "Kleding", monthlyBudget: 8000,
			descriptions:  []string{"H&M", "Zara", "Zalando", "Bristol"},
			txPerMonthMin: 1, txPerMonthMax: 4},
		{name: "Uitgaan", tracksTransactions: true, monthlyBudget: 15000,
			descriptions:  []string{"Restaurant", "Café", "Bioscoop", "Pretpark"},
			txPerMonthMin: 4, txPerMonthMax: 9},
		{name: "Vervoer", tracksTransactions: true, monthlyBudget: 20000,
			descriptions:  []string{"Tanken", "NS kaartje", "Parkeren", "APK keuring", "Wasstraat"},
			txPerMonthMin: 5, txPerMonthMax: 10},
		{name: "Vakantie", monthlyBudget: 25000,
			descriptions:  []string{"Vliegtickets", "Hotel", "Vakantiehuisje", "Reisverzekering extra"},
			txPerMonthMin: 0, txPerMonthMax: 3},
		{name: "Cadeaus", tracksTransactions: true, monthlyBudget: 6000,
			descriptions:  []string{"Verjaardagscadeau", "Sinterklaascadeau", "Kerstcadeau", "Bloemen"},
			txPerMonthMin: 1, txPerMonthMax: 4},
		{name: "Huishouden", tracksTransactions: true, monthlyBudget: 9000,
			descriptions:  []string{"Gamma", "Action huishouden", "IKEA", "Blokker"},
			txPerMonthMin: 2, txPerMonthMax: 6},
		{name: "Onderhoud huis", monthlyBudget: 10000,
			descriptions:  []string{"Loodgieter", "Schilder", "Tuinman", "Dakreparatie"},
			txPerMonthMin: 0, txPerMonthMax: 3},
		{name: "Overig", tracksTransactions: true, monthlyBudget: 40000,
			txPerMonthMin: 0, txPerMonthMax: 0}, // Overig's transaction volume is driven separately, see budget.go
	}
}

// seedCategories inserts all categories (parent before child, so parent_id
// is known) plus a couple of category_aliases rows nesting an import-style
// header under the "Boodschappen" parent.
func (s *seeder) seedCategories() error {
	s.categoryID = make(map[string]int64)
	s.catSpecs = categorySpecs()

	// Two passes: top-level categories first, then children, so parent_id is
	// always available by the time a child is inserted.
	order := 0
	for _, spec := range s.catSpecs {
		if spec.parent != "" {
			continue
		}
		id, err := s.scanID(
			`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order)
			 VALUES ($1,$2,$3,$4) RETURNING id`,
			spec.name, spec.monthlyBudget, spec.isItemized, order,
		)
		if err != nil {
			return fmt.Errorf("insert category %q: %w", spec.name, err)
		}
		s.categoryID[spec.name] = id
		order++
	}
	for _, spec := range s.catSpecs {
		if spec.parent == "" {
			continue
		}
		parentID, ok := s.categoryID[spec.parent]
		if !ok {
			return fmt.Errorf("category %q references unknown parent %q", spec.name, spec.parent)
		}
		id, err := s.scanID(
			`INSERT INTO categories (name, default_amount_cents, is_itemized, sort_order, parent_id)
			 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			spec.name, spec.monthlyBudget, spec.isItemized, order, parentID,
		)
		if err != nil {
			return fmt.Errorf("insert category %q: %w", spec.name, err)
		}
		s.categoryID[spec.name] = id
		order++
	}

	aliases := []struct {
		alias  string
		parent string
	}{
		{"Bunq", "Boodschappen"},
		{"AH Bezorgservice", "Boodschappen"},
	}
	for _, a := range aliases {
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO category_aliases (alias_name, parent_category_id) VALUES ($1,$2)`,
			a.alias, s.categoryID[a.parent],
		); err != nil {
			return fmt.Errorf("insert category_alias %q: %w", a.alias, err)
		}
	}
	return nil
}
