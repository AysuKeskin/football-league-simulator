package fixture

import (
	"reflect"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// pairKey is an order-independent identifier for an unordered team pair.
func pairKey(a, b int64) [2]int64 {
	if a < b {
		return [2]int64{a, b}
	}
	return [2]int64{b, a}
}

func TestGenerate_FourTeamsProduceSixWeeks(t *testing.T) {
	g := New()
	matches := g.Generate([]int64{1, 2, 3, 4}, 0)

	if got, want := len(matches), 12; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}

	weeks := map[int]int{}
	for _, m := range matches {
		weeks[m.WeekNumber]++
	}
	if got, want := len(weeks), 6; got != want {
		t.Fatalf("distinct weeks = %d, want %d", got, want)
	}
	for w := 1; w <= 6; w++ {
		if weeks[w] != 2 {
			t.Errorf("week %d has %d matches, want 2", w, weeks[w])
		}
	}
}

func TestGenerate_EveryPairPlaysTwiceWithSwappedSides(t *testing.T) {
	g := New()
	matches := g.Generate([]int64{1, 2, 3, 4}, 0)

	// Each unordered pair must appear exactly twice; the two occurrences
	// must have opposite (home, away) assignments.
	pairCount := map[[2]int64]int{}
	directed := map[[2]int64]int{}
	for _, m := range matches {
		pairCount[pairKey(m.HomeTeamID, m.AwayTeamID)]++
		directed[[2]int64{m.HomeTeamID, m.AwayTeamID}]++
	}

	if got, want := len(pairCount), 6; got != want {
		t.Fatalf("distinct unordered pairs = %d, want %d", got, want)
	}
	for pair, c := range pairCount {
		if c != 2 {
			t.Errorf("pair %v played %d times, want 2", pair, c)
		}
	}
	for d, c := range directed {
		if c != 1 {
			t.Errorf("directed pair %v appeared %d times, want 1 (home/away should swap on second leg)", d, c)
		}
	}
}

func TestGenerate_NoTeamPlaysTwiceInSameWeek(t *testing.T) {
	g := New()
	matches := g.Generate([]int64{1, 2, 3, 4}, 0)

	weekTeams := map[int]map[int64]bool{}
	for _, m := range matches {
		teams, ok := weekTeams[m.WeekNumber]
		if !ok {
			teams = map[int64]bool{}
			weekTeams[m.WeekNumber] = teams
		}
		if teams[m.HomeTeamID] {
			t.Errorf("team %d plays twice in week %d", m.HomeTeamID, m.WeekNumber)
		}
		if teams[m.AwayTeamID] {
			t.Errorf("team %d plays twice in week %d", m.AwayTeamID, m.WeekNumber)
		}
		teams[m.HomeTeamID] = true
		teams[m.AwayTeamID] = true
	}
}

func TestGenerate_SixTeamsProduceTenWeeks(t *testing.T) {
	g := New()
	matches := g.Generate([]int64{1, 2, 3, 4, 5, 6}, 0)

	// 6 teams → 2*(6-1) = 10 weeks, 6*5 = 30 matches, 3 per week.
	if got, want := len(matches), 30; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}
	weeks := map[int]int{}
	for _, m := range matches {
		weeks[m.WeekNumber]++
	}
	if got, want := len(weeks), 10; got != want {
		t.Fatalf("distinct weeks = %d, want %d", got, want)
	}
	for w, c := range weeks {
		if c != 3 {
			t.Errorf("week %d has %d matches, want 3", w, c)
		}
	}
}

func TestGenerate_RejectsTooFewTeams(t *testing.T) {
	g := New()
	if got := g.Generate([]int64{1, 2, 3}, 0); len(got) != 0 {
		t.Errorf("3 teams returned %d matches, want 0", len(got))
	}
	if got := g.Generate([]int64{}, 0); len(got) != 0 {
		t.Errorf("empty teams returned %d matches, want 0", len(got))
	}
}

func TestGenerate_RejectsOddTeamCount(t *testing.T) {
	g := New()
	if got := g.Generate([]int64{1, 2, 3, 4, 5}, 0); len(got) != 0 {
		t.Errorf("5 teams returned %d matches, want 0", len(got))
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	g := New()
	teams := []int64{10, 20, 30, 40}
	first := g.Generate(teams, 42)
	second := g.Generate(teams, 42)

	if !reflect.DeepEqual(first, second) {
		t.Fatal("two calls with identical inputs produced different schedules")
	}
}

func TestGenerate_OutputStatusAndGoalsAreScheduled(t *testing.T) {
	g := New()
	matches := g.Generate([]int64{1, 2, 3, 4}, 0)
	for i, m := range matches {
		if m.Status != domain.MatchStatusScheduled {
			t.Errorf("match[%d] status = %q, want %q", i, m.Status, domain.MatchStatusScheduled)
		}
		if m.HomeGoals != nil || m.AwayGoals != nil {
			t.Errorf("match[%d] has non-nil goals, want nil for scheduled match", i)
		}
	}
}
