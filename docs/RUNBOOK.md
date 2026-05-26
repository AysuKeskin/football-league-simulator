# Runbook

Operational guide for running and troubleshooting the simulator. For first-time
setup and deployment see [DEPLOYMENT.md](DEPLOYMENT.md); for env vars see the
[README configuration table](../README.md#configuration).

---

## Startup

The server applies its own migrations on boot, then opens the pool and listens.
A healthy start logs three lines (zerolog, structured):

```
INF database migrations applied
INF postgres pool opened
INF http server listening port=8080
```

If you don't see all three, the process exited early — read the last log line.

| Symptom | Likely cause | Action |
|---|---|---|
| Exit before "migrations applied" | bad/unreachable `DATABASE_URL` | check the DSN; confirm Postgres is up |
| Exit at migrations | dirty migration state / hand-edited schema | inspect with `make migrate-version`; the DB and `schema.sql` must match |
| Starts then exits | port already in use | free `PORT` (default 8080) or set another |

---

## Health & readiness

Two probes, different jobs:

- **`GET /health`** — liveness. 200 means the process is up. No dependency check.
- **`GET /ready`** — readiness. Pings the database; **200** reachable, **503**
  not. A 503 logs `WARN readiness check failed` (the raw DB error is logged,
  never returned in the body).

A 503 on `/ready` while `/health` is 200 ⇒ the app is up but the database is
unreachable: verify `DATABASE_URL`, network/SSL mode, and that Postgres accepts
connections.

For the live AWS EC2 deployment, `/ready` checks the PostgreSQL server running
on the same instance (`127.0.0.1:5432`).

---

## Seeding

`make seed` loads the fixed 8-team pool **and** a fresh `NOT_STARTED` demo
league ("Premier League"). It is idempotent — re-running is safe (teams use
`ON CONFLICT DO NOTHING`; the demo league is skipped if one of that name
already exists).

- **Order matters:** run it *after* the app has applied migrations (i.e. after
  the server has started once), otherwise the target tables don't exist yet.
- **AWS demo database:** copy `database/seed.sql` to the EC2 instance and run
  it with `psql` against `127.0.0.1` (see [DEPLOYMENT.md](DEPLOYMENT.md)).
- **Managed remote database:** point `MIGRATE_DATABASE_URL` at the deployed DB
  and run `make seed`.

---

## Common request-time issues

| Response | Meaning | Fix |
|---|---|---|
| `400` on `POST /api/v1/leagues` | empty team catalog (unseeded DB) or team count not even / `< 4` | run `make seed`; pick an even number (≥4) of `teamIds` |
| `409` on `GET …/predictions` | league is before week 4 | play to week 4 first (predictions gate at week 4) |
| `409` on `PUT /api/v1/matches/{id}` | editing a `SCHEDULED` match | only **played** results are editable |
| `409` on `play-week` / `play-all` | league already `FINISHED` | reset it first, or delete it |
| `404` on any `:id` route | league/match doesn't exist | check the id; it may have been deleted |

All errors share the envelope `{ "error": { "code", "message" } }`.

---

## Managing state

- **Reset a league** — `POST /api/v1/leagues/{id}/reset` clears results but
  keeps fixtures; replaying with the same seed reproduces the identical season.
- **Recalculate** — `POST /api/v1/leagues/{id}/recalculate` rebuilds all
  standings snapshots from `matches` if a cache ever looks stale (snapshots are
  derived state and always rebuildable).
- **Delete a league** — `DELETE /api/v1/leagues/{id}` removes it and all its
  data (fixtures, snapshots, audit logs) in one cascade; the team pool is
  untouched.
- **Restore the demo league** — if you deleted it, re-run `make seed`.

---

## Logs

- Tail the compose stack locally: `make docker-logs`.
- For the live EC2 service (`journalctl -u fls` over SSH), see [DEPLOYMENT.md](DEPLOYMENT.md).
- Levels: `debug` (verbose flow, off by default) · `info` (state transitions:
  league created, week played, match edited) · `warn` (recoverable, e.g. a
  `/ready` ping failure) · `error` (unexpected failures, with the wrapped
  error). Set verbosity with `LOG_LEVEL`.
- Lines are structured with context fields (e.g. `league_id`, `from_week`,
  `to_week`) — grep on those rather than free text.

---

## Live deployment

The app runs on AWS EC2 behind Nginx at
`https://football-league-simulator.aysu-keskin.uk`. The server layout (binary,
`fls.service`, env file), SSH access, binary updates, EC2 database reseeding,
and Nginx/Certbot are all covered in [DEPLOYMENT.md](DEPLOYMENT.md) — this
runbook stays focused on the running app's behaviour.
