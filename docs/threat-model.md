# Threat model

A lightweight threat model for the FinWatch bootstrap. It will grow as features
land. FinWatch uses **synthetic data only** and is not a security/compliance
product; this model focuses on keeping the reference implementation safe to run
and contribute to.

## Assets

- Source code and contracts (public, open-source).
- Synthetic operational data (no confidentiality value, but integrity matters
  for correctness).
- Local developer environment and CI pipeline.

## Trust boundaries

1. Browser ↔ API (HTTP/WS).
2. API ↔ PostgreSQL.
3. Repository ↔ CI (GitHub Actions).
4. Project ↔ third-party dependencies.

## STRIDE summary (current scope)

| Threat | Example | Current mitigation | Planned |
| ------ | ------- | ------------------ | ------- |
| **Spoofing** | Unauthenticated access to endpoints | JWT bearer-token auth on `/me`, `/transactions`, `/alerts*`, `/rules*`; `/login` and `/health/*` stay public | — |
| **Tampering** | Malformed/oversized requests | Bounded HTTP timeouts; input validated at handlers (per-feature) | Schema validation against OpenAPI |
| **Repudiation** | No trace of actions | Per-request IDs + structured access logs; rule changes log `rule_id`, `enabled` and the acting `user_id` | Persistent audit log for state changes |
| **Information disclosure** | Secrets/PII leakage | No secrets in repo; synthetic data only; logs exclude secrets/PII | Secret scanning, log review |
| **Denial of service** | Slow-client / resource exhaustion | Read/write/idle timeouts; panic recovery; graceful shutdown | Rate limiting on `POST /login` (bcrypt cost makes it both a brute-force target and a CPU sink), connection caps |
| **Elevation of privilege** | Acting beyond role | Role checked server-side from the verified token: `PATCH /rules/{id}` requires `admin` (`RequireRole`), operators get 403; reads and alert workflow stay open to both roles | Role-aware UI (hide admin actions from operators) |

## Supply chain

- Dependencies are minimal and pinned via lockfiles (`go.sum`, `package-lock.json`).
- GitHub Actions are pinned to immutable commit SHAs.
- Dependabot monitors Go modules, npm, and GitHub Actions.
- CI uses least-privilege `GITHUB_TOKEN` permissions and holds no deployment
  secrets.

## Secrets & data handling

- `.env` is git-ignored; only example development values are committed.
- Container images run as non-root with no shell (distroless) where practical.
- See [security/data-classification.md](security/data-classification.md).

## Explicitly out of scope (bootstrap)

Transaction ingestion, rule evaluation, alerting, and live streaming — each will extend this model when implemented.
