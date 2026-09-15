package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool defaults sized for this app's actual deployment: a single replica with a
// 500m CPU limit, talking to the *shared* cnpg-shared cluster. pgxpool's own
// default is max(4, runtime.NumCPU()), which reads the node's core count rather
// than the pod's limit and would claim 16-32 connections from a cluster other
// apps depend on.
const (
	defaultMaxConns         = 10
	defaultMinConns         = 2
	defaultMaxConnLifetime  = 30 * time.Minute
	defaultMaxConnIdleTime  = 5 * time.Minute
	defaultLifetimeJitter   = 5 * time.Minute
	defaultStatementTimeout = "30s"
	// A transaction left idle (a handler that returns without commit/rollback)
	// holds its row locks until the client disconnects; this bounds that to the
	// same order as a statement.
	defaultIdleInTxTimeout = "60s"
)

// NewPool builds the shared connection pool. Sizing and timeouts are read from
// the environment here rather than from internal/config because they're
// infrastructure knobs for this one pool, not application settings any handler
// reads — DATABASE_URL's own pgx pool parameters still apply for anything not
// overridden below.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.ParseConfig: %w", err)
	}

	poolCfg.MaxConns = envInt32("DB_MAX_CONNS", defaultMaxConns)
	poolCfg.MinConns = envInt32("DB_MIN_CONNS", defaultMinConns)
	if poolCfg.MinConns > poolCfg.MaxConns {
		poolCfg.MinConns = poolCfg.MaxConns
	}
	poolCfg.MaxConnLifetime = defaultMaxConnLifetime
	poolCfg.MaxConnLifetimeJitter = defaultLifetimeJitter
	poolCfg.MaxConnIdleTime = defaultMaxConnIdleTime

	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	setRuntimeParam(poolCfg, "statement_timeout", envString("DB_STATEMENT_TIMEOUT", defaultStatementTimeout))
	setRuntimeParam(poolCfg, "idle_in_transaction_session_timeout", envString("DB_IDLE_IN_TX_TIMEOUT", defaultIdleInTxTimeout))

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// setRuntimeParam never overwrites a value the DSN already carries — an
// operator who put statement_timeout in DATABASE_URL meant it.
func setRuntimeParam(cfg *pgxpool.Config, key, value string) {
	if value == "" {
		return
	}
	if _, ok := cfg.ConnConfig.RuntimeParams[key]; ok {
		return
	}
	cfg.ConnConfig.RuntimeParams[key] = value
}

func envInt32(key string, def int32) int32 {
	v, err := strconv.ParseInt(os.Getenv(key), 10, 32)
	if err != nil || v <= 0 {
		return def
	}
	return int32(v)
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
