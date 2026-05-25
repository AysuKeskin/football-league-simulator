package httpapi

import (
	"time"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

// ---- Requests -------------------------------------------------------

// createLeagueRequest is the body of POST /leagues. Both teamIds and
// seed are optional: omit teamIds to use the default catalog, omit seed
// for a random (but stored) seed.
type createLeagueRequest struct {
	Name    string  `json:"name" binding:"required"`
	TeamIDs []int64 `json:"teamIds"`
	Seed    *int64  `json:"seed"`
}

// updateResultRequest is the body of PUT /matches/{id}. Goals are
// pointers with binding:"required" so omitting a field is a 400 while an
// explicit 0 (a real 0-0 scoreline) is accepted. Reason is optional.
type updateResultRequest struct {
	HomeGoals *int   `json:"homeGoals" binding:"required"`
	AwayGoals *int   `json:"awayGoals" binding:"required"`
	Reason    string `json:"reason"`
}

// updateRatingRequest is the body of PATCH /teams/{id}/ratings. All three
// fields are required and range-checked at bind time, so a missing or
// out-of-range value is a 400 before the service is called.
type updateRatingRequest struct {
	Attack   int `json:"attack"   binding:"required,min=1,max=100"`
	Midfield int `json:"midfield" binding:"required,min=1,max=100"`
	Defense  int `json:"defense"  binding:"required,min=1,max=100"`
}

// ---- Responses ------------------------------------------------------

type leagueResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	CurrentWeek int    `json:"currentWeek"`
	TotalWeeks  int    `json:"totalWeeks"`
	Status      string `json:"status"`
	Seed        int64  `json:"seed"`
}

type standingResponse struct {
	Rank           int    `json:"rank"`
	TeamID         int64  `json:"teamId"`
	Team           string `json:"team"`
	Played         int    `json:"played"`
	Won            int    `json:"won"`
	Drawn          int    `json:"drawn"`
	Lost           int    `json:"lost"`
	GoalsFor       int    `json:"goalsFor"`
	GoalsAgainst   int    `json:"goalsAgainst"`
	GoalDifference int    `json:"goalDifference"`
	Points         int    `json:"points"`
}

type matchResponse struct {
	ID         int64  `json:"id"`
	Week       int    `json:"week"`
	HomeTeamID int64  `json:"homeTeamId"`
	AwayTeamID int64  `json:"awayTeamId"`
	HomeGoals  *int   `json:"homeGoals"`
	AwayGoals  *int   `json:"awayGoals"`
	Status     string `json:"status"`
}

type weekMatchesResponse struct {
	Week    int             `json:"week"`
	Matches []matchResponse `json:"matches"`
}

type fixturesResponse struct {
	Weeks []weekMatchesResponse `json:"weeks"`
}

type weekDetailResponse struct {
	Week      int                `json:"week"`
	Matches   []matchResponse    `json:"matches"`
	Standings []standingResponse `json:"standings"`
}

type playResponse struct {
	CurrentWeek int                   `json:"currentWeek"`
	Status      string                `json:"status"`
	PlayedWeeks []weekMatchesResponse `json:"playedWeeks"`
	Standings   []standingResponse    `json:"standings"`
}

// editResultResponse is returned by PUT /matches/{id}: the updated match
// plus the recomputed current table.
type editResultResponse struct {
	Match     matchResponse      `json:"match"`
	Standings []standingResponse `json:"standings"`
}

type teamResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Attack   int    `json:"attack"`
	Midfield int    `json:"midfield"`
	Defense  int    `json:"defense"`
}

type auditResponse struct {
	ID           int64  `json:"id"`
	MatchID      int64  `json:"matchId"`
	OldHomeGoals int    `json:"oldHomeGoals"`
	OldAwayGoals int    `json:"oldAwayGoals"`
	NewHomeGoals int    `json:"newHomeGoals"`
	NewAwayGoals int    `json:"newAwayGoals"`
	Reason       string `json:"reason"`
	ChangedAt    string `json:"changedAt"`
}

