package domain

// LeagueStatus enumerates the lifecycle of a league.
//
// Transitions: NotStarted -> InProgress -> Finished. Resetting a league
// moves it back to NotStarted; no other backward transitions are allowed.
type LeagueStatus string

const (
	LeagueStatusNotStarted LeagueStatus = "NOT_STARTED"
	LeagueStatusInProgress LeagueStatus = "IN_PROGRESS"
	LeagueStatusFinished   LeagueStatus = "FINISHED"
)

// League is a single competition: a fixed set of teams playing a
// double round-robin under Premier League scoring rules.
//
// TotalWeeks is derived from team count at creation time (2*(n-1)) and
// never changes. CurrentWeek advances by exactly one each play-week call.
// RandomSeed makes the entire simulation reproducible.
type League struct {
	BaseModel
	Name        string
	CurrentWeek int
	TotalWeeks  int
	Status      LeagueStatus
	RandomSeed  int64
}
