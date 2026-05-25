package postgres_test

import (
	"context"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

// seedLeagueWithTeams creates a league plus four member teams and returns
// the league ID and the team IDs, so match tests have valid foreign keys.
func seedLeagueWithTeams(t *testing.T, ctx context.Context, pool postgres.Querier) (int64, []int64) {
	t.Helper()
	ids := []int64{
		seedTeam(t, ctx, pool, "T1"),
		seedTeam(t, ctx, pool, "T2"),
		seedTeam(t, ctx, pool, "T3"),
		seedTeam(t, ctx, pool, "T4"),
	}
	league := newLeague("MatchLeague")
	if err := postgres.NewLeagueRepo(pool).Create(ctx, league, ids); err != nil {
		t.Fatalf("seed league: %v", err)
	}
	return league.ID, ids
}

func TestMatchRepo_BulkCreateAndListOrdered(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewMatchRepo(pool)

	// Two weeks, intentionally inserted out of week order to prove the
	// query sorts by week_number.
	matches := []domain.Match{
		{LeagueID: leagueID, WeekNumber: 2, HomeTeamID: ids[0], AwayTeamID: ids[1]},
		{LeagueID: leagueID, WeekNumber: 1, HomeTeamID: ids[2], AwayTeamID: ids[3]},
	}
	if err := repo.BulkCreate(ctx, matches); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}

	got, err := repo.ListByLeague(ctx, leagueID)
	if err != nil {
		t.Fatalf("ListByLeague: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].WeekNumber != 1 || got[1].WeekNumber != 2 {
		t.Errorf("not ordered by week: %d, %d", got[0].WeekNumber, got[1].WeekNumber)
	}
	// Freshly created matches are SCHEDULED with no goals.
	if got[0].Status != domain.MatchStatusScheduled || got[0].HomeGoals != nil {
		t.Errorf("new match should be SCHEDULED with nil goals, got %+v", got[0])
	}
}

func TestMatchRepo_ListByLeagueAndWeek(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewMatchRepo(pool)

	matches := []domain.Match{
		{LeagueID: leagueID, WeekNumber: 1, HomeTeamID: ids[0], AwayTeamID: ids[1]},
		{LeagueID: leagueID, WeekNumber: 1, HomeTeamID: ids[2], AwayTeamID: ids[3]},
		{LeagueID: leagueID, WeekNumber: 2, HomeTeamID: ids[0], AwayTeamID: ids[2]},
	}
	if err := repo.BulkCreate(ctx, matches); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}

	got, err := repo.ListByLeagueAndWeek(ctx, leagueID, 1)
	if err != nil {
		t.Fatalf("ListByLeagueAndWeek: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("week 1 returned %d matches, want 2", len(got))
	}
}

// TestMatchRepo_UpdateResult covers the nullable-goal lifecycle: a match
// starts SCHEDULED with nil goals, and after UpdateResult it is PLAYED
// with both goals populated (the *int fields are no longer nil).
func TestMatchRepo_UpdateResult(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewMatchRepo(pool)

	if err := repo.BulkCreate(ctx, []domain.Match{
		{LeagueID: leagueID, WeekNumber: 1, HomeTeamID: ids[0], AwayTeamID: ids[1]},
	}); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}

	week1, _ := repo.ListByLeagueAndWeek(ctx, leagueID, 1)
	matchID := week1[0].ID

	if err := repo.UpdateResult(ctx, matchID, 3, 1); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}

	got, err := repo.GetByID(ctx, matchID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.MatchStatusPlayed {
		t.Errorf("status = %s, want PLAYED", got.Status)
	}
	if got.HomeGoals == nil || got.AwayGoals == nil || *got.HomeGoals != 3 || *got.AwayGoals != 1 {
		t.Errorf("goals not recorded correctly: %+v", got)
	}
	if got.PlayedAt == nil {
		t.Error("played_at should be set after UpdateResult")
	}
}
