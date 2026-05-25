package postgres_test

import (
	"context"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

// seedPlayedMatch creates a league with one played match and returns its
// match ID, so audit tests have a real match_id to reference (FK).
func seedPlayedMatch(t *testing.T, ctx context.Context, pool postgres.Querier) int64 {
	t.Helper()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	matchRepo := postgres.NewMatchRepo(pool)
	if err := matchRepo.BulkCreate(ctx, []domain.Match{
		{LeagueID: leagueID, WeekNumber: 1, HomeTeamID: ids[0], AwayTeamID: ids[1]},
	}); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	week1, _ := matchRepo.ListByLeagueAndWeek(ctx, leagueID, 1)
	if err := matchRepo.UpdateResult(ctx, week1[0].ID, 1, 0); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}
	return week1[0].ID
}

func TestMatchAuditRepo_CreateAndList(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	matchID := seedPlayedMatch(t, ctx, pool)
	repo := postgres.NewMatchAuditRepo(pool)

	first := &domain.MatchAudit{
		MatchID: matchID, OldHomeGoals: 1, OldAwayGoals: 0,
		NewHomeGoals: 2, NewAwayGoals: 2, Reason: "correction one",
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if first.ID == 0 || first.ChangedAt.IsZero() {
		t.Errorf("Create did not populate id/changed_at: %+v", first)
	}

	second := &domain.MatchAudit{
		MatchID: matchID, OldHomeGoals: 2, OldAwayGoals: 2,
		NewHomeGoals: 3, NewAwayGoals: 1, Reason: "correction two",
	}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	history, err := repo.ListByMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("ListByMatch: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history len = %d, want 2", len(history))
	}
	// Newest first.
	if history[0].Reason != "correction two" {
		t.Errorf("history[0].Reason = %q, want newest 'correction two'", history[0].Reason)
	}
}
