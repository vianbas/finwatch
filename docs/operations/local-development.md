# Local development

This guide gets the full FinWatch stack running locally.

## Prerequisites

| Tool   | Version | Notes |
| ------ | ------- | ----- |
| Docker | 24+     | With **Compose v2** (`docker compose`, not `docker-compose`). |
| Go     | 1.26    | For running the backend/tests outside Docker. |
| Node   | 24      | With npm 11, for the frontend outside Docker. |
| Make   | any     | Task runner for the commands below. |

> Running `make dev` only requires Docker + Compose. Go and Node are needed for
> running tests/builds directly on the host (e.g. `make verify`).

## One-command start

```sh
cp .env.example .env
make dev
```

This builds and starts three services:

| Service | URL / port                         | Purpose                |
| ------- | ---------------------------------- | ---------------------- |
| db      | localhost:5432                     | PostgreSQL (synthetic) |
| api     | http://localhost:8080              | Go API                 |
| web     | http://localhost:8081              | React app              |

Verify the API:

```sh
curl http://localhost:8080/health/live    # {"status":"ok"}
curl http://localhost:8080/health/ready    # {"status":"ready"} once db is up
```

Stop the stack:

```sh
make stop
```

## Running pieces directly

Backend:

```sh
cd apps/api
go test ./...
go run ./cmd/api      # needs DATABASE_URL etc. from your environment
```

Frontend:

```sh
cd apps/web
npm install
npm run dev           # Vite dev server on http://localhost:5173
```

## Database migrations

Migrations are **not** applied automatically by the API. Apply them with the
`migrate` CLI:

```sh
migrate -path apps/api/migrations -database "$DATABASE_URL" up
```

See [apps/api/migrations/README.md](../../apps/api/migrations/README.md).

## Quality gate

Before pushing:

```sh
make verify          # fmt, vet, lint, type-check, tests, builds
git diff --check
```

## Configuration

All configuration is via environment variables (see `.env.example`) and is
validated at API startup — an invalid value stops the process immediately with a
descriptive error.

## Troubleshooting

- **`docker compose` not found** — install Docker Compose v2 (bundled with
  recent Docker Desktop / the `docker-compose-plugin` package).
- **API readiness returns 503** — the database is not up yet; wait for the `db`
  health check, then retry.
- **Port already in use** — stop the conflicting process or change the published
  ports in `docker-compose.yml`.
