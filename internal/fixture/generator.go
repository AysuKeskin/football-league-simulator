// Package fixture builds the double round-robin schedule for a league
// using the standard circle method.
package fixture

import (
	"math/rand/v2"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

func New() domain.FixtureGenerator {
	return circleGenerator{}
}

type circleGenerator struct{}

// Generate returns an empty slice when teamIDs is too small (n<4) or
// odd; the interface has no error return because callers validate
// before invoking.
//
// The seed shuffles the wheel before pairing, so the same teams with
// different seeds produce different (but each individually reproducible)
// schedules. Without this, the circle method would always pair team 0
// with team n-1 in week 1.
func (circleGenerator) Generate(teamIDs []int64, seed int64) []domain.Match {
	n := len(teamIDs)
	if n < 4 || n%2 != 0 {
		return nil
	}

	wheel := make([]int64, n)
	copy(wheel, teamIDs)

	// Seed split across PCG's two uint64 inputs so any int64 produces
	// a distinct stream. Same seed → same shuffle → same schedule.
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9E3779B97F4A7C15))
	rng.Shuffle(n, func(i, j int) { wheel[i], wheel[j] = wheel[j], wheel[i] })

	rounds := n - 1
	perRound := n / 2
	matches := make([]domain.Match, 0, n*(n-1))

	for r := 0; r < rounds; r++ {
		week := r + 1
		for i := 0; i < perRound; i++ {
			home := wheel[i]
			away := wheel[n-1-i]

			matches = append(matches, domain.Match{
				WeekNumber: week,
				HomeTeamID: home,
				AwayTeamID: away,
				Status:     domain.MatchStatusScheduled,
			})
			// Second leg: same pair, sides swapped, scheduled rounds
			// weeks later so every pair plays once per half.
			matches = append(matches, domain.Match{
				WeekNumber: week + rounds,
				HomeTeamID: away,
				AwayTeamID: home,
				Status:     domain.MatchStatusScheduled,
			})
		}

		// Circle rotation: fix wheel[0], shift wheel[1..n-1] right,
		// wrapping the last element into position 1.
		last := wheel[n-1]
		copy(wheel[2:], wheel[1:n-1])
		wheel[1] = last
	}

	return matches
}
