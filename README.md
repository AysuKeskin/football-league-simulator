# Football League Simulator

A Go backend that simulates a football league with probabilistic match results, Premier League scoring rules, and Monte Carlo championship predictions. Built as the Insider backend case.

> **Status:** scaffold (Step 1 of [`docs/PLAN.md`](docs/PLAN.md)). Only `/health` is wired so far; the simulation, database, and full API land in later steps.

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
make seed                     # load default 4 teams
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
make seed          # load default 4 teams
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
database/               schema.sql, seed.sql, queries.sql, migrations/
docs/                   DESIGN.md, PLAN.md
```

---

## API

With the stack running:

- **Swagger UI** — [`http://localhost:8080/swagger`](http://localhost:8080/swagger): every endpoint, grouped by tag, with example payloads and "Try it out".
- **OpenAPI spec** — `http://localhost:8080/openapi.yaml` (also committed at [`api/openapi.yaml`](api/openapi.yaml)).
- **Postman** — import [`api/postman_collection.json`](api/postman_collection.json), set the `baseUrl` variable, and run the requests top-to-bottom (creating a league captures `leagueId` for the rest of the flow).

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

## Documentation

| Document | Purpose |
|---|---|
| [`docs/DESIGN.md`](docs/DESIGN.md) | Architecture, schema, API reference, design patterns |
| [`docs/PLAN.md`](docs/PLAN.md) | Step-by-step delivery plan |
| [`api/openapi.yaml`](api/openapi.yaml) | OpenAPI 3.0 contract (served at `/openapi.yaml`, rendered at `/swagger`) |
| [`api/postman_collection.json`](api/postman_collection.json) | Click-through demo collection |


