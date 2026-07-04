package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheIronRock95/home-finance/internal/importer"
)

func main() {
	xlsx := flag.String("xlsx", "", "path to Excel file")
	year := flag.Int("year", 0, "year to import (e.g. 2026)")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "postgres DSN")
	wipe := flag.Bool("wipe", false, "delete existing data for year before import")
	closeThrough := flag.Int("close-through", 0, "close periods through this month number")
	flag.Parse()

	if *xlsx == "" || *year == 0 {
		fmt.Fprintln(os.Stderr, "usage: importer --xlsx <path> --year <year> [--dsn <dsn>] [--wipe] [--close-through <month>]")
		os.Exit(1)
	}
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or --dsn required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping", "err", err)
		os.Exit(1)
	}

	months, err := importer.ParseXLSX(*xlsx)
	if err != nil {
		slog.Error("parse", "err", err)
		os.Exit(1)
	}
	slog.Info("parsed", "sheets", len(months))

	report, err := importer.Run(ctx, pool, months, importer.ImportOptions{
		Year:         *year,
		Wipe:         *wipe,
		CloseThrough: *closeThrough,
	})
	if err != nil {
		slog.Error("import", "err", err)
		os.Exit(1)
	}

	fmt.Printf("\nImport report for %d:\n", *year)
	fmt.Printf("%-8s %14s %14s %14s %8s\n", "Month", "Income", "Expense", "Surplus", "Status")
	fmt.Printf("%s\n", "────────────────────────────────────────────────────────────────")
	for _, mr := range report.Months {
		status := "open"
		if mr.Closed {
			status = "closed"
		}
		fmt.Printf("%-8d %14.2f %14.2f %14.2f %8s\n",
			mr.Month,
			float64(mr.IncomeTotalCents)/100,
			float64(mr.ExpenseTotalCents)/100,
			float64(mr.SurplusCents)/100,
			status,
		)
	}
}
