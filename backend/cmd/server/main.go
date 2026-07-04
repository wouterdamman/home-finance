package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	migrations "github.com/TheIronRock95/home-finance/migrations"
	"github.com/TheIronRock95/home-finance/internal/auth"
	"github.com/TheIronRock95/home-finance/internal/config"
	"github.com/TheIronRock95/home-finance/internal/httpapi"
	"github.com/TheIronRock95/home-finance/internal/store"
)

func main() {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	logLevel := slog.LevelInfo
	if cfg.Env == "development" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate || *migrateOnly {
		goose.SetBaseFS(migrations.FS)
		if err := goose.SetDialect("postgres"); err != nil {
			slog.Error("goose dialect", "err", err)
			os.Exit(1)
		}
		sqlDB := stdlib.OpenDBFromPool(pool)
		if err := goose.Up(sqlDB, "."); err != nil {
			slog.Error("goose up", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations ok")
	}

	if *migrateOnly {
		return
	}

	sm := auth.NewSessionManager(pool, cfg.Env != "development")

	var oidcProvider *auth.Provider
	if !cfg.DevFakeAuth && cfg.OIDCIssuerURL != "" {
		oidcProvider, err = auth.NewProvider(ctx, cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL)
		if err != nil {
			slog.Error("oidc init", "err", err)
			os.Exit(1)
		}
	}

	handler := httpapi.NewServer(cfg, pool, sm, oidcProvider)

	addr := ":" + cfg.Port
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
