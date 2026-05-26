# Design Document — Football League Simulator

A Go backend that simulates a football league with probabilistic match results, Premier League scoring, and Monte Carlo championship predictions.

---

## 1. Design principles

- **Interface-driven.** Services depend on interfaces declared in `internal/domain`; concrete implementations (Postgres repositories, Poisson simulator, Monte Carlo predictor) are swappable.
- **Minimal API surface.** Requests carry only what is required; responses are flat JSON without wrapping envelopes. IDs are integers, timestamps are RFC3339 UTC.
- **Deterministic by default.** Every league has a `random_seed`, making simulations reproducible.
- **Single source of truth.** Standings are derived from the `matches` table; snapshots are cached views. Editing a match always triggers a recompute.
- **Fixed team pool.** The team catalog is a fixed, seeded set of clubs. It can be listed and have its ratings edited, but teams are never created or deleted at runtime — a league is composed by *picking* from the pool. Ratings are the only team attribute that drives simulation, so there is no need for runtime team CRUD; this keeps the write surface small and every league reproducible from the same catalog. See §7.

---

## 2. Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.25 |
| HTTP framework | Gin |
| Database driver | pgx/v5 |
| Migrations | golang-migrate (applied automatically on startup) |
| Validation | go-playground/validator (via Gin binding tags) |
| Logging | zerolog |
| Tests | standard library `testing` + dockertest |
| API docs | hand-written OpenAPI 3 (`api/openapi.yaml`), rendered with Swagger UI at `/swagger` |
| Web UI | Embedded Vue 3 runtime + single-page UI served at `/` |
| Container | Docker + docker-compose; distroless runtime image |

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
  repository/postgres/           pgx implementations (+ dbtest/ harness)
  service/                       orchestration layer + recalc helper
  httpapi/                       Gin transport layer:
    router.go                      route registration
    *_handler.go                   thin handlers (league, match, team, prediction, docs, ui)
    dto.go                         request/response types
    errors.go                      domain error → HTTP mapping
web/
  index.html                     embedded Vue single-page UI (served at /)
database/
  schema.sql
  seed.sql                       8-team pool + a starter demo league
  queries.sql
  migrations/                    embedded; applied on startup
docs/
  DESIGN.md  PLAN.md  DEPLOYMENT.md
api/
  openapi.yaml                   embedded; served at /openapi.yaml, rendered at /swagger
  postman_collection.json
Dockerfile
docker-compose.yml
Makefile
.env.example
README.md
```

Handlers live directly in `internal/httpapi` as `*_handler.go` files (no separate `handler` package), and DTOs are a single flat `dto.go` (no `dto` subpackage). Cross-cutting concerns are minimal: the router uses Gin's built-in recovery; there is no separate `middleware` package.

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
    // Pure and deterministic: simulates the scheduled matches `simulations`
    // times from `seed`, keeping played results fixed. No I/O.
    Predict(teams []Team, matches []Match, simulations int, seed int64) []Prediction
}

type LeagueRepository interface { ... }
type TeamRepository interface { ... }
type MatchRepository interface { ... }
type StandingsSnapshotRepository interface { ... }
type MatchAuditRepository interface { ... }
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

Sort by the Premier League criteria: **points → goal difference → goals scored**. Teams still level are deemed to occupy the same position (PL settles title/relegation/qualification ties with a play-off, which is out of scope here); **team name** is appended only as a deterministic display order for level teams — not a ranking criterion. Implemented as a pure function over `[]Match`; no DB writes.

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

match_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  match_id BIGINT REFERENCES matches ON DELETE CASCADE,
  old_home_goals INT, old_away_goals INT,
  new_home_goals INT, new_away_goals INT,
  reason TEXT,
  changed_at TIMESTAMPTZ
)
```

Predictions are computed on demand and not persisted; there is no `prediction_runs` table.

`database/queries.sql` ships the non-trivial reads (standings aggregation, weekly match listing).

**Seed data.** `database/seed.sql` loads the fixed eight-team pool and one starter league ("Premier League": four teams, six weeks, twelve `SCHEDULED` matches, not yet played) so a fresh database opens onto real data instead of an empty page. Its fixtures are hand-written *static demo data* — a valid double round-robin, not a second implementation of the `FixtureGenerator`. Real leagues created through the API always use the Go generator. The block is idempotent (skipped if a league of that name already exists).

---

## 7. API surface

Responses are flat JSON. Errors use the shape `{ "error": { "code": "STRING", "message": "..." } }`.

### Core

| Method | Path | Purpose |
|---|---|---|
| POST   | `/api/v1/leagues` | Create league by picking teams from the pool (teams and seed optional) |
| GET    | `/api/v1/leagues` | List leagues |
| GET    | `/api/v1/leagues/{id}` | League summary |
| DELETE | `/api/v1/leagues/{id}` | Delete a league and all its data (204; cascades — see §8) |
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
| GET    | `/api/v1/teams` | List the fixed team pool |
| GET    | `/api/v1/leagues/{id}/teams` | Teams in a league, with ratings |
| PATCH  | `/api/v1/teams/{id}/ratings` | Update ratings (affects future matches only) |
| GET    | `/api/v1/leagues/{id}/standings/history` | All weekly snapshots |
| GET    | `/api/v1/matches/{id}/audit` | Edit log for a match |
| POST   | `/api/v1/leagues/{id}/recalculate` | Force re-derive standings |
| GET    | `/health`, `/ready` | Liveness + DB ping |
| GET    | `/`, `/swagger`, `/openapi.yaml` | Web UI, Swagger UI, OpenAPI contract |

