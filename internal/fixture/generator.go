// Package fixture builds the double round-robin schedule for a league
// using the standard circle method.
package fixture

import (
	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

func New() domain.FixtureGenerator {
	return circleGenerator{}
}

type circleGenerator struct{}

// Generate returns an empty slice when teamIDs is too small (n<4) or
// odd; the interface has no error return because callers validate
// before invoking. The seed argument is unused for now — the circle
// method is deterministic — but is kept on the interface for a future
// shuffled-pairing variant.
func (circleGenerator) Generate(teamIDs []int64, _ int64) []domain.Match {
	n := len(teamIDs)
	if n < 4 || n%2 != 0 {
		return nil
	}

	wheel := make([]int64, n)
	copy(wheel, teamIDs)

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
