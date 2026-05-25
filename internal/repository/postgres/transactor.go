package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// Transactor implements domain.Transactor over a pgx pool.
type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

// WithinTx begins a transaction, hands fn a tx-scoped Repositories
// bundle, and commits iff fn returns nil. The deferred Rollback is a
// no-op once Commit has succeeded, so it safely covers the error and
// panic paths without double-finishing the transaction.
func (t *Transactor) WithinTx(ctx context.Context, fn func(domain.Repositories) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(repoSet{q: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// NewRepositories returns a Repositories bundle bound to the given
// Querier. Pass a *pgxpool.Pool for non-transactional reads; the
// Transactor uses the same type internally for tx-scoped work.
func NewRepositories(q Querier) domain.Repositories {
	return repoSet{q: q}
}

// repoSet builds repositories bound to a single Querier (pool or tx).
// Each accessor returns a fresh repo; they are cheap value wrappers
// around the shared querier, so all repos in one WithinTx call share
// the tx.
type repoSet struct {
	q Querier
}

func (r repoSet) Leagues() domain.LeagueRepository              { return NewLeagueRepo(r.q) }
func (r repoSet) Teams() domain.TeamRepository                  { return NewTeamRepo(r.q) }
func (r repoSet) Matches() domain.MatchRepository               { return NewMatchRepo(r.q) }
func (r repoSet) Snapshots() domain.StandingsSnapshotRepository { return NewStandingsSnapshotRepo(r.q) }
func (r repoSet) Audits() domain.MatchAuditRepository           { return NewMatchAuditRepo(r.q) }
