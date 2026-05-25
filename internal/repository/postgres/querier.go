package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the minimal contract every repository depends on. Both
// *pgxpool.Pool and pgx.Tx satisfy it, so a repository constructed
// with a pool runs each method on its own connection, while one
// constructed with a tx participates in the caller's transaction.
//
// Services use this seam to compose multi-write operations atomically:
//
//   tx, _ := pool.Begin(ctx)
//   defer tx.Rollback(ctx)
//   NewLeagueRepo(tx).Create(ctx, league, teamIDs)
//   NewMatchRepo(tx).BulkCreate(ctx, fixtures)
//   tx.Commit(ctx)
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
