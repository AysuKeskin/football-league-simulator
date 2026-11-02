# Football League Simulator

A Go backend that simulates a football league with probabilistic match results, Premier League scoring rules, and Monte Carlo championship predictions.

## ▶️ [Watch the demo video](https://youtu.be/g4DMZ8TcGqw)

[![Football League Simulator demo video](https://img.youtube.com/vi/g4DMZ8TcGqw/hqdefault.jpg)](https://youtu.be/g4DMZ8TcGqw)

> **Website:** [https://football-league-simulator.aysu-keskin.uk](https://football-league-simulator.aysu-keskin.uk)
>
> ❗ The website is temporarily suspended in order to prevent server costs.

---

## Features

- **Match simulation:** a Poisson model uses team ratings and home advantage to generate scores.
- **Championship predictions:** from week 4, Monte Carlo simulations estimate title odds and final positions.
- **Reproducible seasons:** the same seed replays the same season.
- **Premier League standings:** ranked by points, goal difference, then goals scored.
- **Match editing:** changing a result rebuilds later standings and records the edit in an audit log.
- **Standings history:** see the table as it stood after each week.
- **Team pool:** create a league from any even number (at least 4) of seeded teams, and edit their ratings.
- **Self-contained:** migrations run on startup, and the web UI and Swagger UI are embedded in the binary.

---

## Quick start

Requires Docker and Docker Compose.

```bash
make docker-up    # build and start the app and Postgres
make seed         # load the 8-team pool and a demo league
```

Then open:

- **Web UI:** [http://localhost:8080](http://localhost:8080)
- **Swagger UI:** [http://localhost:8080/swagger](http://localhost:8080/swagger)

Run `make docker-down` to stop everything.

### Run with Go

Requires Go 1.25+ and a running Postgres. To use the one from Docker Compose:

```bash
docker compose up -d postgres
DATABASE_URL='postgres://fls:fls@localhost:5432/fls?sslmode=disable' go run ./cmd/server
```

### Common commands

```bash
make help    # list all targets
make test    # run all tests
make vet     # static analysis
make build   # compile to ./bin/server
```

---

## Configuration

Settings come from environment variables. Docker Compose provides working defaults, or you can copy `.env.example` to `.env` to change them.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | Postgres connection string |
| `PORT` | `8080` | HTTP port |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

---

## Project layout

```
cmd/server/             application entry point
internal/
  config/               environment configuration
  domain/               entities and interfaces
  fixture/              fixture generator
  simulation/           Poisson match simulator
  standings/            league table calculator
  prediction/           Monte Carlo predictor
  repository/postgres/  database access
  service/              business logic and transactions
  httpapi/              routes, handlers, and error mapping
api/                    OpenAPI spec and Postman collection
web/                    embedded Vue UI
database/               schema, seed data, queries, and migrations
docs/                   project documentation
```

---

## API

- **Swagger UI:** [http://localhost:8080/swagger](http://localhost:8080/swagger)
- **OpenAPI spec:** [`api/openapi.yaml`](api/openapi.yaml)
- **Postman:** import [`api/postman_collection.json`](api/postman_collection.json) and run the requests from top to bottom.

### Demo flow

Requires `jq`.

```bash
BASE=http://localhost:8080

# 1. Create a league with a fixed seed
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
| [`docs/DESIGN.md`](docs/DESIGN.md) | Architecture, schema, and API reference |
| [`docs/DATABASE_SCHEMA.md`](docs/DATABASE_SCHEMA.md) | Database diagram and table details |
| [`docs/PREDICTION_ALGORITHM.md`](docs/PREDICTION_ALGORITHM.md) | Match model and prediction math |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Local and AWS EC2 deployment |
| [`docs/RUNBOOK.md`](docs/RUNBOOK.md) | Troubleshooting |
| [`docs/PLAN.md`](docs/PLAN.md) | Delivery plan |

---

## AI Usage

I built this project myself and used Claude Code and ChatGPT/Codex as coding assistants to speed up the work.
