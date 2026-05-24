# Design Document — Football League Simulator

A Go backend that simulates a football league with probabilistic match results, Premier League scoring, and Monte Carlo championship predictions.

---

## 1. Design principles

- **Interface-driven.** Services depend on interfaces declared in `internal/domain`; concrete implementations (Postgres repositories, Poisson simulator, Monte Carlo predictor) are swappable.
- **Minimal API surface.** Requests carry only what is required; responses are flat JSON without wrapping envelopes. IDs are integers, timestamps are RFC3339 UTC.
- **Deterministic by default.** Every league has a `random_seed`, making simulations reproducible.
- **Single source of truth.** Standings are derived from the `matches` table; snapshots are cached views. Editing a match always triggers a recompute.

---

## 2. Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.22 |
| HTTP framework | Gin |
| Database driver | pgx/v5 |
| Migrations | golang-migrate |
| Validation | go-playground/validator |
| Logging | zerolog |
| Tests | testify + dockertest |
| API docs | swaggo/swag |
| Container | Docker + docker-compose |

---

## 3. Project layout

```
cmd/
  server/main.go                 composition root
internal/
  config/                        env loading + validation
  domain/                        entities + interfaces (no external deps)
  fixture/                       FixtureGenerator (circle round-robin)
  simulation/                    MatchSimulator (Poisson-based)
  standings/                     StandingsCalculator (pure)
  prediction/                    PredictionEngine (Monte Carlo)
  repository/postgres/           pgx implementations
  service/                       orchestration layer
  handler/                       Gin handlers
  httpapi/
    router.go
    dto/                         request/response types
    errors.go                    error → HTTP mapping
  middleware/                    request-id, recovery, logging
  external/sportsdb/             TheSportsDB client + cache
database/
  schema.sql
  seed.sql
  queries.sql
  migrations/
docs/
api/
  openapi.yaml
  postman_collection.json
Dockerfile
docker-compose.yml
Makefile
.env.example
README.md
```

---

## 4. Domain model

### 4.1 Entities (struct composition)

```go
type BaseModel struct {
    ID        int64
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Rating struct {
    Attack   int  // 1-100
    Midfield int  // 1-100
    Defense  int  // 1-100
}

type Team struct {
    BaseModel
    Rating
    Name string
}

type League struct {
    BaseModel
    Name        string
    CurrentWeek int
    TotalWeeks  int
    Status      LeagueStatus  // NOT_STARTED | IN_PROGRESS | FINISHED
    RandomSeed  int64
}

type Match struct {
    BaseModel
    LeagueID   int64
    WeekNumber int
    HomeTeamID int64
    AwayTeamID int64
    HomeGoals  *int            // nil = not played
    AwayGoals  *int
    Status     MatchStatus     // SCHEDULED | PLAYED
    PlayedAt   *time.Time
}

type StandingRow struct {
    Rank           int
    TeamID         int64
    TeamName       string
    Played         int
    Won            int
    Drawn          int
    Lost           int
    GoalsFor       int
    GoalsAgainst   int
    GoalDifference int
    Points         int
}
```

### 4.2 Core interfaces

```go
type FixtureGenerator interface {
    Generate(teamIDs []int64, seed int64) []Match
}

type MatchSimulator interface {
    Simulate(home, away Team, rng *rand.Rand) (homeGoals, awayGoals int)
}

type StandingsCalculator interface {
    Calculate(teams []Team, matches []Match) []StandingRow
}

type PredictionEngine interface {
    Predict(ctx context.Context, leagueID int64, simulations int) ([]Prediction, error)
}

type LeagueRepository interface { ... }
type TeamRepository interface { ... }
type MatchRepository interface { ... }
type StandingsSnapshotRepository interface { ... }
type PredictionRunRepository interface { ... }
type MatchAuditRepository interface { ... }
type ExternalProfileRepository interface { ... }
```

Services depend only on interfaces. Tests substitute fakes; production wires Postgres, the Poisson simulator, and the Monte Carlo engine at the composition root.

---

## 5. Algorithms

### 5.1 Fixture generation — circle method

Standard round-robin for `n` even teams produces `n−1` rounds of `n/2` matches each. Run twice (swapping home/away on the second leg) for a double round-robin, yielding `2(n−1)` weeks. For four teams this gives **six weeks** and **twelve matches**.

Validation: `n >= 4` and `n % 2 == 0`.

### 5.2 Match simulation — probabilistic, not random

```
homeStrength = home.Attack * 0.5 + home.Midfield * 0.3 + (100 - away.Defense) * 0.2
awayStrength = away.Attack * 0.5 + away.Midfield * 0.3 + (100 - home.Defense) * 0.2

homeExpected = BASE_GOALS * (homeStrength / awayStrength) + HOME_ADVANTAGE
awayExpected = BASE_GOALS * (awayStrength / homeStrength)

homeGoals = Poisson(homeExpected)
awayGoals = Poisson(awayExpected)
```

