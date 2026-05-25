package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres"
	"github.com/AysuKeskin/football-league-simulator/internal/repository/postgres/dbtest"
)

func TestTransactor_CommitsOnSuccess(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	tr := postgres.NewTransactor(pool)

	var createdID int64
	err := tr.WithinTx(ctx, func(repos domain.Repositories) error {
		league := newLeague("Committed")
		if err := repos.Leagues().Create(ctx, league, nil); err != nil {
			return err
		}
		createdID = league.ID
		return nil
	})
	if err != nil {
		t.Fatalf("WithinTx: %v", err)
	}

	// The league must be visible after commit, read on the pool.
	if _, err := postgres.NewLeagueRepo(pool).GetByID(ctx, createdID); err != nil {
		t.Errorf("committed league not found: %v", err)
	}
}

func TestTransactor_RollsBackOnError(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	tr := postgres.NewTransactor(pool)

	sentinel := errors.New("boom")
	var attemptedID int64

	err := tr.WithinTx(ctx, func(repos domain.Repositories) error {
		league := newLeague("Doomed")
		if err := repos.Leagues().Create(ctx, league, nil); err != nil {
			return err
		}
		attemptedID = league.ID
		return sentinel // force rollback
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTx err = %v, want sentinel", err)
	}

	// The insert inside the rolled-back tx must not survive.
	_, err = postgres.NewLeagueRepo(pool).GetByID(ctx, attemptedID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("rolled-back league still present (err=%v); rollback did not work", err)
	}
}

func TestTransactor_MultipleReposShareTransaction(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	tr := postgres.NewTransactor(pool)

	// Create a league and its fixtures atomically through two repos in
	// one tx — the core compose pattern the service layer relies on.
	a := seedTeam(t, ctx, pool, "A")
	b := seedTeam(t, ctx, pool, "B")

	var leagueID int64
	err := tr.WithinTx(ctx, func(repos domain.Repositories) error {
		league := newLeague("Atomic")
		if err := repos.Leagues().Create(ctx, league, []int64{a, b}); err != nil {
			return err
		}
		leagueID = league.ID
		return repos.Matches().BulkCreate(ctx, []domain.Match{
			{LeagueID: league.ID, WeekNumber: 1, HomeTeamID: a, AwayTeamID: b},
		})
	})
	if err != nil {
		t.Fatalf("WithinTx: %v", err)
	}

	matches, err := postgres.NewMatchRepo(pool).ListByLeague(ctx, leagueID)
	if err != nil {
		t.Fatalf("ListByLeague: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match committed alongside league, got %d", len(matches))
	}
}

func TestLeagueRepo_GetByIDForUpdateReturnsRow(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	tr := postgres.NewTransactor(pool)

	repo := postgres.NewLeagueRepo(pool)
	league := newLeague("Lockable")
	if err := repo.Create(ctx, league, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// FOR UPDATE only locks inside a tx, so exercise it there.
	err := tr.WithinTx(ctx, func(repos domain.Repositories) error {
		got, err := repos.Leagues().GetByIDForUpdate(ctx, league.ID)
		if err != nil {
			return err
		}
		if got.ID != league.ID || got.Name != "Lockable" {
			t.Errorf("GetByIDForUpdate returned %+v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithinTx: %v", err)
	}
}
