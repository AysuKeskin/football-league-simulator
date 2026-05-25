// Package prediction estimates how a league will finish by simulating
// the remaining fixtures many times (Monte Carlo) and aggregating the
// outcomes into per-team championship odds and expected positions.
package prediction

import (
	"math/rand/v2"
	"sort"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// New returns a Monte Carlo prediction engine that simulates remaining
// fixtures with sim and ranks each trial's outcome with calc.
func New(sim domain.MatchSimulator, calc domain.StandingsCalculator) domain.PredictionEngine {
	return engine{sim: sim, calc: calc}
}

type engine struct {
	sim  domain.MatchSimulator
	calc domain.StandingsCalculator
}

// Predict runs `simulations` independent trials. Played matches are kept
// as-is; scheduled matches are simulated afresh each trial. Each trial's
// final table contributes one finishing rank per team, which are then
// aggregated into championship chance, average position, and the modal
// (most likely) position.
func (e engine) Predict(teams []domain.Team, matches []domain.Match, simulations int, seed int64) []domain.Prediction {
	if simulations <= 0 || len(teams) == 0 {
		return nil
	}

	byID := make(map[int64]domain.Team, len(teams))
	for _, t := range teams {
		byID[t.ID] = t
	}

	// Partition once: played results are constant across trials.
	var played, scheduled []domain.Match
	for _, m := range matches {
		if m.Status == domain.MatchStatusPlayed {
			played = append(played, m)
		} else {
			scheduled = append(scheduled, m)
		}
	}

	n := len(teams)
	// rankCounts[teamID][r] = number of trials team finished at rank r+1.
	rankCounts := make(map[int64][]int, n)
	for _, t := range teams {
		rankCounts[t.ID] = make([]int, n)
	}

	// Reused per trial to avoid reallocating the combined slice.
	combined := make([]domain.Match, 0, len(played)+len(scheduled))

	for i := 0; i < simulations; i++ {
		// Independent, reproducible RNG per trial: same (seed, i) always
		// yields the same trial, so the whole prediction is deterministic.
		rng := rand.New(rand.NewPCG(uint64(seed), uint64(i)))

		combined = combined[:0]
		combined = append(combined, played...)
		for _, m := range scheduled {
			hg, ag := e.sim.Simulate(byID[m.HomeTeamID], byID[m.AwayTeamID], rng)
			sm := m
			sm.HomeGoals = &hg
			sm.AwayGoals = &ag
			sm.Status = domain.MatchStatusPlayed
			combined = append(combined, sm)
		}

		table := e.calc.Calculate(teams, combined)
		for _, row := range table {
			rankCounts[row.TeamID][row.Rank-1]++
		}
	}

	return aggregate(teams, byID, rankCounts, simulations)
}

// aggregate turns per-team rank tallies into predictions, sorted by
// championship chance desc then average position asc.
func aggregate(teams []domain.Team, byID map[int64]domain.Team, rankCounts map[int64][]int, simulations int) []domain.Prediction {
	out := make([]domain.Prediction, 0, len(teams))
	for id, counts := range rankCounts {
		var weightedRankSum, modeCount, modeRank int
		for r, c := range counts {
			weightedRankSum += (r + 1) * c
			if c > modeCount {
				modeCount = c
				modeRank = r + 1
			}
		}
		out = append(out, domain.Prediction{
			TeamID:                  id,
			TeamName:                byID[id].Name,
			ChampionshipChance:      float64(counts[0]) / float64(simulations) * 100,
			AverageFinalPosition:    float64(weightedRankSum) / float64(simulations),
			MostLikelyFinalPosition: modeRank,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ChampionshipChance != out[j].ChampionshipChance {
			return out[i].ChampionshipChance > out[j].ChampionshipChance
		}
		if out[i].AverageFinalPosition != out[j].AverageFinalPosition {
			return out[i].AverageFinalPosition < out[j].AverageFinalPosition
		}
		return out[i].TeamName < out[j].TeamName
	})
	return out
}
