package postgres_test

import (
	"context"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

func newTeam(name string, attack, midfield, defense int) *domain.Team {
	t := &domain.Team{Name: name}
	t.Attack = attack
	t.Midfield = midfield
	t.Defense = defense
	return t
}

func TestTeamRepo_CreateAndGet(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewTeamRepo(pool)

	team := newTeam("Arsenal", 85, 84, 83)
	if err := repo.Create(ctx, team); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if team.ID == 0 || team.CreatedAt.IsZero() {
		t.Errorf("Create did not populate generated columns: %+v", team)
	}

	got, err := repo.GetByID(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Arsenal" || got.Attack != 85 || got.Midfield != 84 || got.Defense != 83 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestTeamRepo_ListByLeague(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	teamRepo := postgres.NewTeamRepo(pool)
	leagueRepo := postgres.NewLeagueRepo(pool)

	// Two teams in the league, one outside it.
	inA := newTeam("InsideA", 80, 80, 80)
	inB := newTeam("InsideB", 80, 80, 80)
	out := newTeam("Outside", 80, 80, 80)
	for _, tm := range []*domain.Team{inA, inB, out} {
		if err := teamRepo.Create(ctx, tm); err != nil {
			t.Fatalf("Create %s: %v", tm.Name, err)
		}
	}

	league := newLeague("Filtered")
	if err := leagueRepo.Create(ctx, league, []int64{inA.ID, inB.ID}); err != nil {
		t.Fatalf("Create league: %v", err)
	}

	got, err := teamRepo.ListByLeague(ctx, league.ID)
	if err != nil {
		t.Fatalf("ListByLeague: %v", err)
	}
	if len(got) != 2 || got[0].Name != "InsideA" || got[1].Name != "InsideB" {
		t.Errorf("ListByLeague = %+v, want [InsideA, InsideB] (Outside excluded)", got)
	}
}

func TestTeamRepo_UpdateRating(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	repo := postgres.NewTeamRepo(pool)

	team := newTeam("Tunable", 70, 70, 70)
	if err := repo.Create(ctx, team); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newRating := domain.Rating{Attack: 95, Midfield: 90, Defense: 88}
	if err := repo.UpdateRating(ctx, team.ID, newRating); err != nil {
		t.Fatalf("UpdateRating: %v", err)
	}

	got, _ := repo.GetByID(ctx, team.ID)
	if got.Attack != 95 || got.Midfield != 90 || got.Defense != 88 {
		t.Errorf("after update rating = %d/%d/%d, want 95/90/88", got.Attack, got.Midfield, got.Defense)
	}
}
