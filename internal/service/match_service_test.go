package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/fixture"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
	"github.com/AysuKeskin/football-league-simulator/internal/simulation"
	"github.com/AysuKeskin/football-league-simulator/internal/standings"
)

// newServices wires both LeagueService and MatchService over one dbtest
// pool, returning the pool so tests can seed the catalog.
func newServices(t *testing.T) (*service.LeagueService, *service.MatchService, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := dbtest.New(t)
	repos := postgres.NewRepositories(pool)
	tx := postgres.NewTransactor(pool)
	league := service.NewLeagueService(repos, tx, fixture.New(), simulation.New(), standings.New())
	match := service.NewMatchService(repos, tx, standings.New())
	return league, match, pool, context.Background()
}

// firstPlayedMatch returns the id of a played match in the league's
// given week (week is assumed played).
func firstPlayedMatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, leagueID int64, week int) int64 {
	t.Helper()
	matches, err := postgres.NewMatchRepo(pool).ListByLeagueAndWeek(ctx, leagueID, week)
	if err != nil || len(matches) == 0 {
		t.Fatalf("no matches in week %d: %v", week, err)
	}
	return matches[0].ID
}

func TestUpdateResult_EditsAuditsAndRecomputes(t *testing.T) {
	leagueSvc, matchSvc, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Edit", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	matchID := firstPlayedMatch(t, ctx, pool, league.ID, 1)
	res, err := matchSvc.UpdateResult(ctx, matchID, service.UpdateResultInput{
		HomeGoals: 7, AwayGoals: 0, Reason: "manual correction",
	})
	if err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}
	if res.Match.HomeGoals == nil || *res.Match.HomeGoals != 7 || *res.Match.AwayGoals != 0 {
		t.Errorf("match not updated: %+v", res.Match)
	}

	// Audit recorded.
	history, err := matchSvc.GetAudit(ctx, matchID)
	if err != nil {
		t.Fatalf("GetAudit: %v", err)
	}
	if len(history) != 1 || history[0].NewHomeGoals != 7 || history[0].Reason != "manual correction" {
		t.Errorf("audit not recorded correctly: %+v", history)
	}
}

func TestUpdateResult_ScheduledMatchConflicts(t *testing.T) {
	leagueSvc, matchSvc, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Sched", TeamIDs: ids})
	// Do not play — week 1 matches are still SCHEDULED.

	matchID := firstPlayedMatch(t, ctx, pool, league.ID, 1) // exists, but scheduled
	_, err := matchSvc.UpdateResult(ctx, matchID, service.UpdateResultInput{HomeGoals: 1, AwayGoals: 1})
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict editing a scheduled match", err)
	}
}

func TestUpdateResult_MissingMatchNotFound(t *testing.T) {
	_, matchSvc, _, ctx := newServices(t)
	_, err := matchSvc.UpdateResult(ctx, 999999, service.UpdateResultInput{HomeGoals: 1, AwayGoals: 0})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// TestUpdateResult_RebuildsDownstreamSnapshots is the core consistency
// test: edit a week-1 result after the whole league is played and assert
// the week-1 snapshot reflects the new score.
func TestUpdateResult_RebuildsDownstreamSnapshots(t *testing.T) {
	leagueSvc, matchSvc, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Rebuild", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	matchID := firstPlayedMatch(t, ctx, pool, league.ID, 1)
	// Capture the week-1 snapshot before the edit.
	snapRepo := postgres.NewStandingsSnapshotRepo(pool)
	before, _ := snapRepo.GetByWeek(ctx, league.ID, 1)

	// Force a lopsided result so the table must change.
	if _, err := matchSvc.UpdateResult(ctx, matchID, service.UpdateResultInput{HomeGoals: 9, AwayGoals: 0}); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}

	after, _ := snapRepo.GetByWeek(ctx, league.ID, 1)
	if len(after) == 0 {
		t.Fatal("week-1 snapshot missing after edit")
	}
	// The edited home team's week-1 goals-for must now be at least 9.
	var found bool
	for _, row := range after {
		if row.GoalsFor >= 9 {
			found = true
		}
	}
	if !found {
		t.Errorf("week-1 snapshot not rebuilt with new score\nbefore=%+v\nafter=%+v", before, after)
	}
}

func TestRecalculate_RebuildsAllSnapshots(t *testing.T) {
	leagueSvc, _, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Recalc", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	// Wipe snapshots behind the service's back, then recalc restores them.
	snapRepo := postgres.NewStandingsSnapshotRepo(pool)
	if err := snapRepo.DeleteFromWeek(ctx, league.ID, 0); err != nil {
		t.Fatalf("DeleteFromWeek: %v", err)
	}

	if _, err := leagueSvc.Recalculate(ctx, league.ID); err != nil {
		t.Fatalf("Recalculate: %v", err)
	}

	all, err := snapRepo.ListHistory(ctx, league.ID)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(all) != 6 {
		t.Errorf("snapshots after recalc = %d weeks, want 6", len(all))
	}
}
