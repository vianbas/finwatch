# Contributing to FinWatch

Thanks for your interest in FinWatch — an open-source reference implementation
for near-real-time transaction monitoring and operational alert workflows using
**synthetic data**.

## Ground rules

- **Synthetic data only.** Never contribute real personal, account, or
  transaction data.
- **No proprietary content.** Do not add any specific organisation's names,
  logos, internal schemas, thresholds, screenshots, or credentials.
- **No secrets.** Never commit secrets or real credentials. Only safe example
  development values belong in `.env.example` and `docker-compose.yml`.
- By contributing you agree your contributions are licensed under Apache-2.0.

## Workflow

1. **Start from an issue.** Open or pick a GitHub issue describing the change.
2. **Branch** from `main` using a descriptive name, e.g. `feat/12-alert-list`.
3. **Develop** following the rules in [`.claude/rules/`](.claude/rules) and
   the architecture in [`docs/`](docs).
4. **Validate** locally:
   ```sh
   make verify
   git diff --check
   ```
   If you changed any `apps/api/queries/*.sql` file or added a migration,
   regenerate the sqlc output and verify it before committing:
   ```sh
   make sqlc          # regenerate apps/api/internal/platform/postgres/db/
   make sqlc-check    # confirm generated code matches sources (runs sqlc diff)
   ```
5. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/):
   `type(scope): summary` (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`,
   `ci`, `build`).
6. **Open a pull request** targeting `main`. Fill in the PR template, including
   `Closes #<issue>`, architecture summary, security considerations, testing
   evidence, known limitations, and follow-up issues.

## Rules

- Never push directly to `main`.
- Maintainers review and merge PRs using **merge commits** (not squash/rebase).
- Do not merge your own PR.

## Conventions

- **Backend:** Go, idiomatic and small. pgx + sqlc, no ORM. Money as integer
  minor units. UTC internally, RFC 3339 at boundaries. See
  [`.claude/rules/backend.md`](.claude/rules/backend.md).
- **Frontend:** React + TypeScript + Vite, Tailwind, TanStack Query, Recharts.
  See [`.claude/rules/frontend.md`](.claude/rules/frontend.md).
- **Contracts first:** update `contracts/openapi.yaml` / `contracts/asyncapi.yaml`
  before changing the corresponding implementation.

## Quality gates

A PR is ready when `make verify` passes: Go fmt + vet + tests + build, and
frontend lint + type-check + tests + build.
