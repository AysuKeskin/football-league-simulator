# Football League Simulator

A Go backend that simulates a football league with probabilistic match results, Premier League scoring rules, and Monte Carlo championship predictions. Built as the Insider backend case.

> **Live:** [https://football-league-simulator.aysu-keskin.uk](https://football-league-simulator.aysu-keskin.uk) — web UI at `/`, interactive API docs at [`/swagger`](https://football-league-simulator.aysu-keskin.uk/swagger)
>
> **Docs:** [Database schema](docs/DATABASE_SCHEMA.md) · [API reference](https://football-league-simulator.aysu-keskin.uk/swagger) ([`api/openapi.yaml`](api/openapi.yaml)) · [Prediction algorithm](docs/PREDICTION_ALGORITHM.md)

---

## Features

- **Probabilistic simulation.** Match scores come from a Poisson model driven by each team's attack/midfield/defense ratings plus a home-advantage term — not a coin flip.
- **Monte Carlo predictions.** From week 4 on, championship odds and expected finishing positions are estimated by simulating the remaining fixtures thousands of times; a finished league reports the actual champion.
- **Deterministic & reproducible.** Every league carries a `random_seed`; the same seed replays the exact same season, which also makes the tests stable.
- **Premier League standings.** Ranked by points → goal difference → goals scored, with team name only as a stable display tiebreak.
- **Match editing with an audit trail.** Correct any played result; standings snapshots from that week forward are rebuilt automatically, and every edit is recorded (old → new, with a reason).
- **Standings history.** Per-week table snapshots, so you can see how the league evolved.
- **Fixed team pool.** A seeded catalog of clubs; create a league by picking any even number (≥ 4) of them and tuning their ratings.
- **Delete a league.** Removes it and all its data in one cascade; the shared team pool is untouched.
- **Batteries included.** Auto-applied migrations on startup, a seeded starter league, an embedded Vue web UI at `/`, and Swagger UI at `/swagger`.

---

## Quick start

### Run locally with Go

Requires Go 1.25+.

```bash
go run ./cmd/server
```

Then in another shell:

```bash
curl localhost:8080/health
# {"status":"ok"}
```

### Run with Docker (recommended)

Requires Docker and Docker Compose.

```bash
cp .env.example .env          # optional; defaults are sensible
make docker-up                # builds and starts app + postgres
                              # (the app applies migrations on startup)
make seed                     # load the 8-team pool + a demo league
curl localhost:8080/health    # {"status":"ok"}
curl localhost:8080/ready     # {"status":"ok"} once postgres is reachable
make docker-down              # stop everything
```

> Migrations run automatically when the server boots, so a fresh database
> is schema-ready on first start. `make migrate-up` / `migrate-down` remain
> for manual control during development.

---

## Make targets

```bash
make help          # list all targets
make run           # run the API locally
make test          # run all unit tests (quiet)
make test-v        # verbose: per-test output
make test-fresh    # ignore the test cache
make test-docker   # run tests in a Go container (no local Go needed)
make vet           # static analysis
make build         # compile to ./bin/server
make docker-up     # docker compose up --build -d
make docker-down   # docker compose down
make docker-logs   # tail compose logs
make migrate-up    # apply pending migrations (no local migrate install needed)
make migrate-down  # roll back one migration
make seed          # load the 8-team pool + a demo league
```

> No Go installed locally? `make test-docker` spins up a `golang:1.25-alpine` container, runs the full test suite, and exits. Only Docker is required.

---

## Configuration

All settings come from the environment. Defaults live in [`internal/config`](internal/config/config.go).

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port the API binds to |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `DATABASE_URL` | _(required)_ | `postgres://user:pass@host:port/db?sslmode=disable` |
| `POSTGRES_USER` | `fls` | Postgres user (compose only) |
| `POSTGRES_PASSWORD` | `fls` | Postgres password (compose only) |
| `POSTGRES_DB` | `fls` | Postgres database (compose only) |

`.env` is gitignored. Use `.env.example` as a template.

---

## Project layout

```
cmd/server/             composition root
internal/
  config/               env loading + validation
  domain/               entities + interfaces (no external deps)
  fixture/              double round-robin generator (pure)
  simulation/           Poisson match simulator (pure)
  standings/            league-table calculator (pure)
  prediction/           Monte Carlo predictor (pure)
  repository/postgres/  pgx repositories + Transactor
  service/              league / match / team / prediction services
  httpapi/              Gin router, handlers, DTOs, error mapping
api/                    openapi.yaml, postman_collection.json
web/                    embedded Vue single-page UI assets
database/               schema.sql, seed.sql, queries.sql, migrations/
docs/                   DESIGN.md, PLAN.md, DEPLOYMENT.md
```

---

## Web UI

A single-page browser UI (Vue 3 runtime embedded in the Go binary, no frontend
build step) lives at the site root:

- **Local:** [`http://localhost:8080/`](http://localhost:8080/) · **Live:** [`https://football-league-simulator.aysu-keskin.uk/`](https://football-league-simulator.aysu-keskin.uk/)
- Premier-League-styled standings table, plus controls to **Create league → Play next week / Play all → Reset**, the current week's results, and the championship predictions panel (from week 4).
- **Tabs:** Table · Fixtures (edit any played result, with its audit trail) · History (week-by-week snapshots) · Teams (the fixed team pool — edit ratings).
- **Create** lets you name the league and pick which teams from the pool play (any even number ≥ 4).
- Reset replays the season from scratch; **Delete** removes a league and all its data (the team pool is untouched).
- It calls the same JSON API; nothing extra to run.

> The team pool and a starter "Premier League" come from the seed. Run
> `make seed` locally; the AWS demo database has already been seeded.

---

## API

Try it on the live deployment, or locally with the stack running:

- **Swagger UI** — live at [`football-league-simulator.aysu-keskin.uk/swagger`](https://football-league-simulator.aysu-keskin.uk/swagger) (or [`http://localhost:8080/swagger`](http://localhost:8080/swagger)): every endpoint, grouped by tag, with example payloads and "Try it out".
- **OpenAPI spec** — live at [`football-league-simulator.aysu-keskin.uk/openapi.yaml`](https://football-league-simulator.aysu-keskin.uk/openapi.yaml) (or `http://localhost:8080/openapi.yaml`); also committed at [`api/openapi.yaml`](api/openapi.yaml).
- **Postman** — import [`api/postman_collection.json`](api/postman_collection.json) and run the requests top-to-bottom. `baseUrl` defaults to `http://localhost:8080` (override only to target another host); `leagueId` / `teamId` / `matchId` are captured automatically as you go, so there's nothing to fill in by hand.

### Demo flow (curl)

```bash
BASE=http://localhost:8080

# 1. Create a league (default teams, fixed seed for reproducibility)
LID=$(curl -s -XPOST $BASE/api/v1/leagues -H 'Content-Type: application/json' \
  -d '{"name":"Demo","seed":42}' | jq .id)

# 2. Play to week 4, then look at predictions
for i in 1 2 3 4; do curl -s -XPOST $BASE/api/v1/leagues/$LID/play-week >/dev/null; done
curl -s "$BASE/api/v1/leagues/$LID/predictions" | jq

# 3. Finish the season and read the final table
curl -s -XPOST $BASE/api/v1/leagues/$LID/play-all >/dev/null
curl -s $BASE/api/v1/leagues/$LID/standings | jq

# 4. Correct a result and inspect the audit trail
MID=$(curl -s $BASE/api/v1/leagues/$LID/fixtures | jq '.weeks[0].matches[0].id')
curl -s -XPUT $BASE/api/v1/matches/$MID -H 'Content-Type: application/json' \
  -d '{"homeGoals":3,"awayGoals":1,"reason":"review"}' >/dev/null
curl -s $BASE/api/v1/matches/$MID/audit | jq
```

---

## Deployment

The live deployment currently runs on AWS EC2:

- Go binary served by `systemd`
- PostgreSQL installed on the same EC2 instance for low-latency demo data
- migrations applied automatically on app startup
- Nginx reverse proxy with Let's Encrypt HTTPS for the custom domain

> **Live:** https://football-league-simulator.aysu-keskin.uk — try [`/swagger`](https://football-league-simulator.aysu-keskin.uk/swagger)

See [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) for update/restart commands.

---

## Documentation

| Document | Purpose |
|---|---|
| [`docs/DESIGN.md`](docs/DESIGN.md) | Architecture, schema, API reference, design patterns |
| [`docs/DATABASE_SCHEMA.md`](docs/DATABASE_SCHEMA.md) | ER diagram + per-table constraint rationale |
| [`docs/PREDICTION_ALGORITHM.md`](docs/PREDICTION_ALGORITHM.md) | Poisson match model + Monte Carlo prediction math |
| [`docs/RUNBOOK.md`](docs/RUNBOOK.md) | Operational troubleshooting (startup, probes, seeding, common errors) |
| [`docs/PLAN.md`](docs/PLAN.md) | Step-by-step delivery plan |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Local + AWS EC2 deployment guide |
| [`api/openapi.yaml`](api/openapi.yaml) | OpenAPI 3.0 contract (served at `/openapi.yaml`, rendered at `/swagger`) |
| [`api/postman_collection.json`](api/postman_collection.json) | Click-through demo collection |
