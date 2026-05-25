package service_test

import (
	"errors"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

func TestPlayWeek_AdvancesWeekAndScores(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	seed := int64(7)
	league, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Play", TeamIDs: ids, Seed: &seed})
	if err != nil {
		t.Fatalf("CreateLeague: %v", err)
	}

	res, err := svc.PlayWeek(ctx, league.ID)
	if err != nil {
		t.Fatalf("PlayWeek: %v", err)
	}
	if res.CurrentWeek != 1 || res.Status != domain.LeagueStatusInProgress {
		t.Errorf("after week 1: week=%d status=%s, want 1 / IN_PROGRESS", res.CurrentWeek, res.Status)
	}
	if len(res.PlayedWeeks) != 1 || len(res.PlayedWeeks[0].Matches) != 2 {
		t.Fatalf("expected 1 week with 2 matches, got %+v", res.PlayedWeeks)
	}
	for _, m := range res.PlayedWeeks[0].Matches {
		if m.Status != domain.MatchStatusPlayed || m.HomeGoals == nil || m.AwayGoals == nil {
			t.Errorf("match not marked played with goals: %+v", m)
		}
	}
}

func TestPlayAll_FinishesLeague(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "All", TeamIDs: ids})
	if err != nil {
		t.Fatalf("CreateLeague: %v", err)
	}

	res, err := svc.PlayAll(ctx, league.ID)
	if err != nil {
		t.Fatalf("PlayAll: %v", err)
	}
	if res.Status != domain.LeagueStatusFinished || res.CurrentWeek != 6 {
		t.Errorf("after play-all: week=%d status=%s, want 6 / FINISHED", res.CurrentWeek, res.Status)
	}
	if len(res.PlayedWeeks) != 6 {
		t.Errorf("played weeks = %d, want 6", len(res.PlayedWeeks))
	}
	// Every team plays 6 games in a 4-team double round-robin.
	played := 0
	for _, row := range res.Standings {
		played += row.Played
	}
	if played != 24 { // 4 teams * 6 games
		t.Errorf("total games played across table = %d, want 24", played)
	}
}

func TestPlayAll_OnFinishedLeagueConflicts(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Done", TeamIDs: ids})
	if _, err := svc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("first PlayAll: %v", err)
	}

	_, err := svc.PlayAll(ctx, league.ID)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict on already-finished league", err)
	}
}

// TestPlayWeekByWeek_MatchesPlayAll proves the per-week (seed, week) RNG
// makes week-by-week play identical to play-all: both reach the same
// final standings for the same seed.
func TestPlayWeekByWeek_MatchesPlayAll(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	seed := int64(2024)

	// League A: play all at once.
	a, _ := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "AllAtOnce", TeamIDs: ids, Seed: &seed})
	resAll, err := svc.PlayAll(ctx, a.ID)
	if err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	// League B: same teams and seed, played one week at a time.
	b, _ := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "WeekByWeek", TeamIDs: ids, Seed: &seed})
	var resWeek *service.PlayResult
	for i := 0; i < 6; i++ {
		resWeek, err = svc.PlayWeek(ctx, b.ID)
		if err != nil {
			t.Fatalf("PlayWeek %d: %v", i+1, err)
		}
	}

	if len(resAll.Standings) != len(resWeek.Standings) {
		t.Fatalf("standings length mismatch")
	}
	for i := range resAll.Standings {
		x, y := resAll.Standings[i], resWeek.Standings[i]
		if x.TeamID != y.TeamID || x.Points != y.Points || x.GoalDifference != y.GoalDifference {
			t.Errorf("rank %d differs: play-all=%+v week-by-week=%+v", i+1, x, y)
		}
	}
}

func TestReset_ReturnsLeagueToNotStarted(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Resettable", TeamIDs: ids})
	if _, err := svc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	if err := svc.Reset(ctx, league.ID); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	got, _ := svc.GetLeague(ctx, league.ID)
	if got.Status != domain.LeagueStatusNotStarted || got.CurrentWeek != 0 {
		t.Errorf("after reset: status=%s week=%d, want NOT_STARTED / 0", got.Status, got.CurrentWeek)
	}
	// Standings must be empty again (all matches back to SCHEDULED).
	rows, _ := svc.Standings(ctx, league.ID)
	for _, r := range rows {
		if r.Played != 0 {
			t.Errorf("team %q still has %d played after reset", r.TeamName, r.Played)
		}
	}
}

// TestReset_ThenReplayIsDeterministic confirms replaying after a reset
// reproduces the same final table (fixtures + seed preserved).
func TestReset_ThenReplayIsDeterministic(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	seed := int64(55)
	league, _ := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Replay", TeamIDs: ids, Seed: &seed})

	first, _ := svc.PlayAll(ctx, league.ID)
	if err := svc.Reset(ctx, league.ID); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	second, _ := svc.PlayAll(ctx, league.ID)

	for i := range first.Standings {
		x, y := first.Standings[i], second.Standings[i]
		if x.TeamID != y.TeamID || x.Points != y.Points {
			t.Errorf("rank %d differs after replay: %+v vs %+v", i+1, x, y)
		}
	}
}
