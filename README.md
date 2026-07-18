# linka.looks-metric

Go backend for LINKa Looks activation and telemetry.

## Runtime

- `POST /requestActivation` sends an activation code.
- `POST /activate` exchanges email and code for a PC hash.
- `POST /registerEvent` stores telemetry only with the current versioned consent marker. Legacy requests without it return success as a no-op. New event rows always have `content = NULL`.
- `GET /stats` renders server-side statistics. Protect this path with nginx Basic Auth.
- `GET /healthz` returns service health.

## Local development

```sh
cp .env.example .env
MAIL_DRY_RUN=true DATABASE_URL=data/metric.db go run ./cmd/metric-api
```

## Docker Compose

```sh
cp .env.example .env
docker compose up -d --build
```

The compose file binds the service to `127.0.0.1:30812` on the host. Public traffic should go through nginx.

## Nginx stats protection

```nginx
location /stats {
  auth_basic "Metric stats";
  auth_basic_user_file /etc/nginx/.htpasswd-metric;
  proxy_pass http://127.0.0.1:30812;
}
```

## Production database

The first production start runs compatible SQLite migrations and adds indexes. Back up the existing database before switching from Node to Go.

## Sanitize legacy event history

This maintenance command is intentionally separate from application startup. Stop the metric service before applying it.

Count rows without changing data:

```sh
go run ./cmd/sanitize-history --database data/metric.db
```

Create a verified SQLite backup and then physically remove legacy `Event.content`:

```sh
go run ./cmd/sanitize-history \
  --database data/metric.db \
  --backup data/metric-before-sanitize.db \
  --apply
```

`--apply` refuses a missing, existing, symlink, or same-as-database backup path. It atomically creates the backup with mode `0600` and verifies it before changing the main database. The apply phase enables SQLite secure deletion, truncates the WAL, vacuums the database, truncates the WAL again, and verifies that no `Event.content` values remain. This command is never run automatically.
