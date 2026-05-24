package simulation

import (
	"math/rand/v2"
	"testing"

	"github.com/AysuKeskin/football-league-simulator/internal/domain"
)

// newRNG returns a deterministic v2 RNG seeded from a single int64.
// We use PCG with the seed split across both required uint64 inputs.
func newRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9E3779B97F4A7C15))
}

func makeTeam(name string, attack, midfield, defense int) domain.Team {
	t := domain.Team{Name: name}
	t.Attack = attack
	t.Midfield = midfield
	t.Defense = defense
	return t
}

func TestSimulate_DeterministicPerSeed(t *testing.T) {
	sim := New()
	home := makeTeam("Home", 85, 80, 75)
	away := makeTeam("Away", 70, 70, 70)

	const runs = 100
	first := make([][2]int, runs)
	second := make([][2]int, runs)

	rng1 := newRNG(42)
	rng2 := newRNG(42)
	for i := 0; i < runs; i++ {
		h, a := sim.Simulate(home, away, rng1)
		first[i] = [2]int{h, a}
		h, a = sim.Simulate(home, away, rng2)
		second[i] = [2]int{h, a}
	}

	for i := 0; i < runs; i++ {
		if first[i] != second[i] {
			t.Fatalf("run %d: first=%v second=%v — same seed produced different results", i, first[i], second[i])
		}
	}
}

func TestSimulate_StrongerTeamWinsSignificantlyMore(t *testing.T) {
	sim := New()
	strong := makeTeam("Strong", 95, 90, 90)
	weak := makeTeam("Weak", 30, 30, 30)
	rng := newRNG(1)

	wins, draws, losses := 0, 0, 0
	const runs = 10000
	for i := 0; i < runs; i++ {
		h, a := sim.Simulate(strong, weak, rng)
		switch {
		case h > a:
			wins++
		case h == a:
			draws++
		default:
			losses++
		}
	}

	// The strong side should dominate by a wide margin. We assert a
	// ratio of wins to losses of at least 3:1 to keep the test stable
	// across random samples while still catching a broken simulator
	// (e.g. one that ignores ratings entirely, which would give ~50%).
	if wins < 3*losses {
		t.Errorf("strong team: wins=%d losses=%d draws=%d — wins should dominate losses by >=3x", wins, losses, draws)
	}
}

func TestSimulate_HomeAdvantageMeasurable(t *testing.T) {
	sim := New()
	a := makeTeam("A", 75, 75, 75)
	b := makeTeam("B", 75, 75, 75)
	rng := newRNG(1)

	// Same matchup, swap who is home each iteration; compare aggregate
	// wins by venue. Home advantage should produce more home wins than
	// away wins across a large sample.
	homeWins, awayWins := 0, 0
	const runs = 10000
	for i := 0; i < runs; i++ {
		h, away := sim.Simulate(a, b, rng)
		if h > away {
			homeWins++
		} else if away > h {
			awayWins++
		}
	}

	if homeWins <= awayWins {
		t.Errorf("home wins=%d, away wins=%d — home should win more often with home advantage applied", homeWins, awayWins)
	}
}

func TestSimulate_GoalsAlwaysInRange(t *testing.T) {
	sim := New()
	// Extreme inputs to exercise lambda clamping.
	extremeHome := makeTeam("Crusher", 100, 100, 100)
	weakAway := makeTeam("Fodder", 1, 1, 1)
	rng := newRNG(7)

	const runs = 5000
	for i := 0; i < runs; i++ {
		h, a := sim.Simulate(extremeHome, weakAway, rng)
		if h < 0 || h > maxGoals {
			t.Fatalf("home goals out of range: %d (max %d)", h, maxGoals)
		}
		if a < 0 || a > maxGoals {
			t.Fatalf("away goals out of range: %d (max %d)", a, maxGoals)
		}
	}
}

func TestSamplePoisson_ZeroLambdaReturnsZero(t *testing.T) {
	rng := newRNG(1)
	for i := 0; i < 100; i++ {
		if got := samplePoisson(0, rng); got != 0 {
			t.Fatalf("samplePoisson(0) = %d, want 0", got)
		}
		if got := samplePoisson(-0.5, rng); got != 0 {
			t.Fatalf("samplePoisson(-0.5) = %d, want 0", got)
		}
	}
}

func TestSamplePoisson_MeanApproximatesLambda(t *testing.T) {
	rng := newRNG(123)
	const lambda = 2.0
	const runs = 10000

	sum := 0
	for i := 0; i < runs; i++ {
		sum += samplePoisson(lambda, rng)
	}
	mean := float64(sum) / float64(runs)

	// With 10k samples the sample mean should be very close to lambda;
	// allow 10% tolerance to keep the test robust across seeds.
	if mean < lambda*0.9 || mean > lambda*1.1 {
		t.Errorf("sample mean = %.3f, want close to %.3f (within 10%%)", mean, lambda)
	}
}
