package importer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheIronRock95/home-finance/internal/domain"
)

type ImportOptions struct {
	Year         int
	Wipe         bool
	CloseThrough int // month number, inclusive; 0 = don't close any
}

type Report struct {
	Months []MonthReport
}

type MonthReport struct {
	Month             int
	IncomeTotalCents  int64
	ExpenseTotalCents int64
	SurplusCents      int64
	Closed            bool
}

func Run(ctx context.Context, pool *pgxpool.Pool, months []MonthData, opts ImportOptions) (*Report, error) {
	if opts.Wipe {
		if err := wipe(ctx, pool, opts.Year); err != nil {
			return nil, fmt.Errorf("wipe: %w", err)
		}
	}

	catIDs := map[string]int64{}
	srcIDs := map[string]int64{}
	potIDs := map[string]int64{}

	if err := loadMasterdata(ctx, pool, catIDs, srcIDs, potIDs); err != nil {
		return nil, fmt.Errorf("load masterdata: %w", err)
	}

	overviews := map[int]*MonthData{}
	details := map[int][]TxRow{}
	for i := range months {
		md := &months[i]
		if md.Year != opts.Year && opts.Year != 0 {
			continue
		}
		if md.Sheet == "Overview" {
			overviews[md.Month] = md
		} else {
			details[md.Month] = append(details[md.Month], md.Txs...)
		}
	}

	var rep Report
	for monthNum := 1; monthNum <= 12; monthNum++ {
		ov, ok := overviews[monthNum]
		if !ok {
			continue
		}

		for _, inc := range ov.Incomes {
			if _, exists := srcIDs[inc.Label]; !exists {
				var id int64
				pool.QueryRow(ctx, `INSERT INTO income_sources (name,default_amount_cents,sort_order) VALUES ($1,$2,$3) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`,
					inc.Label, inc.AmountCents, len(srcIDs)).Scan(&id)
				srcIDs[inc.Label] = id
			}
		}
		for _, bl := range ov.Lines {
			if _, exists := catIDs[bl.Label]; !exists {
				var id int64
				pool.QueryRow(ctx, `INSERT INTO categories (name,default_amount_cents,is_itemized,sort_order) VALUES ($1,$2,false,$3) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`,
					bl.Label, bl.AmountCents, len(catIDs)).Scan(&id)
				catIDs[bl.Label] = id
			}
		}
		for _, tx := range details[monthNum] {
			if _, exists := catIDs[tx.CategoryLabel]; !exists {
				var id int64
				pool.QueryRow(ctx, `INSERT INTO categories (name,default_amount_cents,is_itemized,sort_order) VALUES ($1,0,true,$2) ON CONFLICT (name) DO UPDATE SET is_itemized=true RETURNING id`,
					tx.CategoryLabel, len(catIDs)).Scan(&id)
				catIDs[tx.CategoryLabel] = id
			}
		}

		var periodID int64
		pool.QueryRow(ctx, `INSERT INTO periods (year,month) VALUES ($1,$2) ON CONFLICT (year,month) DO UPDATE SET year=EXCLUDED.year RETURNING id`,
			opts.Year, monthNum).Scan(&periodID)

		for i, inc := range ov.Incomes {
			srcID := srcIDs[inc.Label]
			pool.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,notes,sort_order) VALUES ($1,$2,$3,$4,'normal','',$5)`,
				periodID, srcID, inc.Label, inc.AmountCents, i)
		}

		for i, bl := range ov.Lines {
			catID := catIDs[bl.Label]
			tracksTransactions := false
			if _, hasDet := details[monthNum]; hasDet {
				for _, tx := range details[monthNum] {
					if tx.CategoryLabel == bl.Label {
						tracksTransactions = true
						break
					}
				}
			}
			pool.Exec(ctx, `INSERT INTO budget_lines (period_id,category_id,label,amount_cents,tracks_transactions,sort_order) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (period_id,category_id) DO NOTHING`,
				periodID, catID, bl.Label, bl.AmountCents, tracksTransactions, i)
		}

		for _, tx := range details[monthNum] {
			catID := catIDs[tx.CategoryLabel]
			pool.Exec(ctx, `INSERT INTO transactions (period_id,category_id,amount_cents,description,tx_date) VALUES ($1,$2,$3,$4,$5)`,
				periodID, catID, tx.AmountCents, tx.Description, time.Date(opts.Year, time.Month(monthNum), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
		}

		var incTotal, expTotal int64
		pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM income_entries WHERE period_id=$1 AND entry_type='normal'`, periodID).Scan(&incTotal)
		pool.QueryRow(ctx, `SELECT COALESCE(SUM(CASE WHEN bl.tracks_transactions THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0) ELSE bl.amount_cents END),0) FROM budget_lines bl WHERE bl.period_id=$1`, periodID).Scan(&expTotal)

		mr := MonthReport{Month: monthNum, IncomeTotalCents: incTotal, ExpenseTotalCents: expTotal, SurplusCents: incTotal - expTotal}

		if opts.CloseThrough >= monthNum {
			if err := closePeriod(ctx, pool, periodID, potIDs, opts.Year, monthNum); err != nil {
				slog.Warn("close period", "month", monthNum, "err", err)
			} else {
				mr.Closed = true
			}
		}

		rep.Months = append(rep.Months, mr)
	}

	return &rep, nil
}