The team pool is fixed (§1): there is deliberately **no** `POST /teams` or `DELETE /teams/{id}`. Teams are seeded once; leagues are formed by selecting from them.

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
- Deleting a league relies on `ON DELETE CASCADE`: a single `DELETE FROM leagues` removes its `league_teams` rows, `matches`, `standings_snapshots` (and their rows), and `match_audit_logs` atomically. The shared `teams` catalog is never touched — those rows have no cascade from leagues.

---

## 9. Testing strategy

### Unit
- **Fixture:** four teams produce six weeks and twelve matches; every pair plays twice with swapped home/away.
- **Simulation:** identical seed produces identical scores; stronger teams win significantly more across 10k runs.
- **Standings:** PL tie-breakers (GD, goals scored) and the name display-order fallback for level teams.
- **Prediction:** deterministic seed produces deterministic Monte Carlo output.
- **Service:** rating changes do not retroactively alter past matches; editing a result triggers recalculation.

### Integration (dockertest spins up Postgres)
- Full lifecycle: create → play four weeks → predictions present → play all → status `FINISHED`.
- Edit a week-1 result after week 4 → standings and later snapshots updated correctly.

### Manual / demo
- The Postman collection scripts the entire demo flow end-to-end.

---

## 10. Documentation

| File | Content |
|---|---|
| `README.md` | Overview, one-command run, demo curl flow, feature list |
| `docs/DESIGN.md` | This document |
| `docs/PREDICTION_ALGORITHM.md` | Poisson match model and Monte Carlo prediction math |
| `docs/DATABASE_SCHEMA.md` | ER diagram + per-table constraint rationale |
| `docs/RUNBOOK.md` | Operational troubleshooting |
| `docs/DEPLOYMENT.md` | Docker, env vars, deploy steps |
| `docs/PLAN.md` | Step-by-step delivery plan |
| `api/openapi.yaml` | Hand-written OpenAPI 3 contract; served at `/openapi.yaml`, rendered at `/swagger` |
| `api/postman_collection.json` | Demo collection |

---

## 11. Deployment

- `docker compose up` brings up the app and Postgres; the app applies migrations on startup. `make seed` loads the eight-team pool and a starter demo league (see §6).
- `Makefile` targets: `run`, `test`, `migrate-up`, `migrate-down`, `seed`, `docker-up`, `docker-down`, `vet`, `build`.
- `.env.example` documents `DATABASE_URL`, `PORT`, `LOG_LEVEL`.
- Active hosting: AWS EC2 serves the Go binary with `systemd`, with PostgreSQL
  installed on the same instance for the demo deployment. `docs/DEPLOYMENT.md`
  covers local Docker and EC2 update/restart steps.

---

## 12. Design patterns

The patterns below are the vocabulary the codebase uses. Each is mapped to the place it lives so a reader can find an example quickly.

### In use

| Pattern | Where | Purpose |
|---|---|---|
| Repository | `internal/domain` interfaces, `internal/repository/postgres` impls | Decouple services from persistence; allow fakes in tests |
| Dependency injection (composition root) | `cmd/server/main.go` | Single place where concrete implementations are wired |
| Strategy | `FixtureGenerator`, `MatchSimulator`, `StandingsCalculator`, `PredictionEngine` | Interface per algorithm; swappable for tests or future variants |
| Service / facade | `internal/service` | Orchestrate repos + algorithms; own transactions; expose one coherent operation per use case |
| Layered architecture | `httpapi (handlers) → service → repository` | Each layer talks only to the one directly below |
| Unit of Work | `pgx.Tx` opened in services, passed to repos | All-or-nothing multi-write operations |
| Pessimistic locking | `SELECT ... FOR UPDATE` on `leagues` row before mutation | Prevent races on `current_week` |
| Snapshot | `standings_snapshots` | Cached projection of derived state; rebuildable from `matches` |
| Audit log | `match_audit_logs` | Immutable history of mutable rows |
| Recovery middleware | `internal/httpapi/router.go` (`gin.Recovery`) | Turn panics into a 500 instead of crashing the process |
| DTO | `internal/httpapi/dto.go` | Wire format separate from domain entities |
| Struct composition | `Team` embeds `Rating` and `BaseModel` | Idiomatic Go reuse; explicit case requirement |
| Pure functions | `fixture`, `simulation`, `standings` | No I/O; trivially testable and deterministic |
| Seeded RNG | `league.random_seed` propagated to every randomized call | Reproducible simulations and tests |

### Available if needed later

- **Builder** — for assembling request defaults if `CreateLeague` grows variants.
- **Specification** — for composable filters if standings queries grow variants.
- **Observer** — could decouple snapshot capture from `LeagueService`; inline is simpler at this scale.
- **Command pattern** — if `play-all` ever needs to be queued and processed asynchronously.

---

## 13. Out of scope

- Authentication and authorization.
- Frontend (a small static viewer may be added as a follow-up).
- Real-time updates or websockets.
- Player-level modeling.
