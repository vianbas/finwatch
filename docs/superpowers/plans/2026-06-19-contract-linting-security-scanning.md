# Contract Linting & Security Scanning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three net-new GitHub Actions workflows — OpenAPI/AsyncAPI contract linting, CodeQL (Go + JS/TS), and PR dependency review — without touching the existing `ci.yml`, application code, or contract files.

**Architecture:** Each concern gets its own dedicated workflow file under `.github/workflows/`, triggered independently of the existing `ci.yml`. Contract linting runs Spectral (`spectral:oas`) against `contracts/openapi.yaml` and the official `@asyncapi/cli` against `contracts/asyncapi.yaml` inside one job, using Node (already a project dependency via `apps/web`). CodeQL runs its own workflow with a `language` matrix (`go`, `javascript-typescript`). Dependency review runs only on `pull_request` events using `actions/dependency-review-action`. All three workflows declare their own least-privilege `permissions:` block (workflow-level, not relying on repo defaults) and pin every third-party action to a commit SHA with a version comment, matching the existing convention in `ci.yml`.

**Tech Stack:** GitHub Actions, Spectral CLI (`@stoplight/spectral-cli`), AsyncAPI CLI (`@asyncapi/cli`), CodeQL (`github/codeql-action`), `actions/dependency-review-action`, Node 24 (via `actions/setup-node`).

## Global Constraints

- Do NOT modify `.github/workflows/ci.yml` — these are net-new, independently triggered workflows.
- Do NOT modify `contracts/openapi.yaml` or `contracts/asyncapi.yaml` unless a verification step in this plan proves an actual lint/validation failure exists. If that happens, stop and report the failure instead of silently editing the contract.
- Do NOT add deployment, release, or publishing steps — scope is CI gating only.
- Every new third-party action MUST be pinned to a commit SHA with a `# vX.Y.Z` comment, per `.claude/rules/git-workflow.md` and the existing `ci.yml` pattern.
- Every new workflow MUST declare its own top-level `permissions:` block — do not rely on org/repo defaults.
- Conventional Commits: `ci(contracts): ...`, `ci(codeql): ...`, `ci(deps): ...`.
- Branch: `ci/8-contract-lint-security-scanning` from `main`. Never push to `main` directly.
- PR body must include `Closes #8`.
- Pinned third-party action SHAs to use (already resolved against GitHub releases API):
  - `actions/checkout` → `df4cb1c069e1874edd31b4311f1884172cec0e10` (`# v6.0.3`) — already used in `ci.yml`, reuse for consistency.
  - `actions/setup-node` → `48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e` (`# v6.4.0`) — already used in `ci.yml`, reuse for consistency.
  - `github/codeql-action/init`, `/autobuild`, `/analyze` → `dd903d2e4f5405488e5ef1422510ee31c8b32357` (`# v3.36.2`).
  - `actions/dependency-review-action` → `a1d282b36b6f3519aa1f3fc636f609c47dddb294` (`# v5.0.0`).

---

### Task 1: Spectral ruleset + OpenAPI/AsyncAPI contract-linting workflow

**Files:**
- Create: `contracts/.spectral.yaml`
- Create: `.github/workflows/contracts.yml`

**Interfaces:**
- Produces: a CI job named `lint-contracts` that fails the workflow run (non-zero exit) on any OpenAPI or AsyncAPI validation error. No other task depends on this job's internals.

- [ ] **Step 1: Create the branch**

```bash
git checkout main
git pull
git checkout -b ci/8-contract-lint-security-scanning
```
Expected: `Switched to a new branch 'ci/8-contract-lint-security-scanning'`

- [ ] **Step 2: Create the Spectral ruleset**

Create `contracts/.spectral.yaml`:

```yaml
extends: ["spectral:oas", "recommended"]
```

- [ ] **Step 3: Verify the ruleset lints the existing OpenAPI contract clean**

Run (from repo root, no install needed thanks to `npx`):
```bash
npx --yes @stoplight/spectral-cli lint contracts/openapi.yaml --ruleset contracts/.spectral.yaml
```
Expected: exit code `0`, no `error`-severity results. (Warnings are acceptable and do not block CI in this plan; only `error` severity fails the job.)

If this reports actual errors, STOP — do not edit `contracts/openapi.yaml` as part of this task. Report the exact Spectral error output instead and wait for direction.

- [ ] **Step 4: Verify the AsyncAPI CLI validates the existing AsyncAPI contract clean**

```bash
npx --yes @asyncapi/cli validate contracts/asyncapi.yaml
```
Expected: output ending in `File contracts/asyncapi.yaml is valid!` (or equivalent success message), exit code `0`.

