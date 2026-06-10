# Container design

This document describes the runtime containers (in the C4 sense) and how they
interact. The system is intentionally a **modular monolith**, not microservices.

## Containers

### Web application (`apps/web`)

- React + TypeScript + Vite single-page app.
- Tailwind + shadcn-compatible components; TanStack Query for server state;
  Recharts for visualisation.
- Validates `VITE_API_URL` and `VITE_WS_URL` at startup.
- Built to static assets and served by an unprivileged nginx container locally.

### API (`apps/api`)

A single Go process composed of internal modules:

```
cmd/api                      process entrypoint, lifecycle, graceful shutdown
internal/config              env loading + startup validation
internal/platform/httpserver chi router, middleware, timeouts, health, errors
internal/platform/postgres   pgx connection pool construction
internal/<module>            (future) feature modules: transactions, alerts, rules
```

- Modules interact through Go interfaces in-process — no network hops.
- Standard middleware: request ID → panic recovery → structured access log.
- Bounded HTTP timeouts and graceful, time-boxed shutdown.

### PostgreSQL

- System of record, accessed via **pgx**; typed queries via **sqlc** (no ORM).
- Versioned migrations in `apps/api/migrations`.
- Hosts the **transactional outbox** (`outbox_events`) that underpins realtime
  delivery (see ADR 0003).

## Realtime path (planned)

```
write tx + outbox row  ->  outbox relay (in-process)  ->  WebSocket hub  ->  clients
        (one DB transaction)        polls unpublished        in-process fan-out
```

No Kafka/Redis/NATS/RabbitMQ. The outbox guarantees an event is recorded
atomically with the state change; the in-process hub fans it out to connected
operators. Only the schema exists in the bootstrap.

## Local composition

`docker-compose.yml` wires three services — `db`, `api`, `web` — with health
checks, a named volume for Postgres data, and example-only credentials. One
command (`make dev`) brings the stack up.

## Cross-cutting decisions

| Concern        | Decision                                                        |
| -------------- | --------------------------------------------------------------- |
| Config         | Environment variables, validated at startup (fail fast).        |
| Logging        | Structured JSON via `log/slog`.                                 |
| Money          | Integer minor units (`BIGINT` / `int64`).                       |
| Time           | UTC internally; RFC 3339 at API/event boundaries.               |
| Auth (future)  | Short-lived JWT access tokens + RBAC.                            |
