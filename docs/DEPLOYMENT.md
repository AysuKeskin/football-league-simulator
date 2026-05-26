# Deployment

The service is a single stateless Go binary plus a Postgres database.
It applies its own migrations on startup (see `RunMigrations` in
`internal/repository/postgres/migrate.go`), so a fresh database becomes
schema-ready on first boot — there is no separate migrate step to run.

The only required configuration is `DATABASE_URL`.

| Variable | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | `postgres://user:pass@host:port/db?sslmode=require` |
| `PORT` | no | defaults to `8080` |
| `LOG_LEVEL` | no | `debug` / `info` / `warn` / `error` (default `info`) |

---

## Local (Docker Compose)

```bash
make docker-up        # builds + starts app and Postgres; app auto-migrates on boot
make seed             # load the 8-team pool + a demo league
curl localhost:8080/health
open http://localhost:8080/swagger
make docker-down
```

---

## Remote (Fly.io + managed Postgres)

The app image is host-agnostic; these steps use Fly.io for the app and an
external managed Postgres (e.g. [Neon](https://neon.tech) free tier). Any
managed Postgres works — only the `DATABASE_URL` changes.

### 1. Prerequisites

```bash
# install flyctl: https://fly.io/docs/flyctl/install/
fly auth login
```

### 2. Provision a Postgres database

Create a free database on Neon (or Supabase) and copy its connection
string. Make sure it requires TLS:

```
postgres://USER:PASSWORD@HOST/DBNAME?sslmode=require
```

### 3. Create the Fly app (without deploying yet)

`fly.toml` is already in the repo; `--no-deploy` just registers the app.

```bash
fly launch --no-deploy
# accept the existing fly.toml; pick a unique app name + region if prompted
```

### 4. Set the database secret

```bash
fly secrets set DATABASE_URL="postgres://USER:PASSWORD@HOST/DBNAME?sslmode=require"
```

Secrets are encrypted and injected as env vars at runtime — never commit
the connection string.

### 5. Deploy

```bash
fly deploy
```

On boot the app applies migrations, opens the pool, then serves. Watch:

```bash
fly logs       # expect "database migrations applied" then "http server listening"
```

### 6. Verify

```bash
APP=https://football-league-simulator.fly.dev
curl -s $APP/health      # {"status":"ok"}
curl -s $APP/ready       # {"status":"ok"} (DB reachable)
open  $APP/swagger       # interactive API docs
```

Then run the demo flow from the README (or import
`api/postman_collection.json`, set `baseUrl` to `$APP`).

### Notes

- **Cold starts:** `min_machines_running = 0` lets the machine scale to
  zero when idle; the first request after idle pays a few seconds of cold
  start (boot + idempotent migration). Set it to `1` in `fly.toml` for an
  always-on demo.
- **Migrations:** applied automatically and idempotently on every boot
  (golang-migrate takes a DB advisory lock, so concurrent boots are safe).
  To inspect/roll back manually against the remote DB:
  `make migrate-version` / `make migrate-down` with
  `MIGRATE_DATABASE_URL` pointed at the remote connection string.
- **Seeding:** the default teams are not seeded automatically in
  production. Seed once after the first deploy if you want them:
  `MIGRATE_DATABASE_URL=<remote-url> make seed` (or `psql ... -f database/seed.sql`).

---

## Troubleshooting

| Symptom | Check |
|---|---|
| App crashes on boot with a migration error | `DATABASE_URL` reachable and includes `?sslmode=require`; `fly logs` |
| `/ready` returns 503 | database unreachable — verify the secret and that the DB allows Fly's egress |
| 404 on `/swagger` or `/openapi.yaml` | old image — redeploy (`fly deploy`); the spec is embedded at build time |
| Cold-start latency on first hit | expected with scale-to-zero; bump `min_machines_running` |
