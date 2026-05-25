package postgres_test

import (
	"context"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

// snapshotRow builds a StandingRow for the given team at a rank. Stats
// are filler — these tests care about persistence and ordering, not the
// numbers themselves.
func snapshotRow(teamID int64, rank, points int) domain.StandingRow {
	return domain.StandingRow{
		TeamID:         teamID,
		Rank:           rank,
		Played:         1,
		Won:            1,
		GoalsFor:       2,
		GoalsAgainst:   0,
		GoalDifference: 2,
		Points:         points,
	}
}

// TestSnapshotRepo_UpsertAndGetByWeek covers the insert path of Upsert
// and the team-name JOIN in GetByWeek.
func TestSnapshotRepo_UpsertAndGetByWeek(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewStandingsSnapshotRepo(pool)

	rows := []domain.StandingRow{
		snapshotRow(ids[0], 1, 9),
		snapshotRow(ids[1], 2, 6),
	}
	if err := repo.Upsert(ctx, leagueID, 1, rows); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repo.GetByWeek(ctx, leagueID, 1)
	if err != nil {
		t.Fatalf("GetByWeek: %v", err)
	}
	if len(got) != 2 || got[0].Rank != 1 || got[1].Rank != 2 {
		t.Fatalf("GetByWeek = %+v, want 2 rows ordered by rank", got)
	}
	// TeamName must be populated from the JOIN, not left blank.
	if got[0].TeamName == "" {
		t.Error("TeamName not populated from teams JOIN")
	}
}

// TestSnapshotRepo_UpsertReplacesExisting proves a second Upsert for the
// same week overwrites the previous rows rather than appending.
func TestSnapshotRepo_UpsertReplacesExisting(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewStandingsSnapshotRepo(pool)

	if err := repo.Upsert(ctx, leagueID, 1, []domain.StandingRow{
		snapshotRow(ids[0], 1, 3),
		snapshotRow(ids[1], 2, 0),
	}); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Re-run the same week with a single, different row.
	if err := repo.Upsert(ctx, leagueID, 1, []domain.StandingRow{
		snapshotRow(ids[1], 1, 3),
	}); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	got, err := repo.GetByWeek(ctx, leagueID, 1)
	if err != nil {
		t.Fatalf("GetByWeek: %v", err)
	}
	if len(got) != 1 || got[0].TeamID != ids[1] {
		t.Errorf("after replace got %+v, want single row for team %d", got, ids[1])
	}
}

func TestSnapshotRepo_ListHistoryOrderedWithCapturedAt(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewStandingsSnapshotRepo(pool)

	for week := 1; week <= 3; week++ {
		if err := repo.Upsert(ctx, leagueID, week, []domain.StandingRow{
			snapshotRow(ids[0], 1, week*3),
		}); err != nil {
			t.Fatalf("Upsert week %d: %v", week, err)
		}
	}

	history, err := repo.ListHistory(ctx, leagueID)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("ListHistory returned %d weeks, want 3", len(history))
	}
	for i, snap := range history {
		if snap.Week != i+1 {
			t.Errorf("history[%d].Week = %d, want %d (ascending)", i, snap.Week, i+1)
		}
		if snap.CapturedAt.IsZero() {
			t.Errorf("week %d missing CapturedAt", snap.Week)
		}
		if len(snap.Rows) != 1 {
			t.Errorf("week %d has %d rows, want 1", snap.Week, len(snap.Rows))
		}
	}
}

// TestSnapshotRepo_DeleteFromWeek covers the edit-result invalidation
// path: deleting from week 2 removes weeks 2 and 3 but keeps week 1.
func TestSnapshotRepo_DeleteFromWeek(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	leagueID, ids := seedLeagueWithTeams(t, ctx, pool)
	repo := postgres.NewStandingsSnapshotRepo(pool)

	for week := 1; week <= 3; week++ {
		if err := repo.Upsert(ctx, leagueID, week, []domain.StandingRow{
			snapshotRow(ids[0], 1, week*3),
		}); err != nil {
			t.Fatalf("Upsert week %d: %v", week, err)
		}
	}

	if err := repo.DeleteFromWeek(ctx, leagueID, 2); err != nil {
		t.Fatalf("DeleteFromWeek: %v", err)
	}

	history, err := repo.ListHistory(ctx, leagueID)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history) != 1 || history[0].Week != 1 {
		t.Errorf("after DeleteFromWeek(2), got %d weeks, want only week 1", len(history))
	}
}
