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
| **Spoofing** | Unauthenticated access to endpoints | JWT bearer-token auth on `/me`, `/transactions`, `/alerts*`; `/login` and `/health/*` stay public | RBAC enforcement (`RequireRole`) mounted on routes |
| **Tampering** | Malformed/oversized requests | Bounded HTTP timeouts; input validated at handlers (per-feature) | Schema validation against OpenAPI |
| **Repudiation** | No trace of actions | Per-request IDs + structured access logs | Audit logging for state changes |
| **Information disclosure** | Secrets/PII leakage | No secrets in repo; synthetic data only; logs exclude secrets/PII | Secret scanning, log review |
| **Denial of service** | Slow-client / resource exhaustion | Read/write/idle timeouts; panic recovery; graceful shutdown; WebSocket clients that fall behind are dropped instead of blocking fan-out, and dead peers are detected by pings and per-write timeouts | Rate limiting on `POST /login` (bcrypt cost makes it both a brute-force target and a CPU sink), connection caps |
| **Elevation of privilege** | Acting beyond role | N/A (`RequireRole` exists but is not yet mounted on any route) | RBAC enforced server-side |

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

RBAC route enforcement, transaction ingestion, rule evaluation, and
alerting — each will extend this model when implemented.

## Realtime stream (`/ws`)

- **Spoofing:** browsers cannot send an `Authorization` header on a WebSocket
  handshake, so the access token is offered as the `bearer.<token>`
  subprotocol and verified before upgrading (401 otherwise). Unlike a query
  parameter, it does not appear in access logs (only the path is logged).
- **Cross-site WebSocket hijacking:** cross-origin handshakes are accepted only
  from `CORS_ALLOWED_ORIGINS`; the token is never sent automatically by the
  browser, so a hostile page cannot ride an operator's session.
- **Token lifetime:** an open stream is closed (1008) when its token expires,
  so a connection cannot outlive the credential that opened it.
- **Planned:** per-user connection caps.
