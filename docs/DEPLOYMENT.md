# Deployment

The service is a Go binary plus a Postgres database. The app applies its own
migrations on startup (see `RunMigrations` in
`internal/repository/postgres/migrate.go`), so a fresh database becomes
schema-ready on first boot.

The active live deployment is an AWS EC2 instance:

```txt
https://football-league-simulator.aysu-keskin.uk
```

It runs the Go server via `systemd` and uses PostgreSQL installed on the same
EC2 host. This keeps demo latency low because API requests no longer cross the
network to an external database. Nginx terminates HTTPS for the custom domain
and proxies traffic to the Go app on `127.0.0.1:8080`.

---

## Configuration

| Variable | Required | Notes |
|---|---|---|
| `DATABASE_URL` | yes | Postgres DSN consumed by pgx |
| `PORT` | no | defaults to `8080` |
| `LOG_LEVEL` | no | `debug` / `info` / `warn` / `error` (default `info`) |

Local Docker Compose uses `.env`. The EC2 deployment uses:

```txt
/home/ubuntu/fls.env
```

Current EC2 shape:

```txt
PORT=8080
LOG_LEVEL=info
DATABASE_URL=postgres://...@127.0.0.1:5432/fls?sslmode=disable
```

The `127.0.0.1` host is local to the EC2 machine, not the developer laptop.

---

## Local

```bash
make docker-up        # builds + starts app and Postgres; app auto-migrates on boot
make seed             # load the 8-team pool + a demo league
curl localhost:8080/health
open http://localhost:8080/swagger
make docker-down
```

---

## AWS EC2

### Live Checks

```bash
APP=https://football-league-simulator.aysu-keskin.uk
curl -s $APP/health
curl -s $APP/ready
open  $APP/swagger
```

### Update The Running App

Build a Linux binary locally:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -o bin/server-linux-amd64 \
  ./cmd/server
```

Upload it:

```bash
scp -o IdentitiesOnly=yes -i ./fls-key.pem \
  bin/server-linux-amd64 \
  ubuntu@63.177.83.131:/home/ubuntu/server.new
```

Swap the binary and restart the service:

```bash
ssh -o IdentitiesOnly=yes -i ./fls-key.pem ubuntu@63.177.83.131 \
  "chmod +x /home/ubuntu/server.new && \
   sudo systemctl stop fls && \
   mv /home/ubuntu/server.new /home/ubuntu/server && \
   sudo systemctl start fls && \
   sudo systemctl is-active fls"
```

### Inspect Logs

```bash
ssh -o IdentitiesOnly=yes -i ./fls-key.pem ubuntu@63.177.83.131
sudo systemctl status fls
journalctl -u fls -f
```

Expected startup lines:

```txt
INF database migrations applied
INF postgres pool opened
INF http server listening port=8080
```

### Seed The EC2 Database

The active EC2 database is local to the instance. To reseed it:

```bash
scp -o IdentitiesOnly=yes -i ./fls-key.pem \
  database/seed.sql \
  ubuntu@63.177.83.131:/home/ubuntu/seed.sql

ssh -o IdentitiesOnly=yes -i ./fls-key.pem ubuntu@63.177.83.131 \
  "PGPASSWORD=<db-password> psql -h 127.0.0.1 -U fls -d fls -f /home/ubuntu/seed.sql"
```

`seed.sql` is idempotent for the team pool and skips the starter league if one
already exists.

### HTTPS / Nginx

The public domain is served by Nginx:

```txt
https://football-league-simulator.aysu-keskin.uk
```

Nginx proxies to the Go app:

```txt
127.0.0.1:8080
```

Useful commands:

```bash
sudo nginx -t
sudo systemctl status nginx
sudo systemctl reload nginx
sudo certbot certificates
systemctl list-timers certbot.timer --no-pager
```

Certificates are managed by Certbot / Let's Encrypt and renew automatically.

### Notes

- The database is not AWS RDS. It is PostgreSQL on the EC2 instance.
- Do not terminate the EC2 instance unless losing the demo database is
  acceptable. Stop/start keeps the disk, but the public IP may change.
- `fls-key.pem` is ignored by git and must not be committed.

---

## Troubleshooting

| Symptom | Check |
|---|---|
| App crashes before listening | `journalctl -u fls -n 50 --no-pager` |
| `/ready` returns 503 | local Postgres is running: `sudo systemctl status postgresql` |
| UI is up but empty | EC2 database may need `seed.sql` |
| 404 on `/swagger` or `/openapi.yaml` | old binary is running; rebuild, upload, and restart `fls` |
| SSH hangs or times out | instance may be overloaded; reboot from the EC2 console |
