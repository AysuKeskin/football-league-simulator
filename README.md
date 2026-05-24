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
make migrate-up               # apply database migrations
make seed                     # load default 4 teams
curl localhost:8080/health    # {"status":"ok"}
curl localhost:8080/ready     # {"status":"ok"} once postgres is reachable
make docker-down              # stop everything
```

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
  httpapi/              Gin router + handlers
docs/                   DESIGN.md, PLAN.md
```

Each subsequent step (see [`docs/PLAN.md`](docs/PLAN.md)) adds another internal package: `domain`, `fixture`, `simulation`, `standings`, `prediction`, `repository`, `service`, …

---

## Documentation

| Document | Purpose |
|---|---|
| [`docs/DESIGN.md`](docs/DESIGN.md) | Architecture, schema, API reference, design patterns |
| [`docs/PLAN.md`](docs/PLAN.md) | Step-by-step delivery plan |