If this fails, STOP — do not edit `contracts/asyncapi.yaml`. Report the exact validation error and wait for direction.

- [ ] **Step 5: Create the contract-linting workflow**

Create `.github/workflows/contracts.yml`:

```yaml
name: Contracts

on:
  push:
    branches: [main]
    paths:
      - "contracts/**"
      - ".github/workflows/contracts.yml"
  pull_request:
    branches: [main]
    paths:
      - "contracts/**"
      - ".github/workflows/contracts.yml"

permissions:
  contents: read

concurrency:
  group: contracts-${{ github.ref }}
  cancel-in-progress: true

jobs:
  lint-contracts:
    name: Lint OpenAPI & AsyncAPI contracts
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - name: Set up Node
        uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6.4.0
        with:
          node-version: "24"
      - name: Lint OpenAPI contract (Spectral)
        run: npx --yes @stoplight/spectral-cli lint contracts/openapi.yaml --ruleset contracts/.spectral.yaml
      - name: Validate AsyncAPI contract
        run: npx --yes @asyncapi/cli validate contracts/asyncapi.yaml
```

- [ ] **Step 6: Validate the workflow YAML is well-formed**

```bash
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/contracts.yml')); puts 'YAML valid'"
```
Expected: `YAML valid`

- [ ] **Step 7: Commit**

```bash
git add contracts/.spectral.yaml .github/workflows/contracts.yml
git commit -m "ci(contracts): lint OpenAPI and AsyncAPI contracts on PR"
```

---

### Task 2: CodeQL workflow (Go + JavaScript/TypeScript)

**Files:**
- Create: `.github/workflows/codeql.yml`

**Interfaces:**
- Produces: a CI workflow named `CodeQL` with a `language` matrix (`go`, `javascript-typescript`) that uploads SARIF results to GitHub code scanning. No other task depends on this job's internals.

- [ ] **Step 1: Create the CodeQL workflow**

Create `.github/workflows/codeql.yml`:

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read
  security-events: write

concurrency:
  group: codeql-${{ github.ref }}
  cancel-in-progress: true

jobs:
  analyze:
    name: Analyze (${{ matrix.language }})
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        language: [go, javascript-typescript]
    steps:
      - name: Checkout
        uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - name: Initialize CodeQL
        uses: github/codeql-action/init@dd903d2e4f5405488e5ef1422510ee31c8b32357 # v3.36.2
        with:
          languages: ${{ matrix.language }}
      - name: Autobuild
        uses: github/codeql-action/autobuild@dd903d2e4f5405488e5ef1422510ee31c8b32357 # v3.36.2
      - name: Perform CodeQL analysis
        uses: github/codeql-action/analyze@dd903d2e4f5405488e5ef1422510ee31c8b32357 # v3.36.2
        with:
          category: "/language:${{ matrix.language }}"
```

- [ ] **Step 2: Validate the workflow YAML is well-formed**

```bash
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/codeql.yml')); puts 'YAML valid'"
```
Expected: `YAML valid`

- [ ] **Step 3: Confirm Autobuild can handle both languages without extra config**

This is a documentation check, not a command: Go (`apps/api`) builds via plain `go build` with no build tags or cgo requirements (confirmed in `apps/api/go.mod` — only `chi`, `pgx`, and small indirect deps), and the JS/TS source under `apps/web` has a standard `package.json`/`vite` setup. CodeQL's Autobuild step handles both without a custom build script. No action needed; record this conclusion in the PR's "known limitations" if Autobuild ever needs a manual build step in the future (it does not today).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/codeql.yml
git commit -m "ci(codeql): scan Go and JavaScript/TypeScript on PRs to main"
```

---

### Task 3: Dependency review workflow

**Files:**
- Create: `.github/workflows/dependency-review.yml`

**Interfaces:**
- Produces: a CI workflow named `Dependency Review` that fails the PR check on newly introduced dependencies with known vulnerabilities. No other task depends on this job's internals.

- [ ] **Step 1: Create the dependency-review workflow**

Create `.github/workflows/dependency-review.yml`:

```yaml
name: Dependency Review

on:
  pull_request:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: dependency-review-${{ github.ref }}
  cancel-in-progress: true

jobs:
  dependency-review:
    name: Dependency review
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3
      - name: Dependency review
        uses: actions/dependency-review-action@a1d282b36b6f3519aa1f3fc636f609c47dddb294 # v5.0.0
        with:
          fail-on-severity: high
```

- [ ] **Step 2: Validate the workflow YAML is well-formed**