Constants: `BASE_GOALS = 1.35`, `HOME_ADVANTAGE = 0.25`. Goals clamped to `[0, 9]`. The RNG is seeded from `league.random_seed`, so identical seeds produce identical leagues.

### 5.3 Standings — Premier League rules

Sort by **points → goal difference → goals for → wins → team name** (the final key is a deterministic tie-breaker). Implemented as a pure function over `[]Match`; no DB writes.

### 5.4 Monte Carlo prediction

For `N` simulations (default 10000):

1. Snapshot current standings from played matches.
2. For each unplayed match, simulate using a branched RNG.
3. Compute final standings and record each team's finishing rank.
4. Aggregate per team: `championshipChance = wins / N`, `averageFinalPosition`, `mostLikelyFinalPosition`.

Predictions are exposed once `league.currentWeek >= 4`. When the league is finished, the endpoint returns the actual final standings instead.

---

## 6. Database schema

```sql
leagues (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  current_week INT NOT NULL DEFAULT 0,
  total_weeks INT NOT NULL,
  status TEXT NOT NULL,                 -- NOT_STARTED | IN_PROGRESS | FINISHED
  random_seed BIGINT NOT NULL,
  created_at, updated_at TIMESTAMPTZ
)

teams (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  attack INT NOT NULL CHECK (attack BETWEEN 1 AND 100),
  midfield INT NOT NULL CHECK (midfield BETWEEN 1 AND 100),
  defense INT NOT NULL CHECK (defense BETWEEN 1 AND 100),
  created_at, updated_at TIMESTAMPTZ
)

league_teams (
  league_id BIGINT REFERENCES leagues ON DELETE CASCADE,
  team_id BIGINT REFERENCES teams,
  PRIMARY KEY (league_id, team_id)
)

matches (
  id BIGSERIAL PRIMARY KEY,
  league_id BIGINT REFERENCES leagues ON DELETE CASCADE,
  week_number INT NOT NULL,
  home_team_id BIGINT REFERENCES teams,
  away_team_id BIGINT REFERENCES teams,
  home_goals INT,                       -- NULL until played
  away_goals INT,
  status TEXT NOT NULL DEFAULT 'SCHEDULED',
  played_at TIMESTAMPTZ,
  INDEX (league_id, week_number)
)

standings_snapshots (
  id BIGSERIAL PRIMARY KEY,
  league_id BIGINT REFERENCES leagues ON DELETE CASCADE,
  week_number INT NOT NULL,
  captured_at TIMESTAMPTZ NOT NULL,
  UNIQUE (league_id, week_number)
)

standings_snapshot_rows (
  snapshot_id BIGINT REFERENCES standings_snapshots ON DELETE CASCADE,
  team_id BIGINT REFERENCES teams,
  rank INT, played INT, won INT, drawn INT, lost INT,
  goals_for INT, goals_against INT, goal_difference INT, points INT,
  PRIMARY KEY (snapshot_id, team_id)
)

prediction_runs (
  id BIGSERIAL PRIMARY KEY,
  league_id BIGINT REFERENCES leagues ON DELETE CASCADE,
  week_number INT NOT NULL,
  simulation_count INT NOT NULL,
  created_at TIMESTAMPTZ
)

prediction_results (
  prediction_run_id BIGINT REFERENCES prediction_runs ON DELETE CASCADE,
  team_id BIGINT REFERENCES teams,
  championship_chance NUMERIC(5,2),
  average_final_position NUMERIC(4,2),
  most_likely_final_position INT,
  PRIMARY KEY (prediction_run_id, team_id)
)

match_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  match_id BIGINT REFERENCES matches ON DELETE CASCADE,
  old_home_goals INT, old_away_goals INT,
  new_home_goals INT, new_away_goals INT,
  reason TEXT,
  changed_at TIMESTAMPTZ
)

external_team_profiles (
  team_id BIGINT PRIMARY KEY REFERENCES teams ON DELETE CASCADE,
  payload JSONB NOT NULL,
  source TEXT NOT NULL,
  fetched_at TIMESTAMPTZ NOT NULL
)
```

`database/queries.sql` ships the non-trivial reads (standings aggregation, top predictions, weekly match listing).

---

## 7. API surface

Responses are flat JSON. Errors use the shape `{ "error": { "code": "STRING", "message": "..." } }`.

### Core

| Method | Path | Purpose |
|---|---|---|
| POST   | `/api/v1/leagues` | Create league (teams and seed optional) |
| GET    | `/api/v1/leagues` | List leagues |
| GET    | `/api/v1/leagues/{id}` | League summary |
| POST   | `/api/v1/leagues/{id}/play-week` | Advance one week |
| POST   | `/api/v1/leagues/{id}/play-all` | Play to finish |
| POST   | `/api/v1/leagues/{id}/reset` | Reset to week 0 |
| GET    | `/api/v1/leagues/{id}/standings` | Current table |
| GET    | `/api/v1/leagues/{id}/fixtures` | All matches grouped by week |
| GET    | `/api/v1/leagues/{id}/weeks/{w}` | Week's matches + snapshot |
| GET    | `/api/v1/leagues/{id}/predictions?simulations=10000` | Monte Carlo (week ≥ 4) |
| PUT    | `/api/v1/matches/{id}` | Edit result, auto-recalculate |