func closePeriod(ctx context.Context, pool *pgxpool.Pool, periodID int64, potIDs map[string]int64, year, month int) error {
	var splitRows []struct {
		PotID int64
		Pct   float64
		Kind  string
	}
	rows, _ := pool.Query(ctx, `SELECT ps.pot_id, ps.percentage, p.kind FROM pot_splits ps JOIN pots p ON p.id=ps.pot_id WHERE ps.period_id=$1`, periodID)
	for rows.Next() {
		var sr struct {
			PotID int64
			Pct   float64
			Kind  string
		}
		rows.Scan(&sr.PotID, &sr.Pct, &sr.Kind)
		splitRows = append(splitRows, sr)
	}
	rows.Close()

	if len(splitRows) == 0 {
		slog.Info("no splits configured for period, skipping close", "period_id", periodID)
		return nil
	}

	inputs := make([]domain.PotSplitInput, len(splitRows))
	for i, sr := range splitRows {
		inputs[i] = domain.PotSplitInput{PotID: sr.PotID, Percentage: sr.Pct}
	}

	var incTotal, expTotal int64
	pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM income_entries WHERE period_id=$1`, periodID).Scan(&incTotal)
	pool.QueryRow(ctx, `SELECT COALESCE(SUM(CASE WHEN bl.tracks_transactions THEN COALESCE((SELECT SUM(t.amount_cents) FROM transactions t WHERE t.period_id=bl.period_id AND t.category_id=bl.category_id),0) ELSE bl.amount_cents END),0) FROM budget_lines bl WHERE bl.period_id=$1`, periodID).Scan(&expTotal)
	surplus := incTotal - expTotal

	allocs := domain.LargestRemainderSplit(surplus, inputs)
	today := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, a := range allocs {
		tx.Exec(ctx, `INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$2,'allocation',$3,'Monthly allocation',$4)`,
			a.PotID, periodID, a.AmountCents, today)

		if splitRows[i].Kind == "carryover" {
			nextMonth := month + 1
			nextYear := year
			if nextMonth > 12 {
				nextMonth = 1
				nextYear++
			}
			var nextPeriodID int64
			tx.QueryRow(ctx, `INSERT INTO periods (year,month) VALUES ($1,$2) ON CONFLICT (year,month) DO UPDATE SET year=EXCLUDED.year RETURNING id`, nextYear, nextMonth).Scan(&nextPeriodID)
			tx.Exec(ctx, `INSERT INTO pot_ledger (pot_id,period_id,source_period_id,entry_type,amount_cents,description,entry_date) VALUES ($1,$2,$3,'carryover_out',$4,'Carryover out',$5)`,
				a.PotID, nextPeriodID, periodID, -a.AmountCents, today)
			monthNames := []string{"", "Januari", "Februari", "Maart", "April", "Mei", "Juni", "Juli", "Augustus", "September", "Oktober", "November", "December"}
			name := monthNames[month]
			tx.Exec(ctx, `INSERT INTO income_entries (period_id,source_id,label,amount_cents,entry_type,source_period_id,notes,sort_order) VALUES ($1,NULL,$2,$3,'carryover',$4,'',0)`,
				nextPeriodID, "Doorlopen maand "+name, a.AmountCents, periodID)
		}
	}

	tx.Exec(ctx, `UPDATE periods SET status='closed',closed_at=now() WHERE id=$1`, periodID)
	return tx.Commit(ctx)
}

func wipe(ctx context.Context, pool *pgxpool.Pool, year int) error {
	_, err := pool.Exec(ctx, `
		DELETE FROM pot_ledger WHERE period_id IN (SELECT id FROM periods WHERE year=$1);
		DELETE FROM pot_splits WHERE period_id IN (SELECT id FROM periods WHERE year=$1);
		DELETE FROM transactions WHERE period_id IN (SELECT id FROM periods WHERE year=$1);
		DELETE FROM budget_lines WHERE period_id IN (SELECT id FROM periods WHERE year=$1);
		DELETE FROM income_entries WHERE period_id IN (SELECT id FROM periods WHERE year=$1);
		DELETE FROM periods WHERE year=$1;
	`, year)
	return err
}

func loadMasterdata(ctx context.Context, pool *pgxpool.Pool, catIDs, srcIDs, potIDs map[string]int64) error {
	rows, err := pool.Query(ctx, `SELECT id,name FROM categories`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		rows.Scan(&id, &name)
		catIDs[strings.TrimSpace(name)] = id
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT id,name FROM income_sources`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		rows.Scan(&id, &name)
		srcIDs[strings.TrimSpace(name)] = id
	}
	rows.Close()

	rows, err = pool.Query(ctx, `SELECT id,name FROM pots`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var name string
		rows.Scan(&id, &name)
		potIDs[strings.TrimSpace(name)] = id
	}
	rows.Close()
	return nil
}
