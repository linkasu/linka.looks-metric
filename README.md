# linka.looks-metric

Go backend for LINKa Looks activation and telemetry.

## Runtime

- `POST /requestActivation` sends an activation code.
- `POST /activate` exchanges email and code for a PC hash.
- `POST /registerEvent` stores telemetry events.
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
