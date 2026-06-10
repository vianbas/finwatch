# Git & workflow rules

## Branching

- Issue-driven development. Every change starts from a GitHub issue and a
  dedicated branch (e.g. `chore/1-bootstrap-monorepo`, `feat/12-alert-list`).
- **Never** push directly to `main`.

## Commits

- Conventional Commits: `type(scope): summary`
  (`feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`).
- Cohesive, logically-scoped commits. Keep unrelated changes in separate commits.

## Pull requests

- Open a PR targeting `main`. PRs are reviewed and merged by maintainers.
- **Never merge your own PR.** Do not merge PRs as part of automated work.
- Merge strategy is **merge commit** — not squash, not rebase.
- PR body must include:
  - `Closes #<issue-number>`
  - architecture summary
  - security considerations
  - testing evidence
  - known limitations
  - follow-up issues

## Before pushing

- Run `make verify`.
- `git diff --check` (no whitespace errors / conflict markers).
- Scan the diff for credentials, personal data, and proprietary references.

## Ownership

- `@vianbas` owns the repository via `CODEOWNERS`.
