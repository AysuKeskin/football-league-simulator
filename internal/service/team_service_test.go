package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
)

func newTeamService(t *testing.T) (*service.TeamService, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := dbtest.New(t)
	return service.NewTeamService(postgres.NewRepositories(pool)), pool, context.Background()
}

func TestTeamService_UpdateRating(t *testing.T) {
	svc, pool, ctx := newTeamService(t)
	ids := seedCatalog(t, ctx, pool, 4)

	got, err := svc.UpdateRating(ctx, ids[0], service.UpdateRatingInput{Attack: 95, Midfield: 90, Defense: 88})
	if err != nil {
		t.Fatalf("UpdateRating: %v", err)
	}
	if got.Attack != 95 || got.Midfield != 90 || got.Defense != 88 {
		t.Errorf("returned rating = %d/%d/%d, want 95/90/88", got.Attack, got.Midfield, got.Defense)
	}

	// Persisted, not just echoed back.
	reread, _ := svc.UpdateRating(ctx, ids[0], service.UpdateRatingInput{Attack: 95, Midfield: 90, Defense: 88})
	if reread.Attack != 95 {
		t.Errorf("re-read attack = %d, want 95", reread.Attack)
	}
}

func TestTeamService_UpdateRating_OutOfRange(t *testing.T) {
	svc, pool, ctx := newTeamService(t)
	ids := seedCatalog(t, ctx, pool, 4)

	for _, in := range []service.UpdateRatingInput{
		{Attack: 0, Midfield: 50, Defense: 50},
		{Attack: 50, Midfield: 101, Defense: 50},
		{Attack: 50, Midfield: 50, Defense: -5},
	} {
		if _, err := svc.UpdateRating(ctx, ids[0], in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("UpdateRating(%+v) err = %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestTeamService_UpdateRating_UnknownTeam(t *testing.T) {
	svc, _, ctx := newTeamService(t)
	_, err := svc.UpdateRating(ctx, 999999, service.UpdateRatingInput{Attack: 50, Midfield: 50, Defense: 50})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTeamService_ListByLeague(t *testing.T) {
	svc, pool, ctx := newTeamService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	// Put only two of the four catalog teams in the league.
	league := &domain.League{
		Name: "Subset", TotalWeeks: 2, Status: domain.LeagueStatusNotStarted, RandomSeed: 1,
	}
	if err := postgres.NewLeagueRepo(pool).Create(ctx, league, ids[:2]); err != nil {
		t.Fatalf("create league: %v", err)
	}

	got, err := svc.ListByLeague(ctx, league.ID)
	if err != nil {
		t.Fatalf("ListByLeague: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("league teams = %d, want 2", len(got))
	}
}

func TestTeamService_ListByLeague_MissingLeague(t *testing.T) {
	svc, _, ctx := newTeamService(t)
	if _, err := svc.ListByLeague(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
