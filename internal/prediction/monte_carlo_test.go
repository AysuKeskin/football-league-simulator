package prediction_test

import (
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
	"github.com/AysuKeskin/football-league-simulator/internal/prediction"
	"github.com/AysuKeskin/football-league-simulator/internal/simulation"
	"github.com/AysuKeskin/football-league-simulator/internal/standings"
)

func team(id int64, name string, attack, midfield, defense int) domain.Team {
	t := domain.Team{Name: name}
	t.ID = id
	t.Attack = attack
	t.Midfield = midfield
	t.Defense = defense
	return t
}

// scheduled returns an unplayed fixture between two teams.
func scheduled(league, week, home, away int64) domain.Match {
	return domain.Match{
		LeagueID: league, WeekNumber: int(week),
		HomeTeamID: home, AwayTeamID: away,
		Status: domain.MatchStatusScheduled,
	}
}

func newEngine() domain.PredictionEngine {
	return prediction.New(simulation.New(), standings.New())
}

// fourTeams returns four teams and a single round-robin of scheduled
// fixtures between them (no results yet).
func fourTeams() ([]domain.Team, []domain.Match) {
	teams := []domain.Team{
		team(1, "Strong", 95, 92, 90),
		team(2, "Good", 80, 80, 80),
		team(3, "Average", 70, 70, 70),
		team(4, "Weak", 45, 45, 45),
	}
	matches := []domain.Match{
		scheduled(1, 1, 1, 2), scheduled(1, 1, 3, 4),
		scheduled(1, 2, 1, 3), scheduled(1, 2, 2, 4),
		scheduled(1, 3, 1, 4), scheduled(1, 3, 2, 3),
	}
	return teams, matches
}

func TestPredict_DeterministicPerSeed(t *testing.T) {
	e := newEngine()
	teams, matches := fourTeams()

	a := e.Predict(teams, matches, 500, 42)
	b := e.Predict(teams, matches, 500, 42)

	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("prediction %d differs between same-seed runs:\n%+v\n%+v", i, a[i], b[i])
		}
	}
}

func TestPredict_StrongerTeamHigherChampionshipChance(t *testing.T) {
	e := newEngine()
	teams, matches := fourTeams()

	preds := e.Predict(teams, matches, 2000, 1)

	byName := map[string]domain.Prediction{}
	for _, p := range preds {
		byName[p.TeamName] = p
	}
	if byName["Strong"].ChampionshipChance <= byName["Weak"].ChampionshipChance {
		t.Errorf("Strong chance %.1f should exceed Weak chance %.1f",
			byName["Strong"].ChampionshipChance, byName["Weak"].ChampionshipChance)
	}
	// The strongest team should be the favourite (sorted first).
	if preds[0].TeamName != "Strong" {
		t.Errorf("favourite = %q, want Strong", preds[0].TeamName)
	}
}

func TestPredict_ChancesSumToHundred(t *testing.T) {
	e := newEngine()
	teams, matches := fourTeams()

	preds := e.Predict(teams, matches, 1000, 7)

	var sum float64
	for _, p := range preds {
		sum += p.ChampionshipChance
		if p.MostLikelyFinalPosition < 1 || p.MostLikelyFinalPosition > len(teams) {
			t.Errorf("%s most-likely position %d out of range", p.TeamName, p.MostLikelyFinalPosition)
		}
	}
	// Ties for first split the count across teams, so the total may
	// slightly exceed 100; allow a small tolerance.
	if sum < 99.0 || sum > 105.0 {
		t.Errorf("championship chances sum = %.2f, want ~100", sum)
	}
}

func TestPredict_AllPlayedLeaderIsCertain(t *testing.T) {
	e := newEngine()
	teams, _ := fourTeams()

	// A fully played league: no scheduled matches, so every trial is the
	// same actual table and the leader is champion 100% of the time.
	g := func(home, away int64, hg, ag int) domain.Match {
		h, a := hg, ag
		return domain.Match{
			LeagueID: 1, WeekNumber: 1, HomeTeamID: home, AwayTeamID: away,
			HomeGoals: &h, AwayGoals: &a, Status: domain.MatchStatusPlayed,
		}
	}
	played := []domain.Match{
		g(1, 2, 3, 0), // Strong wins
		g(1, 3, 3, 0),
		g(1, 4, 3, 0),
		g(2, 3, 1, 0),
		g(2, 4, 1, 0),
		g(3, 4, 1, 0),
	}

	preds := e.Predict(teams, played, 100, 99)
	if preds[0].TeamName != "Strong" || preds[0].ChampionshipChance != 100 {
		t.Errorf("leader should be Strong at 100%%, got %+v", preds[0])
	}
}
