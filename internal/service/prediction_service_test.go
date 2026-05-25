package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/fixture"
	"github.com/AysuKeskin/football-league-simulator/internal/prediction"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
	"github.com/AysuKeskin/football-league-simulator/internal/service"
	"github.com/AysuKeskin/football-league-simulator/internal/simulation"
	"github.com/AysuKeskin/football-league-simulator/internal/standings"
)

// newPredictionServices wires a LeagueService (to drive play) and a
// PredictionService over one dbtest pool.
func newPredictionServices(t *testing.T) (*service.LeagueService, *service.PredictionService, *pgxpool.Pool, context.Context) {
	t.Helper()
	pool := dbtest.New(t)
	repos := postgres.NewRepositories(pool)
	tx := postgres.NewTransactor(pool)
	leagueSvc := service.NewLeagueService(repos, tx, fixture.New(), simulation.New(), standings.New())
	predSvc := service.NewPredictionService(repos, prediction.New(simulation.New(), standings.New()), standings.New())
	return leagueSvc, predSvc, pool, context.Background()
}

func TestPredict_RejectedBeforeWeek4(t *testing.T) {
	leagueSvc, predSvc, pool, ctx := newPredictionServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Early", TeamIDs: ids})

	// Play only three weeks — still below the week-4 gate.
	for i := 0; i < 3; i++ {
		if _, err := leagueSvc.PlayWeek(ctx, league.ID); err != nil {
			t.Fatalf("PlayWeek %d: %v", i+1, err)
		}
	}

	_, err := predSvc.Predict(ctx, league.ID, 0)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict before week 4", err)
	}
}

func TestPredict_InProgressReturnsForecast(t *testing.T) {
	leagueSvc, predSvc, pool, ctx := newPredictionServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Mid", TeamIDs: ids})

	for i := 0; i < 4; i++ {
		if _, err := leagueSvc.PlayWeek(ctx, league.ID); err != nil {
			t.Fatalf("PlayWeek %d: %v", i+1, err)
		}
	}

	res, err := predSvc.Predict(ctx, league.ID, 500)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if res.Finished {
		t.Fatal("league is mid-season, Finished should be false")
	}
	if res.Simulations != 500 || len(res.Predictions) != 4 {
		t.Errorf("unexpected forecast: sims=%d preds=%d", res.Simulations, len(res.Predictions))
	}
	var sum float64
	for _, p := range res.Predictions {
		sum += p.ChampionshipChance
	}
	if sum < 99.0 || sum > 105.0 {
		t.Errorf("championship chances sum = %.2f, want ~100", sum)
	}
}

func TestPredict_FinishedReturnsActualTable(t *testing.T) {
	leagueSvc, predSvc, pool, ctx := newPredictionServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Done", TeamIDs: ids})
	if _, err := leagueSvc.PlayAll(ctx, league.ID); err != nil {
		t.Fatalf("PlayAll: %v", err)
	}

	res, err := predSvc.Predict(ctx, league.ID, 0)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if !res.Finished {
		t.Fatal("finished league should report Finished=true")
	}
	if len(res.FinalStandings) != 4 || len(res.Predictions) != 0 {
		t.Errorf("finished result should carry standings, not predictions: %+v", res)
	}
	if res.Champion != res.FinalStandings[0].TeamName {
		t.Errorf("champion %q != table leader %q", res.Champion, res.FinalStandings[0].TeamName)
	}
}

func TestPredict_DefaultsSimulationsWhenUnset(t *testing.T) {
	leagueSvc, predSvc, pool, ctx := newPredictionServices(t)
	ids := seedCatalog(t, ctx, pool, 4)
	league, _ := leagueSvc.CreateLeague(ctx, service.CreateLeagueInput{Name: "Default", TeamIDs: ids})
	for i := 0; i < 4; i++ {
		leagueSvc.PlayWeek(ctx, league.ID)
	}

	res, err := predSvc.Predict(ctx, league.ID, 0)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if res.Simulations != 10000 {
		t.Errorf("simulations = %d, want default 10000", res.Simulations)
	}
}

func TestPredict_NotFound(t *testing.T) {
	_, predSvc, _, ctx := newPredictionServices(t)
	if _, err := predSvc.Predict(ctx, 999999, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
