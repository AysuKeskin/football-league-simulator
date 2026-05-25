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

// newService wires the real Transactor, repos, and algorithm packages
// over a dbtest pool — an integration test of the service against a
// real database, no mocks. The pool is returned so tests can seed the
// catalog without calling dbtest.New again (which would re-truncate).
func newService(t *testing.T) (*service.LeagueService, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := dbtest.New(t)
	svc := service.NewLeagueService(
		postgres.NewRepositories(pool),
		postgres.NewTransactor(pool),
		fixture.New(),
		simulation.New(),
		standings.New(),
	)
	return svc, pool, context.Background()
}

// seedCatalog inserts n teams directly so CreateLeague's default-team
// path has a catalog to draw from. Returns their IDs.
func seedCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, n int) []int64 {
	t.Helper()
	ids := make([]int64, n)
	for i := 0; i < n; i++ {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO teams (name, attack, midfield, defense) VALUES ($1, 80, 80, 80) RETURNING id`,
			"Team"+string(rune('A'+i)),
		).Scan(&id)
		if err != nil {
			t.Fatalf("seed team: %v", err)
		}
		ids[i] = id
	}
	return ids
}

func TestCreateLeague_GeneratesFixturesForFourTeams(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)

	league, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Demo", TeamIDs: ids})
	if err != nil {
		t.Fatalf("CreateLeague: %v", err)
	}
	if league.ID == 0 || league.TotalWeeks != 6 || league.Status != domain.LeagueStatusNotStarted {
		t.Errorf("unexpected league: %+v", league)
	}

	// 4 teams → 12 fixtures, all SCHEDULED.
	matches, err := svc.Fixtures(ctx, league.ID)
	if err != nil {
		t.Fatalf("Fixtures: %v", err)
	}
	if len(matches) != 12 {
		t.Errorf("fixture count = %d, want 12", len(matches))
	}
}

func TestCreateLeague_DefaultsToWholeCatalog(t *testing.T) {
	svc, pool, ctx := newService(t)
	seedCatalog(t, ctx, pool, 4)

	// No TeamIDs → the four catalog teams are used.
	league, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "DefaultTeams"})
	if err != nil {
		t.Fatalf("CreateLeague: %v", err)
	}
	if league.TotalWeeks != 6 {
		t.Errorf("TotalWeeks = %d, want 6 (derived from 4 default teams)", league.TotalWeeks)
	}
}

func TestCreateLeague_RejectsOddTeamCount(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)

	_, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Odd", TeamIDs: ids[:3]})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput for 3 teams", err)
	}
}

func TestCreateLeague_SameSeedSameFixtures(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)
	seed := int64(99)

	a, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "A", TeamIDs: ids, Seed: &seed})
	if err != nil {
		t.Fatalf("CreateLeague A: %v", err)
	}
	b, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "B", TeamIDs: ids, Seed: &seed})
	if err != nil {
		t.Fatalf("CreateLeague B: %v", err)
	}

	ma, _ := svc.Fixtures(ctx, a.ID)
	mb, _ := svc.Fixtures(ctx, b.ID)
	if len(ma) != len(mb) {
		t.Fatalf("fixture counts differ: %d vs %d", len(ma), len(mb))
	}
	// Same seed → same pairing order per week.
	for i := range ma {
		if ma[i].WeekNumber != mb[i].WeekNumber ||
			ma[i].HomeTeamID != mb[i].HomeTeamID ||
			ma[i].AwayTeamID != mb[i].AwayTeamID {
			t.Fatalf("fixture %d differs between same-seed leagues", i)
		}
	}
}

func TestStandings_EmptyBeforeAnyPlay(t *testing.T) {
	svc, pool, ctx := newService(t)
	ids := seedCatalog(t, ctx, pool, 4)

	league, err := svc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Fresh", TeamIDs: ids})
	if err != nil {
		t.Fatalf("CreateLeague: %v", err)
	}

	rows, err := svc.Standings(ctx, league.ID)
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	// All four teams present, every stat zero before any match is played.
	if len(rows) != 4 {
		t.Fatalf("standings rows = %d, want 4", len(rows))
	}
	for _, r := range rows {
		if r.Played != 0 || r.Points != 0 {
			t.Errorf("team %q has non-zero stats before play: %+v", r.TeamName, r)
		}
	}
}

func TestGetLeague_NotFound(t *testing.T) {
	svc, _, ctx := newService(t)
	if _, err := svc.GetLeague(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
