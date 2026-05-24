// Package postgres provides Postgres-backed implementations of the
// repository interfaces declared in internal/domain.
//
// This file owns the connection pool lifecycle. Repository types are
// added in later steps; for now we expose only the pool primitives that
// the composition root needs to wire /ready and pass to repos.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pingTimeout bounds how long a single liveness Ping is allowed to
// block. Kept small so /ready degrades quickly when the DB is gone.
const pingTimeout = 2 * time.Second

// NewPool opens a pgxpool against the supplied DSN and verifies the
// connection with a bounded Ping. Returning the *pgxpool.Pool directly
// is intentional: callers store the pool, pass it to repositories, and
// close it at shutdown.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: initial ping: %w", err)
	}

	return pool, nil
}