// ---- Mappers (domain → response) ------------------------------------

func toTeamResponse(t domain.Team) teamResponse {
	return teamResponse{
		ID: t.ID, Name: t.Name,
		Attack: t.Attack, Midfield: t.Midfield, Defense: t.Defense,
	}
}

func toTeamResponses(teams []domain.Team) []teamResponse {
	out := make([]teamResponse, len(teams))
	for i, t := range teams {
		out[i] = toTeamResponse(t)
	}
	return out
}

func toLeagueResponse(l *domain.League) leagueResponse {
	return leagueResponse{
		ID:          l.ID,
		Name:        l.Name,
		CurrentWeek: l.CurrentWeek,
		TotalWeeks:  l.TotalWeeks,
		Status:      string(l.Status),
		Seed:        l.RandomSeed,
	}
}

func toStandingResponses(rows []domain.StandingRow) []standingResponse {
	out := make([]standingResponse, len(rows))
	for i, r := range rows {
		out[i] = standingResponse{
			Rank: r.Rank, TeamID: r.TeamID, Team: r.TeamName,
			Played: r.Played, Won: r.Won, Drawn: r.Drawn, Lost: r.Lost,
			GoalsFor: r.GoalsFor, GoalsAgainst: r.GoalsAgainst,
			GoalDifference: r.GoalDifference, Points: r.Points,
		}
	}
	return out
}

func toMatchResponse(m domain.Match) matchResponse {
	return matchResponse{
		ID: m.ID, Week: m.WeekNumber,
		HomeTeamID: m.HomeTeamID, AwayTeamID: m.AwayTeamID,
		HomeGoals: m.HomeGoals, AwayGoals: m.AwayGoals,
		Status: string(m.Status),
	}
}

// groupByWeek turns a flat, week-ordered match slice into per-week
// groups for the fixtures response.
func groupByWeek(matches []domain.Match) []weekMatchesResponse {
	var weeks []weekMatchesResponse
	idx := map[int]int{} // week number → position in weeks
	for _, m := range matches {
		pos, ok := idx[m.WeekNumber]
		if !ok {
			pos = len(weeks)
			idx[m.WeekNumber] = pos
			weeks = append(weeks, weekMatchesResponse{Week: m.WeekNumber})
		}
		weeks[pos].Matches = append(weeks[pos].Matches, toMatchResponse(m))
	}
	return weeks
}

func toEditResultResponse(res *service.EditResult) editResultResponse {
	return editResultResponse{
		Match:     toMatchResponse(res.Match),
		Standings: toStandingResponses(res.Standings),
	}
}

func toAuditResponses(audits []domain.MatchAudit) []auditResponse {
	out := make([]auditResponse, len(audits))
	for i, a := range audits {
		out[i] = auditResponse{
			ID: a.ID, MatchID: a.MatchID,
			OldHomeGoals: a.OldHomeGoals, OldAwayGoals: a.OldAwayGoals,
			NewHomeGoals: a.NewHomeGoals, NewAwayGoals: a.NewAwayGoals,
			Reason:    a.Reason,
			ChangedAt: a.ChangedAt.UTC().Format(time.RFC3339),
		}
	}
	return out
}

func toPlayResponse(res *service.PlayResult) playResponse {
	weeks := make([]weekMatchesResponse, len(res.PlayedWeeks))
	for i, w := range res.PlayedWeeks {
		ms := make([]matchResponse, len(w.Matches))
		for j, m := range w.Matches {
			ms[j] = toMatchResponse(m)
		}
		weeks[i] = weekMatchesResponse{Week: w.Week, Matches: ms}
	}
	return playResponse{
		CurrentWeek: res.CurrentWeek,
		Status:      string(res.Status),
		PlayedWeeks: weeks,
		Standings:   toStandingResponses(res.Standings),
	}
}
