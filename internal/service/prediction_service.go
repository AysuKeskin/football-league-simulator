package service

import (
	"context"
	"fmt"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

const (
	// defaultSimulations is used when the caller does not specify a count.
	defaultSimulations = 10000
	// maxSimulations bounds latency; requests above it are clamped.
	maxSimulations = 100000
	// predictionsFromWeek is the earliest week predictions are offered,
	// matching the PDF which shows the championship panel from week 4.
	predictionsFromWeek = 4
)

// PredictionService answers championship-prediction queries. It is
// read-only: it loads league data and either runs the Monte Carlo engine
// (in progress) or returns the actual table (finished). No transactor.
type PredictionService struct {
	repos     domain.Repositories
	engine    domain.PredictionEngine
	standings domain.StandingsCalculator
}

func NewPredictionService(repos domain.Repositories, engine domain.PredictionEngine, standings domain.StandingsCalculator) *PredictionService {
	return &PredictionService{repos: repos, engine: engine, standings: standings}
}

// PredictionResult is either an in-progress Monte Carlo forecast
// (Predictions populated) or, once the league is FINISHED, the real
// final table (FinalStandings + Champion populated, Finished true).
type PredictionResult struct {
	Week           int
	Simulations    int
	Finished       bool
	Champion       string
	Predictions    []domain.Prediction
	FinalStandings []domain.StandingRow
}

// Predict returns championship predictions for a league.
//
// Returns ErrNotFound for an unknown league and ErrConflict before week
// predictionsFromWeek. Once the league is finished it returns the actual
// final table instead of a forecast.
func (s *PredictionService) Predict(ctx context.Context, leagueID int64, simulations int) (*PredictionResult, error) {
	league, err := s.repos.Leagues().GetByID(ctx, leagueID)
	if err != nil {
		return nil, err
	}
	if league.CurrentWeek < predictionsFromWeek {
		return nil, fmt.Errorf("%w: predictions available from week %d (currently week %d)",
			domain.ErrConflict, predictionsFromWeek, league.CurrentWeek)
	}

	teams, err := s.repos.Teams().ListByLeague(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	matches, err := s.repos.Matches().ListByLeague(ctx, leagueID)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}

	if league.Status == domain.LeagueStatusFinished {
		table := s.standings.Calculate(teams, matches)
		var champion string
		if len(table) > 0 {
			champion = table[0].TeamName
		}
		return &PredictionResult{
			Week:           league.CurrentWeek,
			Finished:       true,
			Champion:       champion,
			FinalStandings: table,
		}, nil
	}

	sims := clampSimulations(simulations)
	return &PredictionResult{
		Week:        league.CurrentWeek,
		Simulations: sims,
		Predictions: s.engine.Predict(teams, matches, sims, league.RandomSeed),
	}, nil
}

// clampSimulations applies the default for non-positive input and caps
// at the maximum to bound request latency.
func clampSimulations(n int) int {
	switch {
	case n <= 0:
		return defaultSimulations
	case n > maxSimulations:
		return maxSimulations
	default:
		return n
	}
}
