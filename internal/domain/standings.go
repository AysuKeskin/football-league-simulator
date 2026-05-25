package domain

// StandingRow is one row of a league table.
//
// Rows are produced by the StandingsCalculator from a slice of played
// matches; the calculator owns the tie-break ordering (Premier League:
// points → goal difference → goals scored, with team name as a
// deterministic display order for level teams). Rank is 1-based.
type StandingRow struct {
	Rank           int
	TeamID         int64
	TeamName       string
	Played         int
	Won            int
	Drawn          int
	Lost           int
	GoalsFor       int
	GoalsAgainst   int
	GoalDifference int
	Points         int
}
