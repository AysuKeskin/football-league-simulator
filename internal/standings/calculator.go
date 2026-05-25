// Package standings derives the league table from a set of played
// matches using Premier League scoring and tie-break rules.
package standings

import (
	"sort"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

func New() domain.StandingsCalculator {
	return calculator{}
}

type calculator struct{}

// Calculate ignores matches not in status PLAYED and matches referencing
// teams absent from `teams`. Every team in `teams` appears in the output,
// even with zero played matches.
//
// Tie-break order follows the Premier League: points > goal difference >
// goals scored. Teams still level are deemed equal; the trailing name
// key is only a deterministic display order (PL lists level teams
// alphabetically), not a ranking criterion.
func (calculator) Calculate(teams []domain.Team, matches []domain.Match) []domain.StandingRow {
	rows := make(map[int64]*domain.StandingRow, len(teams))
	for _, t := range teams {
		rows[t.ID] = &domain.StandingRow{TeamID: t.ID, TeamName: t.Name}
	}

	for _, m := range matches {
		// A PLAYED match always has both goal pointers set; this is
		// enforced by the matches_played_goals_consistent CHECK in
		// database/schema.sql, so the status check is sufficient.
		if m.Status != domain.MatchStatusPlayed {
			continue
		}
		hg, ag := *m.HomeGoals, *m.AwayGoals
		applyResult(rows[m.HomeTeamID], hg, ag)
		applyResult(rows[m.AwayTeamID], ag, hg)
	}

	out := make([]domain.StandingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Points != b.Points:
			return a.Points > b.Points
		case a.GoalDifference != b.GoalDifference:
			return a.GoalDifference > b.GoalDifference
		case a.GoalsFor != b.GoalsFor:
			return a.GoalsFor > b.GoalsFor
		default:
			// Level on all PL criteria → deterministic display order only.
			return a.TeamName < b.TeamName
		}
	})

	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

// applyResult mutates a row in place; the nil check guards against a
// match referencing a team not in the input slice.
func applyResult(row *domain.StandingRow, scored, conceded int) {
	if row == nil {
		return
	}
	row.Played++
	row.GoalsFor += scored
	row.GoalsAgainst += conceded
	row.GoalDifference = row.GoalsFor - row.GoalsAgainst
	switch {
	case scored > conceded:
		row.Won++
		row.Points += 3
	case scored == conceded:
		row.Drawn++
		row.Points++
	default:
		row.Lost++
	}
}
