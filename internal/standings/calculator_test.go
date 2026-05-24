package standings

import (
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

func mkTeam(id int64, name string) domain.Team {
	t := domain.Team{Name: name}
	t.ID = id
	return t
}

func played(homeID, awayID int64, hg, ag int) domain.Match {
	h, a := hg, ag
	return domain.Match{
		HomeTeamID: homeID,
		AwayTeamID: awayID,
		HomeGoals:  &h,
		AwayGoals:  &a,
		Status:     domain.MatchStatusPlayed,
	}
}

func scheduled(homeID, awayID int64) domain.Match {
	return domain.Match{
		HomeTeamID: homeID,
		AwayTeamID: awayID,
		Status:     domain.MatchStatusScheduled,
	}
}

// findByName returns the row for the given team name (test helper).
func findByName(t *testing.T, rows []domain.StandingRow, name string) domain.StandingRow {
	t.Helper()
	for _, r := range rows {
		if r.TeamName == name {
			return r
		}
	}
	t.Fatalf("no row for %q", name)
	return domain.StandingRow{}
}

func TestCalculate_BasicAccumulation(t *testing.T) {
	teams := []domain.Team{
		mkTeam(1, "Alpha"),
		mkTeam(2, "Bravo"),
		mkTeam(3, "Charlie"),
		mkTeam(4, "Delta"),
	}
	matches := []domain.Match{
		played(1, 2, 3, 1), // Alpha beats Bravo
		played(1, 3, 2, 2), // Alpha draws Charlie
		played(4, 1, 2, 1), // Delta beats Alpha
	}

	rows := New().Calculate(teams, matches)
	alpha := findByName(t, rows, "Alpha")

	if alpha.Played != 3 || alpha.Won != 1 || alpha.Drawn != 1 || alpha.Lost != 1 {
		t.Errorf("Alpha P/W/D/L = %d/%d/%d/%d, want 3/1/1/1", alpha.Played, alpha.Won, alpha.Drawn, alpha.Lost)
	}
	if alpha.GoalsFor != 6 || alpha.GoalsAgainst != 5 || alpha.GoalDifference != 1 {
		t.Errorf("Alpha GF/GA/GD = %d/%d/%d, want 6/5/1", alpha.GoalsFor, alpha.GoalsAgainst, alpha.GoalDifference)
	}
	if alpha.Points != 4 {
		t.Errorf("Alpha points = %d, want 4 (1W + 1D)", alpha.Points)
	}
}

func TestCalculate_RankIsOneBased(t *testing.T) {
	teams := []domain.Team{mkTeam(1, "A"), mkTeam(2, "B")}
	rows := New().Calculate(teams, []domain.Match{played(1, 2, 1, 0)})

	if rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Errorf("ranks = %d, %d; want 1, 2", rows[0].Rank, rows[1].Rank)
	}
}

func TestCalculate_TieBreakByGoalDifference(t *testing.T) {
	// Two teams, both 1 win: A wins 5-0, B wins 1-0. Equal points,
	// A should rank first on GD.
	teams := []domain.Team{
		mkTeam(1, "A"), mkTeam(2, "B"),
		mkTeam(3, "X"), mkTeam(4, "Y"),
	}
	matches := []domain.Match{
		played(1, 3, 5, 0), // A beats X 5-0
		played(2, 4, 1, 0), // B beats Y 1-0
	}
	rows := New().Calculate(teams, matches)

	if rows[0].TeamName != "A" {
		t.Errorf("top team = %q, want A (higher GD)", rows[0].TeamName)
	}
}

func TestCalculate_TieBreakByGoalsFor(t *testing.T) {
	// Equal points and GD: A wins 3-1, B wins 2-0. Both have GD=2 and
	// 3 points; A's GF=3 beats B's GF=2.
	teams := []domain.Team{
		mkTeam(1, "A"), mkTeam(2, "B"),
		mkTeam(3, "X"), mkTeam(4, "Y"),
	}
	matches := []domain.Match{
		played(1, 3, 3, 1),
		played(2, 4, 2, 0),
	}
	rows := New().Calculate(teams, matches)

	if rows[0].TeamName != "A" {
		t.Errorf("top team = %q, want A (higher GF after GD tie)", rows[0].TeamName)
	}
}

