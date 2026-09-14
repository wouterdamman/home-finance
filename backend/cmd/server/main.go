package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/wouterdamman/home-finance/internal/auditexport"
	"github.com/wouterdamman/home-finance/internal/auth"
	"github.com/wouterdamman/home-finance/internal/config"
	"github.com/wouterdamman/home-finance/internal/httpapi"
	"github.com/wouterdamman/home-finance/internal/store"
	migrations "github.com/wouterdamman/home-finance/migrations"
)

// HTTP timeouts. Without them a connection that stalls mid-headers holds a
// goroutine and an fd for the life of the process. writeTimeout is the loose
// one on purpose: the xlsx export and import handlers build or parse a whole
// workbook inside the request, which is the slowest legitimate work the server
// does — everything else answers in milliseconds.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 120 * time.Second
	idleTimeout       = 120 * time.Second
	// Shorter than the k8s default 30s termination grace period, so in-flight
	// requests (notably a period close mid-transaction) finish before SIGKILL.
	shutdownTimeout = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	migrateOnly := flag.Bool("migrate-only", false, "run migrations and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logLevel := slog.LevelInfo
	if cfg.Env == "development" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// Cancelled on SIGINT/SIGTERM; both the HTTP shutdown and the background
	// audit exporter hang off it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.AutoMigrate || *migrateOnly {
		goose.SetBaseFS(migrations.FS)
		if err := goose.SetDialect("postgres"); err != nil {
			return err
		}
		sqlDB := stdlib.OpenDBFromPool(pool)
		if err := goose.Up(sqlDB, "."); err != nil {
			return err
		}
		slog.Info("migrations ok")
	}

	if *migrateOnly {
		return nil
	}

	if err := cfg.ValidateOIDC(); err != nil {
		return err
	}

	sm := auth.NewSessionManager(pool, cfg.SessionSecure)

	var oidcProvider *auth.Provider
	if !cfg.DevFakeAuth && cfg.OIDCIssuerURL != "" {
		oidcProvider, err = auth.NewProvider(ctx, cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL)
		if err != nil {
			return err
		}
	}

	handler := httpapi.NewServer(cfg, pool, sm, oidcProvider)

	go auditexport.Start(ctx, pool, cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutdown signal received, draining")
	// Detached from ctx, which is already cancelled by the signal.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed, closing connections", "err", err)
		srv.Close()
	}
	slog.Info("shutdown complete")
	return nil
}
