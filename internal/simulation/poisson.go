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
	// baseGoals calibrates per-side expected goals to roughly the EPL
	// long-run average (≈2.7 goals/game split across the two sides).
	baseGoals = 1.35
	// homeAdvantage shifts the home side's expected goals upward to
	// reproduce the well-documented home-field effect.
	homeAdvantage = 0.25
	// maxGoals bounds both the input lambda and the output. Clamping
	// the lambda (not just the sampled count) keeps Knuth's loop
	// bounded under pathological rating combinations; clamping the
	// output protects downstream code from absurd scorelines.
	maxGoals = 9
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
//
// Lambda is clamped to maxGoals before sampling rather than only
// clamping the output: extreme inputs (e.g. ratio 100+ from lopsided
// ratings) would otherwise make math.Exp(-lambda) underflow to 0 and
// loop indefinitely. The clamp distorts the distribution at the tail,
// which is acceptable because outputs are also capped at maxGoals.
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
