// Package service holds the application's business logic. Services
// orchestrate repositories (via the domain.Transactor seam) and the
// pure algorithm packages; they own validation, transactions, and the
// rules that span multiple repositories.
package service

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// LeagueService coordinates league creation, reads, and (in later
// commits) the play/reset state machine.
type LeagueService struct {
	repos     domain.Repositories     // pool-backed, for reads
	tx        domain.Transactor       // for atomic multi-write operations
	fixtures  domain.FixtureGenerator
	standings domain.StandingsCalculator
}

func NewLeagueService(
	repos domain.Repositories,
	tx domain.Transactor,
	fixtures domain.FixtureGenerator,
	standings domain.StandingsCalculator,
) *LeagueService {
	return &LeagueService{repos: repos, tx: tx, fixtures: fixtures, standings: standings}
}

// CreateLeagueInput is the service-level request for CreateLeague.
// It is distinct from any HTTP DTO so the service has no transport
// dependency.
type CreateLeagueInput struct {
	Name    string
	TeamIDs []int64 // empty → use the whole team catalog
	Seed    *int64  // nil → a random seed is generated and stored
}

// CreateLeague validates the team set, generates the fixture schedule,
// and persists the league plus its fixtures atomically.
//
// With no TeamIDs the entire team catalog is used (the demo's seeded
// four teams). The team count must be at least 4 and even, matching the
// double round-robin requirement.
func (s *LeagueService) CreateLeague(ctx context.Context, in CreateLeagueInput) (*domain.League, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}

	teamIDs, err := s.resolveTeamIDs(ctx, in.TeamIDs)
	if err != nil {
		return nil, err
	}
	if n := len(teamIDs); n < 4 || n%2 != 0 {
		return nil, fmt.Errorf("%w: need an even number of teams >= 4, got %d", domain.ErrInvalidInput, n)
	}

	seed := int64(rand.Uint64())
	if in.Seed != nil {
		seed = *in.Seed
	}

	league := &domain.League{
		Name:        in.Name,
		CurrentWeek: 0,
		TotalWeeks:  2 * (len(teamIDs) - 1),
		Status:      domain.LeagueStatusNotStarted,
		RandomSeed:  seed,
	}

	err = s.tx.WithinTx(ctx, func(r domain.Repositories) error {
		if err := r.Leagues().Create(ctx, league, teamIDs); err != nil {
			return fmt.Errorf("create league: %w", err)
		}
		fixtures := s.fixtures.Generate(teamIDs, seed)
		for i := range fixtures {
			fixtures[i].LeagueID = league.ID
		}
		if err := r.Matches().BulkCreate(ctx, fixtures); err != nil {
			return fmt.Errorf("create fixtures: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return league, nil
}

// resolveTeamIDs returns the explicit IDs when provided, otherwise the
// whole catalog's IDs.
func (s *LeagueService) resolveTeamIDs(ctx context.Context, explicit []int64) ([]int64, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}
	teams, err := s.repos.Teams().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	ids := make([]int64, len(teams))
	for i, t := range teams {
		ids[i] = t.ID
	}
	return ids, nil
}

// GetLeague returns a league or domain.ErrNotFound.
func (s *LeagueService) GetLeague(ctx context.Context, id int64) (*domain.League, error) {
	return s.repos.Leagues().GetByID(ctx, id)
}

// ListLeagues returns all leagues, newest first.
func (s *LeagueService) ListLeagues(ctx context.Context) ([]domain.League, error) {
	return s.repos.Leagues().List(ctx)
}

// Standings computes the current table live from played matches. It does
// not read snapshots, so it is always correct even between snapshot
// writes. Returns domain.ErrNotFound when the league does not exist.
func (s *LeagueService) Standings(ctx context.Context, leagueID int64) ([]domain.StandingRow, error) {
	if _, err := s.repos.Leagues().GetByID(ctx, leagueID); err != nil {
		return nil, err
	}
	teams, err := s.repos.Teams().ListByLeague(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	matches, err := s.repos.Matches().ListByLeague(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}
	return s.standings.Calculate(teams, matches), nil
}

// Fixtures returns every match in the league, ordered by week then id.
// Grouping into weeks is left to the transport layer.
func (s *LeagueService) Fixtures(ctx context.Context, leagueID int64) ([]domain.Match, error) {
	if _, err := s.repos.Leagues().GetByID(ctx, leagueID); err != nil {
		return nil, err
	}
	return s.repos.Matches().ListByLeague(ctx, leagueID)
}

// WeekDetail bundles one week's matches with the standings snapshot
// captured at the end of that week. The snapshot may be empty if the
// week has not been played yet.
type WeekDetail struct {
	Week      int
	Matches   []domain.Match
	Standings []domain.StandingRow
}

// WeekDetail returns the matches and historical table for a single week.
func (s *LeagueService) WeekDetail(ctx context.Context, leagueID int64, week int) (*WeekDetail, error) {
	if _, err := s.repos.Leagues().GetByID(ctx, leagueID); err != nil {
		return nil, err
	}
	matches, err := s.repos.Matches().ListByLeagueAndWeek(ctx, leagueID, week)
	if err != nil {
		return nil, fmt.Errorf("list week matches: %w", err)
	}
	snapshot, err := s.repos.Snapshots().GetByWeek(ctx, leagueID, week)
	if err != nil {
		return nil, fmt.Errorf("get week snapshot: %w", err)
	}
	return &WeekDetail{Week: week, Matches: matches, Standings: snapshot}, nil
}
