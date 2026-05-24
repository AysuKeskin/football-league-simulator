package domain

import "time"

// MatchStatus enumerates the lifecycle of a single fixture.
//
// Scheduled matches have nil goal pointers; played matches have both
// goals set and PlayedAt populated. Edits to a played match preserve
// status but update goals (and PlayedAt to reflect the edit time).
type MatchStatus string

const (
	MatchStatusScheduled MatchStatus = "SCHEDULED"
	MatchStatusPlayed    MatchStatus = "PLAYED"
)

// Match is one fixture inside a league.
//
// HomeGoals/AwayGoals are pointers so a scheduled (unplayed) match can
// be distinguished from a 0-0 result without an extra sentinel.
// WeekNumber is 1-based and never changes after fixture generation.
type Match struct {
	BaseModel
	LeagueID   int64
	WeekNumber int
	HomeTeamID int64
	AwayTeamID int64
	HomeGoals  *int
	AwayGoals  *int
	Status     MatchStatus
	PlayedAt   *time.Time
}