```bash
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/dependency-review.yml')); puts 'YAML valid'"
```
Expected: `YAML valid`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/dependency-review.yml
git commit -m "ci(deps): fail PRs that introduce high-severity vulnerable dependencies"
```

---

### Task 4: Open the PR

**Files:** none (no new files; this task pushes the branch and opens the PR)

**Interfaces:** none — this is the finishing step for issue #8.

- [ ] **Step 1: Push the branch**

```bash
git push -u origin ci/8-contract-lint-security-scanning
```
Expected: branch created on `origin`, no force-push.

- [ ] **Step 2: Open the PR**

```bash
gh pr create \
  --base main \
  --title "ci: contract linting and security scanning" \
  --body "$(cat <<'EOF'
## Summary
- Add a `contracts.yml` workflow that lints `contracts/openapi.yaml` with Spectral (`spectral:oas`) and validates `contracts/asyncapi.yaml` with the official AsyncAPI CLI.
- Add a `codeql.yml` workflow scanning Go and JavaScript/TypeScript on PRs to `main`.
- Add a `dependency-review.yml` workflow that fails PRs introducing high-severity vulnerable dependencies.
- All new third-party actions are pinned to commit SHAs with version comments; every new workflow declares its own least-privilege `permissions:` block. `ci.yml` is untouched.

Closes #8

## Architecture summary
Three independent, narrowly-scoped GitHub Actions workflows, each triggered only by the events it needs (contract changes, all PRs/pushes to main, PRs only respectively). No changes to application code, contracts, or the existing CI pipeline.

## Security considerations
- CodeQL adds SAST coverage for both languages in the monorepo.
- `actions/dependency-review-action` blocks PRs introducing high-severity vulnerable dependencies, complementing Dependabot's scheduled updates.
- All new actions are SHA-pinned; permissions are least-privilege per workflow (`contents: read` plus `security-events: write` only on the CodeQL workflow, which needs it to upload SARIF results).

## Testing evidence
- `npx @stoplight/spectral-cli lint contracts/openapi.yaml --ruleset contracts/.spectral.yaml` — clean, exit 0.
- `npx @asyncapi/cli validate contracts/asyncapi.yaml` — valid, exit 0.
- All three new workflow files validated as well-formed YAML.
- CI run results for `Contracts`, `CodeQL`, and `Dependency Review` attached after first push (see Checks tab).

## Known limitations
- `fail-on-severity: high` in dependency review may need tuning if it proves too strict or too lenient once real PRs flow through it.
- CodeQL Autobuild has not been tested against a real PR diff yet; if Autobuild ever fails to infer the build steps, a manual `build-mode: manual` step will be needed for the Go matrix entry.

## Follow-up issues
- None identified; revisit `fail-on-severity` threshold if it generates noise.
EOF
)"
```
Expected: PR created against `main`, URL printed.

- [ ] **Step 3: Confirm CI runs and report status**

```bash
gh pr checks --watch
```
Expected: all three new workflows (plus existing `ci.yml` jobs) report success, or report the exact failure for debugging.

---

## Failure modes and expected fixes

| Failure | Likely cause | Fix |
|---|---|---|
| Spectral reports `error`-severity OAS issues | Pre-existing contract defect not previously caught | Stop, report exact rule ID + line, get explicit approval before editing `contracts/openapi.yaml` |
| AsyncAPI CLI reports invalid document | Pre-existing AsyncAPI 3.0 schema defect | Stop, report exact validation error, get explicit approval before editing `contracts/asyncapi.yaml` |
| CodeQL Autobuild fails for Go | Multi-module workspace or build tag CodeQL can't infer | Add `build-mode: manual` with explicit `go build ./...` step scoped to `apps/api` |
| CodeQL Autobuild fails for JS/TS | Missing `npm ci` before analysis | Add an explicit `actions/setup-node` + `npm ci` step in `apps/web` before the `analyze` step |
| Dependency review fails on an existing (not newly introduced) dependency | `fail-on-severity` too strict for current baseline | Confirm via `gh pr checks` output whether the flagged dependency is net-new to the PR diff; if not net-new, this indicates a pre-existing vulnerable dependency to track as a follow-up issue, not a workflow bug |
| Any workflow YAML fails to parse | Indentation or quoting error introduced while authoring | Re-run the `ruby -e "require 'yaml'..."` check from the relevant task step before committing |

## Verification commands (full set, for final review)

```bash
npx --yes @stoplight/spectral-cli lint contracts/openapi.yaml --ruleset contracts/.spectral.yaml
npx --yes @asyncapi/cli validate contracts/asyncapi.yaml
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/contracts.yml')); puts 'YAML valid'"
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/codeql.yml')); puts 'YAML valid'"
ruby -e "require 'yaml'; YAML.safe_load(File.read('.github/workflows/dependency-review.yml')); puts 'YAML valid'"
git status --short
gh pr checks --watch
```
