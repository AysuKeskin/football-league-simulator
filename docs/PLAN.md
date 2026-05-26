# Implementation Plan

Step-by-step delivery plan. Each step is sized to ship independently, with its own scope, deliverables, and acceptance criteria. Steps build on each other; nothing in a later step is required by an earlier one.

See [`DESIGN.md`](./DESIGN.md) for architecture, schema, and API reference.

---

## Step 1 — Project scaffold

**Goal:** Empty Go service runs locally and inside Docker.

**Scope**
- `go mod init`, base dependencies (Gin, zerolog, pgx, validator).
- `cmd/server/main.go` boots Gin on `PORT`.
- `internal/config` loads env via a single struct.
- `internal/httpapi/router.go` registers `/health`.
- `Dockerfile`, `docker-compose.yml` (app + Postgres), `.env.example`, `Makefile` (`run`, `test`, `docker-up`, `docker-down`).
- CI-ready `go test ./...` passes (no tests yet).

**Acceptance**
- `docker compose up` boots app + Postgres without errors.
- `curl localhost:8080/health` → `{"status":"ok"}`.

---

## Step 2 — Database foundation

**Goal:** Schema, migrations, and seed data exist; app can connect.

**Scope**
- `database/migrations/` with golang-migrate up/down for every table in `DESIGN.md §6`.
- `database/schema.sql` (deliverable copy) and `database/seed.sql` (a fixed 8-team pool with sensible ratings; leagues are created by picking from it).
- `database/queries.sql` placeholder, filled per step.
- `internal/repository/postgres/db.go` opens a `pgxpool`.
- `/ready` endpoint pings the DB.

**Acceptance**
- `make migrate` applies cleanly to a fresh DB.
- `curl localhost:8080/ready` returns 200 when DB is up, 503 when down.
- `psql \dt` shows every table.

---

## Step 3 — Domain types and repository interfaces

**Goal:** Core types and interfaces compile; no business logic yet.

**Scope**
- `internal/domain/` with entities and enums from `DESIGN.md §4.1`.
- `internal/domain/repository.go` with every repository interface signature.
- `internal/domain/errors.go` with sentinel errors (`ErrNotFound`, `ErrConflict`, `ErrInvalidInput`).
- No implementations — these arrive in later steps.

**Acceptance**
- `go build ./...` and `go vet ./...` pass.
- Domain package has zero external dependencies.

---

## Step 4 — Fixture generator

**Goal:** Pure function that produces a double round-robin schedule.

**Scope**
- `internal/fixture/generator.go` implementing `FixtureGenerator` via the circle method.
- Validation: `n >= 4` and even.
- Unit tests: for n=4 → 6 weeks × 2 matches; every pair plays twice with swapped home/away; deterministic per seed.

**Acceptance**
- `go test ./internal/fixture` passes with >90% coverage of the package.

---

## Step 5 — Match simulator

**Goal:** Probabilistic match engine with reproducible output.

**Scope**
- `internal/simulation/poisson.go` implementing `MatchSimulator` per `DESIGN.md §5.2`.
- Goals clamped to `[0, 9]`.
- Unit tests: identical seed produces identical results; over 10k runs a much stronger team wins significantly more than it loses; home advantage measurable in aggregate.

**Acceptance**
- `go test ./internal/simulation` passes.

---

## Step 6 — Standings calculator

**Goal:** Pure function deriving the league table from matches.

**Scope**
- `internal/standings/calculator.go` implementing `StandingsCalculator`.
- Premier League tie-breakers (points → GD → goals scored); team name is a deterministic display order for level teams, not a ranking criterion.
- Unit tests covering every tie-breaker independently.

**Acceptance**
- `go test ./internal/standings` passes.

---

## Step 7 — Postgres repositories

**Goal:** Concrete implementations for league, team, match, snapshot repositories.

**Scope**
- `internal/repository/postgres/` files for `league_repo.go`, `team_repo.go`, `match_repo.go`, `standings_snapshot_repo.go`.
- Transaction helper that accepts `pgx.Tx` so services can compose multi-write operations.
- Integration tests using dockertest hitting real Postgres.

**Acceptance**
- All repository methods covered by integration tests.
- `make test` (which spins up Postgres via dockertest) is green.

---

## Step 8 — Core league lifecycle endpoints

**Goal:** The happy path required by the case brief works end-to-end.

**Scope**
- `internal/service/league_service.go` orchestrating: create league → generate fixtures → persist.
- Handlers and routes:
  - `POST /api/v1/leagues`
  - `GET /api/v1/leagues`
  - `GET /api/v1/leagues/{id}`
  - `POST /api/v1/leagues/{id}/play-week`
  - `POST /api/v1/leagues/{id}/play-all`
  - `POST /api/v1/leagues/{id}/reset`
  - `GET /api/v1/leagues/{id}/standings`
  - `GET /api/v1/leagues/{id}/fixtures`
  - `GET /api/v1/leagues/{id}/weeks/{w}`
