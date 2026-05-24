// Package simulation produces match results from team ratings.
//
// Goals are sampled from a Poisson distribution whose mean is derived
// from relative team strengths plus a home advantage term. Poisson is
// used (rather than uniform random goals) so realistic scorelines
// dominate — a uniform sampler would make 6-5 as likely as 1-0.
package simulation

import (
	"math"
	"math/rand/v2"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

const (
	baseGoals     = 1.35
	homeAdvantage = 0.25
	maxGoals      = 9
)

func New() domain.MatchSimulator {
	return poissonSimulator{}
}

type poissonSimulator struct{}

func (poissonSimulator) Simulate(home, away domain.Team, rng *rand.Rand) (int, int) {
	hs := strength(home, away)
	as := strength(away, home)

	homeExpected := baseGoals*(hs/as) + homeAdvantage
	awayExpected := baseGoals * (as / hs)

	return samplePoisson(homeExpected, rng), samplePoisson(awayExpected, rng)
}

// strength is always strictly positive because rating values are
// constrained to [1, 100] at the database edge, so the division in
// Simulate is safe.
func strength(team, opponent domain.Team) float64 {
	return 0.5*float64(team.Attack) +
		0.3*float64(team.Midfield) +
		0.2*float64(100-opponent.Defense)
}

// samplePoisson draws from Poisson(lambda) via Knuth's algorithm.
// Lambda is clamped to maxGoals so the loop stays bounded under
// pathological rating combinations.
func samplePoisson(lambda float64, rng *rand.Rand) int {
	if lambda <= 0 {
		return 0
	}
	if lambda > maxGoals {
		lambda = maxGoals
	}

	l := math.Exp(-lambda)
	k := 0
	p := 1.0
	for {
		k++
		p *= rng.Float64()
		if p <= l {
			break
		}
	}
	k-- // Knuth's loop overshoots by one

	if k > maxGoals {
		return maxGoals
	}
	return k
}
