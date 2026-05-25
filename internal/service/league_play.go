package service

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// PlayResult is what the play-week / play-all endpoints return: the
// weeks simulated in this call, the resulting table, and the league's
// new state.
type PlayResult struct {
	PlayedWeeks []WeekMatches
	Standings   []domain.StandingRow
	CurrentWeek int
	Status      domain.LeagueStatus
}

// WeekMatches groups one week's now-played matches.
type WeekMatches struct {
	Week    int
	Matches []domain.Match
}

// PlayWeek simulates the next unplayed week and persists its snapshot.
//
// The league row is locked FOR UPDATE for the duration of the
// transaction, so concurrent play-week / play-all / reset calls on the
// same league serialize rather than racing on current_week.
func (s *LeagueService) PlayWeek(ctx context.Context, leagueID int64) (*PlayResult, error) {
	var result *PlayResult
	err := s.tx.WithinTx(ctx, func(r domain.Repositories) error {
		league, err := r.Leagues().GetByIDForUpdate(ctx, leagueID)
		if err != nil {
			return err
		}
		if league.Status == domain.LeagueStatusFinished {
			return fmt.Errorf("%w: league already finished", domain.ErrConflict)
		}

		week := league.CurrentWeek + 1
		played, err := s.simulateWeek(ctx, r, league, week)
		if err != nil {
			return err
		}

		standings, err := s.advanceAndSnapshot(ctx, r, league, week)
		if err != nil {
			return err
		}
		result = &PlayResult{
			PlayedWeeks: []WeekMatches{{Week: week, Matches: played}},
			Standings:   standings,
			CurrentWeek: week,
			Status:      statusForWeek(week, league.TotalWeeks),
		}
		return nil
	})
	return result, err
}

// PlayAll simulates every remaining week in one transaction and leaves
// the league FINISHED.
func (s *LeagueService) PlayAll(ctx context.Context, leagueID int64) (*PlayResult, error) {
	var result *PlayResult
	err := s.tx.WithinTx(ctx, func(r domain.Repositories) error {
		league, err := r.Leagues().GetByIDForUpdate(ctx, leagueID)
		if err != nil {
			return err
		}
		if league.Status == domain.LeagueStatusFinished {
			return fmt.Errorf("%w: league already finished", domain.ErrConflict)
		}

		var weeks []WeekMatches
		var standings []domain.StandingRow
		for week := league.CurrentWeek + 1; week <= league.TotalWeeks; week++ {
			played, err := s.simulateWeek(ctx, r, league, week)
			if err != nil {
				return err
			}
			standings, err = s.advanceAndSnapshot(ctx, r, league, week)
			if err != nil {
				return err
			}
			weeks = append(weeks, WeekMatches{Week: week, Matches: played})
		}
		result = &PlayResult{
			PlayedWeeks: weeks,
			Standings:   standings,
			CurrentWeek: league.TotalWeeks,
			Status:      domain.LeagueStatusFinished,
		}
		return nil
	})
	return result, err
}

// Reset un-plays a league: results are cleared, snapshots dropped, and
// the league returns to week 0 / NOT_STARTED. Fixtures are preserved, so
// replaying with the same seed reproduces the same results.
func (s *LeagueService) Reset(ctx context.Context, leagueID int64) error {
	return s.tx.WithinTx(ctx, func(r domain.Repositories) error {
		if _, err := r.Leagues().GetByIDForUpdate(ctx, leagueID); err != nil {
			return err
		}
		if err := r.Matches().ClearResults(ctx, leagueID); err != nil {
			return fmt.Errorf("clear results: %w", err)
		}
		// fromWeek 0 deletes every snapshot for the league.
		if err := r.Snapshots().DeleteFromWeek(ctx, leagueID, 0); err != nil {
			return fmt.Errorf("delete snapshots: %w", err)
		}
		if err := r.Leagues().UpdateStatusAndWeek(ctx, leagueID, domain.LeagueStatusNotStarted, 0); err != nil {
			return fmt.Errorf("reset league state: %w", err)
		}
		return nil
	})
}

// simulateWeek scores every match in the given week and records the
// results. The per-week RNG is seeded from (league seed, week) so a week
// produces identical results whether reached via play-week or play-all,
// and replays identically after a reset.
func (s *LeagueService) simulateWeek(ctx context.Context, r domain.Repositories, league *domain.League, week int) ([]domain.Match, error) {
	teams, err := r.Teams().ListByLeague(ctx, league.ID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	byID := make(map[int64]domain.Team, len(teams))
	for _, t := range teams {
		byID[t.ID] = t
	}

	matches, err := r.Matches().ListByLeagueAndWeek(ctx, league.ID, week)
	if err != nil {
		return nil, fmt.Errorf("list week matches: %w", err)
	}

	rng := rand.New(rand.NewPCG(uint64(league.RandomSeed), uint64(week)))
	for _, m := range matches {
		home := byID[m.HomeTeamID]
		away := byID[m.AwayTeamID]
		hg, ag := s.simulator.Simulate(home, away, rng)
		if err := r.Matches().UpdateResult(ctx, m.ID, hg, ag); err != nil {
			return nil, fmt.Errorf("record result: %w", err)
		}
	}

	// Re-fetch so the returned matches carry the persisted scores.
	return r.Matches().ListByLeagueAndWeek(ctx, league.ID, week)
}

// advanceAndSnapshot recomputes the cumulative table after `week`,
// persists it as that week's snapshot, advances current_week, and
// updates league status. Returns the freshly computed standings.
func (s *LeagueService) advanceAndSnapshot(ctx context.Context, r domain.Repositories, league *domain.League, week int) ([]domain.StandingRow, error) {
	teams, err := r.Teams().ListByLeague(ctx, league.ID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	matches, err := r.Matches().ListByLeague(ctx, league.ID)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}
	standings := s.standings.Calculate(teams, matches)

	if err := r.Snapshots().Upsert(ctx, league.ID, week, standings); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}
	if err := r.Leagues().UpdateStatusAndWeek(ctx, league.ID, statusForWeek(week, league.TotalWeeks), week); err != nil {
		return nil, fmt.Errorf("advance league: %w", err)
	}
	return standings, nil
}

// statusForWeek returns FINISHED once the last week is played, otherwise
// IN_PROGRESS.
func statusForWeek(week, totalWeeks int) domain.LeagueStatus {
	if week >= totalWeeks {
		return domain.LeagueStatusFinished
	}
	return domain.LeagueStatusInProgress
}
