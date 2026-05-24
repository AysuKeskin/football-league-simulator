// Package fixture builds the schedule of matches for a league.
//
// The generator uses the standard "circle method" for round-robin
// tournaments: for n even teams the method produces n-1 rounds in which
// every team plays exactly once per round. Running it twice with home
// and away swapped on the second leg yields a full double round-robin
// (Premier League-style) of 2(n-1) weeks and n(n-1) matches.
//
// The package is pure: no I/O, no globals, no time-based randomness.
// The seed argument on the interface is reserved for forward
// compatibility and is intentionally unused by the deterministic
// circle method.
package fixture

import (
	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// New returns a circle-method double round-robin generator.
func New() domain.FixtureGenerator {
	return circleGenerator{}
}

type circleGenerator struct{}

// Generate produces a complete double round-robin for teamIDs.
//
// Returns an empty slice when teamIDs is too small (n<4) or odd —
// the interface has no error return because the caller (service layer)
// is expected to validate inputs before invoking.
//
// Matches are returned in chronological order with WeekNumber 1-based.
// LeagueID is zero on every match; the caller assigns it when persisting.
// The seed argument is unused for now (the circle method is fully
// deterministic) and is preserved on the interface for a future
// shuffled-pairing variant.
func (circleGenerator) Generate(teamIDs []int64, _ int64) []domain.Match {
	n := len(teamIDs)
	if n < 4 || n%2 != 0 {
		return nil
	}

	// Working copy: the circle method rotates all entries except index 0.
	wheel := make([]int64, n)
	copy(wheel, teamIDs)

	rounds := n - 1            // first-leg rounds
	perRound := n / 2          // matches per round
	totalMatches := n * (n - 1) // first leg + second leg
	matches := make([]domain.Match, 0, totalMatches)

	for r := 0; r < rounds; r++ {
		week := r + 1
		for i := 0; i < perRound; i++ {
			home := wheel[i]
			away := wheel[n-1-i]

			// First leg: home/away as paired by the wheel.
			matches = append(matches, domain.Match{
				WeekNumber: week,
				HomeTeamID: home,
				AwayTeamID: away,
				Status:     domain.MatchStatusScheduled,
			})

			// Second leg: same pairing, sides swapped, scheduled
			// rounds*1 weeks after the first leg's matching round so
			// every pair plays once per half.
			matches = append(matches, domain.Match{
				WeekNumber: week + rounds,
				HomeTeamID: away,
				AwayTeamID: home,
				Status:     domain.MatchStatusScheduled,
			})
		}

		// Rotate everything except wheel[0]: last element moves to
		// index 1, the rest shift right by one. Classic circle step.
		last := wheel[n-1]
		copy(wheel[2:], wheel[1:n-1])
		wheel[1] = last
	}

	return matches
}
