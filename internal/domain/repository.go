package domain

import (
	"context"
	"math/rand/v2"
	"time"
)

// ------------------------------------------------------------------
// Algorithm interfaces
//
// These are the pluggable strategies that the simulation pipeline
// composes. Each is implemented in a dedicated package (fixture,
// simulation, standings, prediction) and wired at the composition root.
// ------------------------------------------------------------------

// FixtureGenerator builds a complete double round-robin schedule for
// the supplied team IDs. The returned matches are in chronological
// order grouped by week; goals are nil (Scheduled).
type FixtureGenerator interface {
	Generate(teamIDs []int64, seed int64) []Match
}

// MatchSimulator produces a score for a single fixture given the two
// teams' ratings and a seeded RNG. The simulator is stateless; all
// randomness flows through the supplied *rand.Rand to keep results
// reproducible.
type MatchSimulator interface {
	Simulate(home, away Team, rng *rand.Rand) (homeGoals, awayGoals int)
}

// StandingsCalculator derives the league table from a set of teams and
// matches. The function is pure: identical inputs always yield identical
// output, including tie-break ordering.
type StandingsCalculator interface {
	Calculate(teams []Team, matches []Match) []StandingRow
}

// PredictionEngine runs Monte Carlo simulations of the remaining
// fixtures and aggregates each team's outcomes.
//
// It is pure: the same teams, matches, simulation count, and seed yield
// identical predictions. The caller (a service) loads the data and
// bounds the simulation count. `matches` is the whole fixture list —
// already-PLAYED results are fixed, SCHEDULED ones are simulated.
type PredictionEngine interface {
	Predict(teams []Team, matches []Match, simulations int, seed int64) []Prediction
}

// ------------------------------------------------------------------
// Repository interfaces
//
// Services consume these interfaces; the Postgres package provides the
// concrete implementations. Methods accept context for cancellation and
// return sentinel errors from errors.go for well-known failure modes.
// ------------------------------------------------------------------

// LeagueRepository persists League aggregates.
type LeagueRepository interface {
	Create(ctx context.Context, league *League, teamIDs []int64) error
	GetByID(ctx context.Context, id int64) (*League, error)
	// GetByIDForUpdate reads the league row with a row-level write lock
	// (SELECT ... FOR UPDATE). It only locks when run inside a
	// transaction; services call it at the start of a state mutation to
	// serialize concurrent play-week / play-all / reset on the same league.
	GetByIDForUpdate(ctx context.Context, id int64) (*League, error)
	List(ctx context.Context) ([]League, error)
	UpdateStatusAndWeek(ctx context.Context, id int64, status LeagueStatus, currentWeek int) error
	Delete(ctx context.Context, id int64) error
}

// TeamRepository persists Team aggregates and league membership lookups.
type TeamRepository interface {
	Create(ctx context.Context, team *Team) error
	GetByID(ctx context.Context, id int64) (*Team, error)
	// List returns the whole team catalog ordered by id. Used to resolve
	// the default team set when a league is created without explicit teams.
	List(ctx context.Context) ([]Team, error)
	ListByLeague(ctx context.Context, leagueID int64) ([]Team, error)
	UpdateRating(ctx context.Context, id int64, rating Rating) error
}

// MatchRepository persists fixtures and results.
type MatchRepository interface {
	BulkCreate(ctx context.Context, matches []Match) error
	GetByID(ctx context.Context, id int64) (*Match, error)
	ListByLeague(ctx context.Context, leagueID int64) ([]Match, error)
	ListByLeagueAndWeek(ctx context.Context, leagueID int64, week int) ([]Match, error)
	UpdateResult(ctx context.Context, id int64, homeGoals, awayGoals int) error
	// ClearResults returns every match in a league to SCHEDULED with no
	// goals — used by Reset to un-play the league while keeping fixtures.
	ClearResults(ctx context.Context, leagueID int64) error
}

// StandingsSnapshotRepository persists week-by-week cached tables so
// they can be served without re-aggregating every request.
type StandingsSnapshotRepository interface {
	Upsert(ctx context.Context, leagueID int64, week int, rows []StandingRow) error
	GetByWeek(ctx context.Context, leagueID int64, week int) ([]StandingRow, error)
	ListAll(ctx context.Context, leagueID int64) (map[int][]StandingRow, error)
	DeleteFromWeek(ctx context.Context, leagueID int64, fromWeek int) error
}

// MatchAudit captures the before/after of a single match-result edit.
// It maps directly to match_audit_logs, which records changed_at rather
// than the created/updated pair, so it does not embed BaseModel.
type MatchAudit struct {
	ID           int64
	MatchID      int64
	OldHomeGoals int
	OldAwayGoals int
	NewHomeGoals int
	NewAwayGoals int
	Reason       string
	ChangedAt    time.Time
}

// MatchAuditRepository persists audit log entries for match edits.
type MatchAuditRepository interface {
	Create(ctx context.Context, audit *MatchAudit) error
	ListByMatch(ctx context.Context, matchID int64) ([]MatchAudit, error)
}

// ------------------------------------------------------------------
// Transaction abstraction
//
// Services orchestrate multi-write operations atomically without
// importing the concrete database package. Transactor.WithinTx runs the
// supplied function inside one transaction and hands it a Repositories
// bundle whose every repo is bound to that transaction; the function's
// returned error decides commit vs rollback.
// ------------------------------------------------------------------

// Repositories bundles transaction-scoped repository accessors.
type Repositories interface {
	Leagues() LeagueRepository
	Teams() TeamRepository
	Matches() MatchRepository
	Snapshots() StandingsSnapshotRepository
	Audits() MatchAuditRepository
}

// Transactor runs a unit of work inside a single transaction.
//
// If fn returns nil the transaction commits; if it returns an error the
// transaction rolls back and that error is propagated. Read-only work
// may also use WithinTx — a transaction with no writes commits cheaply.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(Repositories) error) error
}
