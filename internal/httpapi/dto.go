package httpapi

import (
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

// ---- Mappers (domain → response) ------------------------------------

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
