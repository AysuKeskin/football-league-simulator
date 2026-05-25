package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

// seedTeam inserts a team directly via SQL so league_repo tests do not
// depend on TeamRepo being correct.
func seedTeam(t *testing.T, ctx context.Context, q postgres.Querier, name string) int64 {
	t.Helper()
	var id int64
	err := q.QueryRow(ctx,
		`INSERT INTO teams (name, attack, midfield, defense) VALUES ($1, 80, 80, 80) RETURNING id`,
		name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seedTeam(%q): %v", name, err)
	}
	return id
}

func newLeague(name string) *domain.League {
	return &domain.League{
		Name:        name,
		CurrentWeek: 0,
		TotalWeeks:  6,
		Status:      domain.LeagueStatusNotStarted,
		RandomSeed:  42,
	}
}

// TestLeagueRepo_CreateAndGet exercises Create (both the leagues insert
// and the league_teams multi-row insert) and GetByID together: round-
// trip the data and confirm IDs, timestamps, status, and membership
// count all survive.
func TestLeagueRepo_CreateAndGet(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewLeagueRepo(pool)

	a := seedTeam(t, ctx, pool, "Alpha")
	b := seedTeam(t, ctx, pool, "Bravo")

	league := newLeague("Demo")
	if err := repo.Create(ctx, league, []int64{a, b}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if league.ID == 0 || league.CreatedAt.IsZero() {
		t.Errorf("Create did not populate generated columns: %+v", league)
	}

	got, err := repo.GetByID(ctx, league.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Demo" || got.Status != domain.LeagueStatusNotStarted || got.RandomSeed != 42 {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	var memberships int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM league_teams WHERE league_id = $1`, league.ID,
	).Scan(&memberships); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberships != 2 {
		t.Errorf("memberships = %d, want 2", memberships)
	}
}

// TestLeagueRepo_GetByIDReturnsErrNotFound covers the sentinel-error
// pattern that Update and Delete also use; testing it once is enough.
func TestLeagueRepo_GetByIDReturnsErrNotFound(t *testing.T) {
	pool := dbtest.New(t)
	repo := postgres.NewLeagueRepo(pool)

	_, err := repo.GetByID(context.Background(), 999999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want wrapping domain.ErrNotFound", err)
	}
}

func TestLeagueRepo_ListNewestFirst(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewLeagueRepo(pool)

	for _, name := range []string{"First", "Second", "Third"} {
		if err := repo.Create(ctx, newLeague(name), nil); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 || got[0].Name != "Third" || got[2].Name != "First" {
		t.Errorf("order wrong: %+v", got)
	}
}

func TestLeagueRepo_UpdateStatusAndWeek(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewLeagueRepo(pool)

	league := newLeague("Mutable")
	if err := repo.Create(ctx, league, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatusAndWeek(ctx, league.ID, domain.LeagueStatusInProgress, 3); err != nil {
		t.Fatalf("UpdateStatusAndWeek: %v", err)
	}

	got, _ := repo.GetByID(ctx, league.ID)
	if got.Status != domain.LeagueStatusInProgress || got.CurrentWeek != 3 {
		t.Errorf("after update: status=%s week=%d, want IN_PROGRESS / 3", got.Status, got.CurrentWeek)
	}
}

// TestLeagueRepo_DeleteCascadesMemberships verifies two schema-level
// guarantees at once: deleting a league removes its league_teams rows
// (ON DELETE CASCADE), but the teams themselves survive (FK without
// cascade to teams).
func TestLeagueRepo_DeleteCascadesMemberships(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewLeagueRepo(pool)

	a := seedTeam(t, ctx, pool, "Alpha")
	league := newLeague("Doomed")
	if err := repo.Create(ctx, league, []int64{a}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, league.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var memberships int
	pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM league_teams WHERE league_id = $1`, league.ID,
	).Scan(&memberships)
	if memberships != 0 {
		t.Errorf("memberships after delete = %d, want 0 (cascade)", memberships)
	}

	var teamExists bool
	pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM teams WHERE id = $1)`, a,
	).Scan(&teamExists)
	if !teamExists {
		t.Error("team should survive league deletion")
	}
}