### Extras

| Method | Path | Purpose |
|---|---|---|
| PATCH  | `/api/v1/teams/{id}/ratings` | Update ratings (affects future matches only) |
| GET    | `/api/v1/leagues/{id}/standings/history` | All weekly snapshots |
| GET    | `/api/v1/leagues/{id}/predictions/history` | All prediction runs |
| GET    | `/api/v1/matches/{id}/audit` | Edit log for a match |
| POST   | `/api/v1/leagues/{id}/recalculate` | Force re-derive standings |
| GET    | `/api/v1/teams/{id}/external-profile` | TheSportsDB metadata (cached) |
| POST   | `/api/v1/teams/{id}/external-profile/refresh` | Bust cache |
| GET    | `/api/v1/leagues/{id}/export` | Full state as JSON |
| POST   | `/api/v1/leagues/import` | Reimport state |
| GET    | `/health`, `/ready` | Liveness + DB ping |

### Example payloads

`POST /api/v1/leagues`
```json
{ "name": "Demo", "teamIds": [1,2,3,4], "seed": 42 }
```
```json
{ "id": 1, "name": "Demo", "currentWeek": 0, "totalWeeks": 6, "status": "NOT_STARTED", "seed": 42 }
```

`GET /api/v1/leagues/{id}/standings`
```json
[
  { "rank": 1, "teamId": 1, "team": "Chelsea", "played": 4, "won": 3, "drawn": 1, "lost": 0,
    "goalsFor": 13, "goalsAgainst": 2, "goalDifference": 11, "points": 10 }
]
```

`PUT /api/v1/matches/{id}`
```json
{ "homeGoals": 2, "awayGoals": 2, "reason": "Manual correction" }
```

---

## 8. Concurrency and consistency

- All multi-step writes (`play-week`, `play-all`, edit result) run inside a single `pgx.Tx`.
- `SELECT ... FOR UPDATE` on the `leagues` row at the start of any state mutation prevents races on `current_week`.
- After every successful state change the service recomputes standings and upserts the snapshot for that week.
- Editing a played match in week N rewrites snapshots for weeks N..currentWeek in the same transaction.

---

## 9. External integration (TheSportsDB)

- Free-tier endpoint `/searchteams.php?t={name}`.
- Client has a 2-second timeout and circuit-breaker behavior. On failure it returns the cached payload if one exists, otherwise a minimal local fallback.
- Cached in `external_team_profiles` with `fetched_at`; TTL 24h, manually refreshable via the dedicated endpoint.
- This path is isolated from the simulation flow and cannot block core endpoints.

---

## 10. Testing strategy

### Unit
- **Fixture:** four teams produce six weeks and twelve matches; every pair plays twice with swapped home/away.
- **Simulation:** identical seed produces identical scores; stronger teams win significantly more across 10k runs.
- **Standings:** all tie-breakers (GD, GF, wins, name).
- **Prediction:** deterministic seed produces deterministic Monte Carlo output.
- **Service:** rating changes do not retroactively alter past matches; editing a result triggers recalculation.

### Integration (dockertest spins up Postgres)
- Full lifecycle: create → play four weeks → predictions present → play all → status `FINISHED`.
- Edit a week-1 result after week 4 → standings and later snapshots updated correctly.
- External API timeout → fallback returned, simulation unaffected.

### Manual / demo
- The Postman collection scripts the entire demo flow end-to-end.

---

## 11. Documentation

| File | Content |
|---|---|
| `README.md` | Overview, one-command run, demo curl flow, feature list |
| `docs/DESIGN.md` | This document |
| `docs/API_DOCUMENTATION.md` | Endpoint reference with examples |
| `docs/PREDICTION_ALGORITHM.md` | Poisson model and Monte Carlo math |
| `docs/DATABASE_SCHEMA.md` | ERD + table descriptions |
| `docs/DEPLOYMENT.md` | Docker, env vars, deploy steps |
| `docs/RUNBOOK.md` | Operational troubleshooting |
| `api/openapi.yaml` | Generated from swag annotations |
| `api/postman_collection.json` | Demo collection |

---

## 12. Deployment

- `docker compose up` brings up the app and Postgres, applies migrations, and seeds default teams.
- `Makefile` targets: `run`, `test`, `migrate`, `seed`, `docker-up`, `docker-down`, `lint`, `swag`.
- `.env.example` documents `DATABASE_URL`, `PORT`, `SPORTSDB_API_KEY`, `LOG_LEVEL`.
- Target hosting: Fly.io (free Postgres tier, single `fly launch`). `docs/DEPLOYMENT.md` covers both local and remote.

---

## 13. Out of scope

- Authentication and authorization.
- Frontend (a small static viewer may be added as a follow-up).
- Real-time updates or websockets.
- Player-level modeling.
