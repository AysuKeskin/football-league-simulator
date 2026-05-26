# Simulation & Prediction Algorithm

How the simulator turns three integer ratings per team into match scores, and
how those scores feed the Monte Carlo championship forecast. Every formula and
constant below is mirrored from the code, with cross-references to keep this
doc in sync with the implementation.

- Match model: [`internal/simulation/poisson.go`](../internal/simulation/poisson.go)
- Prediction: [`internal/prediction/monte_carlo.go`](../internal/prediction/monte_carlo.go)
- Standings used by both: [`internal/standings/calculator.go`](../internal/standings/calculator.go)

---

## 1. Inputs

Each team carries three ratings, each constrained to `[1, 100]` by a DB CHECK:

| Rating | Meaning |
|---|---|
| `attack` | ability to create and convert chances |
| `midfield` | control, possession, chance suppression |
| `defense` | ability to prevent the opponent from scoring |

Because every rating is at least 1, the strength values below are always
strictly positive, so the divisions in the goal model never hit zero.

---

## 2. Team strength

For a fixture, each side's strength is computed **relative to its opponent's
defense** (`poisson.go`):

```
strength(team, opponent) = 0.5 · team.Attack
                         + 0.3 · team.Midfield
                         + 0.2 · (100 − opponent.Defense)
```

- **Attack is weighted highest (0.5)** — scoring is the dominant driver of
  goals.
- **Midfield (0.3)** contributes territory and control.
- **Defense enters as the *opponent's* `100 − defense` (0.2)** — you score more
  easily against a weak defense. Using the opponent's defense (rather than your
  own) is what couples the two sides: a strong back line suppresses the other
  team's expected goals.

---

## 3. Expected goals

Strengths are turned into per-side Poisson means (`poisson.go`):

```
homeExpected = baseGoals · (homeStrength / awayStrength) + homeAdvantage
awayExpected = baseGoals · (awayStrength / homeStrength)
```

| Constant | Value | Why |
|---|---|---|
| `baseGoals` | `1.35` | calibrates per-side expected goals to roughly the EPL long-run average (≈2.7 goals/game split across both sides) |
| `homeAdvantage` | `0.25` | shifts the home side's mean up to reproduce the documented home-field effect |
| `maxGoals` | `9` | clamps both the input mean (lambda) and the sampled output |

The ratio form means only the *relative* gap between the teams matters: two
evenly matched sides both sit near `baseGoals`; a mismatch pushes the favourite
up and the underdog down.

---

## 4. Sampling — Poisson, not uniform

Goals are drawn from a **Poisson distribution** via Knuth's algorithm, not from
a uniform random number. This matters: a uniform sampler would make a 6–5 as
likely as a 1–0, whereas Poisson concentrates probability on realistic
scorelines (0, 1, 2 goals) with a thinning tail.

Two clamps keep it well-behaved:
- **Lambda is clamped to `maxGoals` before sampling.** A lopsided rating ratio
  could otherwise make `e^(−lambda)` underflow to 0 and spin Knuth's loop
  forever. The clamp distorts only the extreme tail.
- **Output is clamped to `maxGoals`** so downstream code never sees absurd
  scores.

A mean `lambda <= 0` short-circuits to 0 goals.

---

## 5. Determinism

Nothing calls the global RNG or `time.Now()`. Every random draw flows from the
league's stored `random_seed`, so the same seed always reproduces the same
season and the same forecast.

- **Playing weeks**: each week derives its own stream,
  `rand.NewPCG(seed, weekNumber)`, so replaying after a reset is identical.
- **Prediction trials**: trial `i` uses `rand.NewPCG(seed, i)`, so a given
  `(seed, i)` pair is a fixed, repeatable trial — the whole prediction is
  deterministic for a fixed seed and simulation count.

---

## 6. Monte Carlo championship prediction

The forecast (`monte_carlo.go`) estimates how the season ends by replaying the
*remaining* fixtures many times:

1. **Partition** the fixtures once: PLAYED results are constant across trials;
   SCHEDULED matches are re-simulated each trial.
2. For each of `N` trials, simulate every scheduled match (§3–§4) with that
   trial's seeded RNG, then compute the **final table** with the same standings
   calculator used everywhere else.
3. Record each team's finishing **rank** in that trial.
4. Aggregate per team over all trials:

| Field | Definition |
|---|---|
| `championshipChance` | share of trials the team finished 1st, as a percentage (`rank-1 count / N · 100`) |
| `averageFinalPosition` | mean finishing rank across trials |
| `mostLikelyFinalPosition` | modal (most frequent) finishing rank |

### Request parameters

- `N` defaults to **10000** (`defaultSimulations`) and is capped at **100000**
  (`maxSimulations`) to bound latency; `?simulations=` overrides it.
- Predictions are offered only from **week 4** onward (`predictionsFromWeek`,
  matching the brief's championship panel). Earlier requests return
  `409 CONFLICT`.

### Finished leagues

Once a league is `FINISHED` there is nothing left to simulate, so the endpoint
returns the **actual final table** instead of a forecast: `finished: true`,
the `champion`, and `finalStandings`. (Standings tie-breaks — points → goal
difference → goals scored — are described in [DESIGN §5.3](DESIGN.md); team
name is only a stable display order, never a ranking criterion.)

---

## 7. Worked example

Two teams, home vs away:

| | attack | midfield | defense |
|---|---|---|---|
| **Home** | 90 | 85 | 82 |
| **Away** | 78 | 76 | 75 |

```
homeStrength = 0.5·90 + 0.3·85 + 0.2·(100−75) = 45 + 25.5 + 5.0 = 75.5
awayStrength = 0.5·78 + 0.3·76 + 0.2·(100−82) = 39 + 22.8 + 3.6 = 65.4

homeExpected = 1.35·(75.5/65.4) + 0.25 ≈ 1.558 + 0.25 ≈ 1.81
awayExpected = 1.35·(65.4/75.5)            ≈ 1.169
```

So the home side averages ≈1.8 goals, the away side ≈1.2 — the favourite, with
a home bump, but far from a certainty on any single match. The actual scoreline
is a Poisson draw around those means.

### Why a forecast can read 100% before the final week

`championshipChance` is the fraction of simulated finishes a team tops. With
one week left, if a leader's points lead exceeds what any chaser can still
gain, **every** simulated outcome leaves them first → 100%: the title is
mathematically clinched. If a chaser's chance is only vanishingly small rather
than zero, a finite sample can still round to 100%, so read it as "top in every
simulated finish" rather than a hard guarantee.