- `SELECT ... FOR UPDATE` on the league row during state mutations.
- Snapshot persisted after each week.
- Integration test: full lifecycle (create → play-all → status `FINISHED`).

**Acceptance**
- Postman demo: create league, play week-by-week, view standings, play all, view final table.

---

## Step 9 — Edit match result + audit + recalculation

**Goal:** Second extra beyond the brief — editing a played result rewrites downstream state.

**Scope**
- `internal/service/match_service.go` with `UpdateResult`.
- `PUT /api/v1/matches/{id}` handler.
- `match_audit_logs` insert in the same transaction.
- Re-derive standings snapshots for weeks `N..currentWeek`.
- `GET /api/v1/matches/{id}/audit`.
- `POST /api/v1/leagues/{id}/recalculate` for manual recompute.
- Integration test: edit a week-1 result after week 4 and assert all later snapshots change.

**Acceptance**
- Editing a match never corrupts standings; audit log records old/new values and reason.

---

## Step 10 — Monte Carlo predictions

**Goal:** Championship probabilities surfaced from week 4 onward.

**Scope**
- `internal/prediction/monte_carlo.go` implementing `PredictionEngine` per `DESIGN.md §5.4`.
- `GET /api/v1/leagues/{id}/predictions?simulations=N` (default 10000, capped).
- Predictions are computed on the fly per request; no persistence.
- Gate: returns 409 with explanatory message when `currentWeek < 4`.
- When league is `FINISHED`, return actual final standings labeled as such.
- Unit test: deterministic seed produces deterministic predictions.

**Acceptance**
- Postman: at week 4, predictions endpoint returns per-team championship %, average position, most likely position.

---

## Step 11 — Team rating management

**Goal:** Ratings editable; future matches affected, past untouched.

**Scope**
- `PATCH /api/v1/teams/{id}/ratings`.
- `GET /api/v1/leagues/{id}/teams` for convenience.
- Unit test: rating change followed by re-prediction shifts probabilities; played matches remain unchanged.

**Acceptance**
- A rating change visibly shifts subsequent simulations and predictions while leaving played matches intact.

---

## Step 12 — Standings history endpoint

**Goal:** Expose the snapshots already being captured.

**Scope**
- `GET /api/v1/leagues/{id}/standings/history` returning all snapshots ordered by week.
- Includes captured-at timestamp so consumers can detect post-edit rewrites.

**Acceptance**
- Endpoint returns one snapshot per played week; after a match edit, the affected snapshots show fresh `captured_at`.

---

## Step 13 — OpenAPI, Swagger UI, Postman collection

**Goal:** Self-serve API documentation.

**Scope**
- swag annotations on every handler.
- `make swag` regenerates `api/openapi.yaml`.
- Swagger UI mounted at `/swagger/index.html`.
- `api/postman_collection.json` scripting the demo flow from `README.md`.

**Acceptance**
- Reviewer can import the Postman collection and run the full demo without reading code.
- `/swagger/index.html` lists every endpoint with example payloads.

---

## Step 14 — Documentation pass

**Goal:** Polished docs aligned with shipped code.

**Scope**
- `README.md`: overview, one-command run, demo curl flow, feature checklist.
- `docs/API_DOCUMENTATION.md`: endpoint-by-endpoint examples.
- `docs/PREDICTION_ALGORITHM.md`: Poisson + Monte Carlo math.
- `docs/DATABASE_SCHEMA.md`: ERD + per-table descriptions.
- `docs/DEPLOYMENT.md`: local + remote.
- `docs/RUNBOOK.md`: common operational issues.

**Acceptance**
- A reviewer with zero prior context can clone, run, and exercise the project in under five minutes using only the README.

---

## Step 15 — Deployment

**Goal:** Live URL.

**Scope**
- AWS EC2 deployment guide in `docs/DEPLOYMENT.md`.
- PostgreSQL running on the EC2 instance for low-latency demo data.
- Migrations applied automatically on app startup.
- Health and readiness verified against the live EC2 URL.
- Live URL added to README.

**Acceptance**
- Public URL responds to `/health`, `/ready`, and the full demo flow.

---

## Step 16 — Web UI

**Goal:** A simple browser UI matching the screen mockup in the case brief — served by the app itself.

**Scope**
- Vue 3 runtime embedded with the app, no frontend build step; a single
  `web/index.html` plus local runtime asset, embedded (`//go:embed`) and served
  by the Go server at `/` (same pattern as `/swagger`).
- Premier League visual style: gradient header, white table card, position accent bar, `Pos/Team/Pl/W/D/L/GF/GA/GD/Pts` columns.
- Full simulator screen: standings table + controls (create / play-week / play-all / reset / delete) + current week's match results + championship predictions panel (from week 4).
- Create picks teams from the fixed pool (any even count ≥ 4); Teams tab edits ratings; Fixtures tab edits results with an audit trail; History tab shows per-week snapshots.
- Consumes the existing JSON API with same-origin `fetch`.

**Acceptance**
- Opening the live URL renders the UI; create → play → predictions → final table works end-to-end against the deployed API.