func TestCalculate_TieBreakByWins(t *testing.T) {
	// Constructed so A (1W 2L) and B (0W 3D) reach the same points,
	// goal difference, and goals for — they only differ in wins. The
	// comparator must put A first.
	//
	//   A: vs C 2-0 (W), vs D 0-1 (L), vs E 1-2 (L)  → 1W 2L, pts=3, GF=3, GA=3
	//   B: vs C 1-1,     vs D 1-1,     vs E 1-1      → 0W 3D, pts=3, GF=3, GA=3
	teams := []domain.Team{
		mkTeam(1, "A"), mkTeam(2, "B"),
		mkTeam(3, "C"), mkTeam(4, "D"), mkTeam(5, "E"),
	}
	matches := []domain.Match{
		played(1, 3, 2, 0),
		played(1, 4, 0, 1),
		played(1, 5, 1, 2),
		played(2, 3, 1, 1),
		played(2, 4, 1, 1),
		played(2, 5, 1, 1),
	}
	rows := New().Calculate(teams, matches)

	a := findByName(t, rows, "A")
	b := findByName(t, rows, "B")
	if a.Points != b.Points || a.GoalDifference != b.GoalDifference || a.GoalsFor != b.GoalsFor {
		t.Fatalf("test setup broken: A=%+v B=%+v should be tied through GF", a, b)
	}
	if a.Won == b.Won {
		t.Fatalf("test setup broken: A.Won == B.Won, can't exercise wins tie-break")
	}

	// A has 1 win, B has 0; A must outrank B.
	if a.Rank >= b.Rank {
		t.Errorf("A.Rank=%d B.Rank=%d — A (more wins) should outrank B", a.Rank, b.Rank)
	}
}

func TestCalculate_TieBreakByName(t *testing.T) {
	// No matches → all stats zero → alphabetical.
	teams := []domain.Team{mkTeam(1, "Zulu"), mkTeam(2, "Alpha"), mkTeam(3, "Mike")}
	rows := New().Calculate(teams, nil)

	wantOrder := []string{"Alpha", "Mike", "Zulu"}
	for i, name := range wantOrder {
		if rows[i].TeamName != name {
			t.Errorf("rows[%d] = %q, want %q", i, rows[i].TeamName, name)
		}
	}
}

func TestCalculate_IgnoresScheduledMatches(t *testing.T) {
	teams := []domain.Team{mkTeam(1, "A"), mkTeam(2, "B")}
	matches := []domain.Match{
		played(1, 2, 2, 0),
		scheduled(2, 1), // second leg not yet played
	}
	rows := New().Calculate(teams, matches)

	a := findByName(t, rows, "A")
	b := findByName(t, rows, "B")
	if a.Played != 1 || b.Played != 1 {
		t.Errorf("scheduled match leaked into stats: A.P=%d B.P=%d, want 1/1", a.Played, b.Played)
	}
}

func TestCalculate_IncludesTeamsWithNoMatches(t *testing.T) {
	teams := []domain.Team{mkTeam(1, "Lonely"), mkTeam(2, "Player")}
	rows := New().Calculate(teams, nil)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Played != 0 || r.Points != 0 {
			t.Errorf("%s should have zero stats, got %+v", r.TeamName, r)
		}
	}
}

func TestCalculate_PremierLeagueScoringSums(t *testing.T) {
	// 2W + 1D = 7 points sanity check.
	teams := []domain.Team{
		mkTeam(1, "A"), mkTeam(2, "B"),
		mkTeam(3, "C"), mkTeam(4, "D"),
	}
	matches := []domain.Match{
		played(1, 2, 1, 0), // A beats B
		played(1, 3, 1, 1), // A draws C
		played(1, 4, 2, 1), // A beats D
	}
	a := findByName(t, New().Calculate(teams, matches), "A")
	if a.Points != 7 {
		t.Errorf("A points = %d, want 7 (3+1+3)", a.Points)
	}
}

func TestCalculate_IgnoresMatchesWithUnknownTeams(t *testing.T) {
	// Match references team 99 (not in teams slice). Known sides still
	// accumulate; unknown side is silently dropped.
	teams := []domain.Team{mkTeam(1, "A"), mkTeam(2, "B")}
	matches := []domain.Match{
		played(1, 99, 3, 0), // 99 not in teams
	}
	rows := New().Calculate(teams, matches)

	a := findByName(t, rows, "A")
	if a.Played != 1 || a.Points != 3 || a.GoalsFor != 3 {
		t.Errorf("A should record the win even when opponent is unknown: got %+v", a)
	}
}
