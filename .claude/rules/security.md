# Security rules

## Data

- **Synthetic data only.** No real customer, account, or transaction data ever
  enters this repository or its fixtures.
- No real personal or financial data in tests, seeds, screenshots, or docs.
- See `docs/security/data-classification.md` for handling expectations.

## Proprietary content

- Do not include any specific bank's names, logos, internal schemas, internal
  thresholds, screenshots, credentials, or proprietary implementation details.
- Positioning is strictly: *an open-source reference implementation for
  near-real-time transaction monitoring and operational alert workflows using
  synthetic data.* Not a certified fraud/AML/sanctions/compliance product.

## Secrets

- No secrets, tokens, or private keys committed — ever.
- `.env.example` and `docker-compose.yml` contain only clearly-labelled example
  development credentials, never anything reused from a real environment.
- `.env` is git-ignored. CI uses least-privilege `GITHUB_TOKEN` permissions and
  contains no deployment or cloud credentials.

## Application security (baseline)

- Validate all configuration at startup; fail fast on invalid input.
- Bounded HTTP timeouts and graceful shutdown by default.
- Panic recovery prevents a single request from crashing the process.
- Structured logs must never contain secrets or raw personal/financial data.
- Planned auth: short-lived JWT access tokens + RBAC (not in the bootstrap).

## Reporting

- Vulnerabilities are reported privately per `SECURITY.md` — never via public
  issues.
