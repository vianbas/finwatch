# ADR 0002: Go backend with pgx + sqlc (no ORM)

- Status: Accepted
- Date: 2026-06-10

## Context

The backend needs predictable performance for near-real-time workloads, a
strong standard library for HTTP and concurrency, simple deployment as a single
static binary, and transparent, reviewable data access.

## Decision

Use **Go** for the backend, with:

- a small, stdlib-compatible HTTP router (**chi**),
- **pgx** for PostgreSQL connectivity, and
- **sqlc** to generate type-safe Go from hand-written SQL.

**No ORM.** SQL is written explicitly and reviewed; sqlc provides type safety
without hiding queries behind an abstraction.

## Consequences

**Positive**

- Single static binary; trivial, secure container images (distroless, non-root).
- `net/http`-compatible routing keeps middleware and testing idiomatic.
- Explicit SQL is auditable and performant; sqlc removes hand-mapping errors.
- `log/slog` gives structured logging without third-party dependencies.

**Negative / trade-offs**

- More boilerplate than an ORM for simple CRUD.
- Query authors must understand SQL and the schema — an acceptable, deliberate
  cost for a monitoring system where query behaviour matters.

## Conventions established

- Money is represented as integer minor units (`int64`).
- Timestamps are UTC internally and RFC 3339 at boundaries.
- Configuration is validated at startup; invalid config is fatal.
- HTTP servers use bounded timeouts and graceful shutdown.

## Alternatives considered

- **ORM (e.g. GORM)** — rejected: hides query behaviour and complicates
  performance reasoning.
- **`database/sql` only** — workable, but manual row scanning is error-prone;
  sqlc gives the same transparency with generated type safety.
