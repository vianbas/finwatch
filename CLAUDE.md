# FinWatch — Claude Code project instructions

FinWatch is an **open-source reference implementation for near-real-time
transaction monitoring and operational alert workflows using synthetic data.**

It is **not** a certified fraud, AML, sanctions, or regulatory compliance
product, and must never be represented as one.

## Non-negotiable guardrails

- **Synthetic data only.** All transaction and customer data is fabricated.
  Never add real personal, financial, or production data.
- **No proprietary references.** Do not introduce any specific bank's names,
  logos, internal schemas, internal thresholds, screenshots, credentials, or
  proprietary implementation details. Keep everything generic and public.
- **No secrets in the repo.** Only safe, clearly-labelled example development
  credentials belong in `.env.example` / `docker-compose.yml`.
- **Money is integer minor units.** Represent monetary values as integer minor
  units (e.g. cents) in a `BIGINT`/`number`. Never use floating point for money.
- **Time is UTC + RFC 3339.** Store and compute in UTC; serialise timestamps as
  RFC 3339 at every API and event boundary.

## Architecture (decided)

- **Modular monolith**, not microservices. One deployable API process hosting
  feature modules behind one HTTP surface.
- **Backend:** Go. HTTP via a small stdlib-compatible router (chi). PostgreSQL
  via **pgx + sqlc** — **no ORM**. Versioned SQL migrations.
- **Frontend:** React + TypeScript + Vite + Tailwind + shadcn-compatible
  components. Data fetching via TanStack Query. Charts via Recharts.
- **Real-time:** PostgreSQL transactional **outbox** + an in-process WebSocket
  hub. Do **not** add Kafka, Redis, NATS, or RabbitMQ.
- **Auth (future):** short-lived JWT access tokens + RBAC. Not implemented in
  the bootstrap.

## Contract-first

The API and event contracts are the source of truth and live in `contracts/`:

- `contracts/openapi.yaml` — REST contract (OpenAPI 3.1).
- `contracts/asyncapi.yaml` — WebSocket/event contract (AsyncAPI 3.0).

Change the contract **before** the implementation. Handlers, response
envelopes, and event payloads must match the committed contract. The Go error
envelope in `internal/platform/httpserver/response.go` mirrors the OpenAPI
`Error` schema; keep them aligned.

## Repository layout

```
apps/api      Go modular monolith (cmd/api entrypoint, internal/ packages)
apps/web      React + TypeScript + Vite frontend
contracts/    OpenAPI + AsyncAPI source of truth
docs/         Architecture, ADRs, threat model, operations, security
.github/      CI, templates, CODEOWNERS, Dependabot
```

Detailed working rules live in `.claude/rules/`:

- `.claude/rules/backend.md`
- `.claude/rules/frontend.md`
- `.claude/rules/security.md`
- `.claude/rules/git-workflow.md`

## Scope discipline

Keep code intentionally small and idiomatic. Do **not** build generic
abstraction layers ahead of need. The bootstrap deliberately does **not**
implement transactions, alerts, rule evaluation, JWT login, or WebSocket
streaming — those arrive in their own issues. Avoid `TODO` comments unless they
reference a tracking GitHub issue (e.g. `TODO(#42):`).

## Local commands

```
make bootstrap   # install/prepare dependencies
make dev         # start the full stack via Docker Compose
make stop        # stop the stack
make lint        # lint backend + frontend
make test        # run backend + frontend tests
make build       # build backend + frontend
make verify      # fmt check, vet, lint, type-check, tests, builds
```
