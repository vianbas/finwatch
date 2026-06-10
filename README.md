# FinWatch

**An open-source reference implementation for near-real-time transaction
monitoring and operational alert workflows using synthetic data.**

> FinWatch is an educational reference architecture. It is **not** a certified
> fraud, AML, sanctions, or regulatory-compliance product, and must not be used
> or represented as one. All transaction and customer data is **synthetic**.

## What this is

A small, idiomatic **modular monolith** demonstrating how to structure a
near-real-time monitoring and alerting platform:

- **Backend** — Go, chi router, PostgreSQL via pgx + sqlc (no ORM), versioned
  migrations, structured JSON logging, validated configuration, graceful shutdown.
- **Frontend** — React + TypeScript + Vite, Tailwind + shadcn-compatible
  components, TanStack Query, Recharts.
- **Real-time (planned)** — PostgreSQL transactional outbox + an in-process
  WebSocket hub. No Kafka/Redis/NATS/RabbitMQ.
- **Contracts** — OpenAPI for REST, AsyncAPI for events, as the source of truth.

This repository is the **bootstrap**: structure, skeletons, contracts, docs,
local environment, and CI. Business features (transactions, alerts, rule
evaluation, auth, live streaming) arrive in later issues.

## Repository layout

```
apps/api      Go modular monolith (cmd/api, internal/ packages, migrations)
apps/web      React + TypeScript + Vite frontend
contracts/    OpenAPI + AsyncAPI contracts
docs/         Architecture, ADRs, threat model, operations, security
.github/      CI, issue/PR templates, CODEOWNERS, Dependabot
```

## Quick start

Requires Docker + Docker Compose v2.

```sh
cp .env.example .env
make dev
```

Then:

- API liveness — http://localhost:8080/health/live
- API readiness — http://localhost:8080/health/ready
- Transactions — http://localhost:8080/transactions (seed first: `make seed N=100`)
- Web app — http://localhost:8081

Stop the stack with `make stop`.

See [docs/operations/local-development.md](docs/operations/local-development.md)
for the full guide.

## Developer commands

| Command          | Purpose                                            |
| ---------------- | -------------------------------------------------- |
| `make bootstrap` | Install backend and frontend dependencies          |
| `make dev`       | Start the full stack via Docker Compose            |
| `make stop`      | Stop the stack                                     |
| `make lint`      | Lint backend and frontend                          |
| `make test`      | Run backend and frontend tests                     |
| `make build`     | Build backend and frontend                         |
| `make verify`    | Run every quality gate (fmt, vet, lint, types, tests, builds) |

## Toolchain

| Tool   | Version |
| ------ | ------- |
| Go     | 1.26    |
| Node   | 24      |
| npm    | 11      |
| Docker | 24+ with Compose v2 |

## Contributing & security

- Contribution workflow: [CONTRIBUTING.md](CONTRIBUTING.md)
- Vulnerability reporting: [SECURITY.md](SECURITY.md)
- Community standards: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Project guardrails for AI-assisted work: [CLAUDE.md](CLAUDE.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
