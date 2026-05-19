// Package postgres holds the Postgres-specific *infrastructure*
// concerns: pool construction and migration execution. Repository
// implementations live in `adapter/persistence/postgres/`.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig groups the tunables exposed by NewPool. Defaults match
// the architecture document.
type PoolConfig struct {
	DSN              string
	MaxConns         int32
	MinConns         int32
	MaxConnLifetime  time.Duration
	MaxConnIdleTime  time.Duration
	HealthCheckEvery time.Duration
	ConnectTimeout   time.Duration
}

// NewPool constructs the pgx pool and verifies it can ping Postgres
// before returning. Boot fails loudly on a bad DSN.
func NewPool(ctx context.Context, in PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(in.DSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse dsn: %w", err)
	}
	cfg.MaxConns = orInt32(in.MaxConns, 25)
	cfg.MinConns = orInt32(in.MinConns, 2)
	cfg.MaxConnLifetime = orDuration(in.MaxConnLifetime, 30*time.Minute)
	cfg.MaxConnIdleTime = orDuration(in.MaxConnIdleTime, 5*time.Minute)
	cfg.HealthCheckPeriod = orDuration(in.HealthCheckEvery, time.Minute)
	cfg.ConnConfig.ConnectTimeout = orDuration(in.ConnectTimeout, 5*time.Second)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

func orInt32(v, def int32) int32 {
	if v <= 0 {
		return def
	}
	return v
}

func orDuration(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}
