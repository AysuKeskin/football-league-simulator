package service_test

import (
	"errors"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

func TestStandingsHistory_AfterPlayAll(t *testing.T) {
	leagueSvc, _, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Hist", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	history, err := leagueSvc.StandingsHistory(ctx, league.ID)
	if err != nil {
		t.Fatalf("StandingsHistory: %v", err)
	}
	if len(history) != 6 {
		t.Fatalf("history weeks = %d, want 6", len(history))
	}
	for i, snap := range history {
		if snap.Week != i+1 {
			t.Errorf("history[%d].Week = %d, want %d (ascending)", i, snap.Week, i+1)
		}
		if snap.CapturedAt.IsZero() {
			t.Errorf("week %d missing CapturedAt", snap.Week)
		}
		if len(snap.Rows) != 4 {
			t.Errorf("week %d has %d rows, want 4", snap.Week, len(snap.Rows))
		}
	}
}

// TestStandingsHistory_EditAdvancesCapturedAt confirms editing a week-1
// result advances that week's CapturedAt (rewrite is detectable).
func TestStandingsHistory_EditAdvancesCapturedAt(t *testing.T) {
	leagueSvc, matchSvc, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Edit", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	before, err := leagueSvc.StandingsHistory(ctx, league.ID)
	if err != nil {
		t.Fatalf("StandingsHistory before: %v", err)
	}
	week1Before := before[0].CapturedAt

	matchID := firstPlayedMatch(t, ctx, pool, league.ID, 1)
	if _, err := matchSvc.UpdateResult(ctx, matchID, service.UpdateResultInput{HomeGoals: 5, AwayGoals: 0}); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}

	after, err := leagueSvc.StandingsHistory(ctx, league.ID)
	if err != nil {
		t.Fatalf("StandingsHistory after: %v", err)
	}
	if after[0].Week != 1 {
		t.Fatalf("first history entry should be week 1, got %d", after[0].Week)
	}
	// The edit reran rebuildSnapshots for week 1, so its captured_at must
	// be strictly newer (play-all and the edit are separate transactions).
	if !after[0].CapturedAt.After(week1Before) {
		t.Errorf("week-1 CapturedAt did not advance after edit: before=%v after=%v", week1Before, after[0].CapturedAt)
	}
}

func TestStandingsHistory_EmptyBeforePlay(t *testing.T) {
	leagueSvc, _, pool, ctx := newServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Fresh", TeamIDs: ids})

	history, err := leagueSvc.StandingsHistory(ctx, league.ID)
	if err != nil {
		t.Fatalf("StandingsHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("fresh league history = %d weeks, want 0", len(history))
	}
}

func TestStandingsHistory_NotFound(t *testing.T) {
	leagueSvc, _, _, ctx := newServices(t)
	if _, err := leagueSvc.StandingsHistory(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
