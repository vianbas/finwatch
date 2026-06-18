# sqlc CI Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a CI gate that fails if committed sqlc-generated code is stale relative to SQL sources, and surface a `make sqlc-check` shortcut alongside the existing `make sqlc`.

**Architecture:** All sqlc infrastructure is already in place and complete — `sqlc.yaml`, `queries/*.sql`, generated `internal/platform/postgres/db/*.go`, all three store packages, and the `make sqlc` + `local-development.md` documentation. This plan closes the one remaining acceptance criterion from issue #7: the optional CI check that generated code is up to date (`sqlc diff`). It also adds a local `make sqlc-check` target and a CONTRIBUTING.md note so the full sqlc workflow is discoverable without reading the ops docs.

**Tech Stack:** sqlc v1.31.1, GitHub Actions, GNU Make, bash

## Global Constraints

- sqlc version MUST be pinned to `1.31.1` — this is the version recorded in every `// versions: sqlc v1.31.1` comment in the committed generated files; using a different version would reformat generated output and produce false diffs.
- Do NOT modify `migrations/`, `queries/`, or `internal/platform/postgres/db/` — those are the inputs and outputs that the CI gate protects; leave them untouched.
- `make verify` must remain green and must NOT call `make sqlc` or `make sqlc-check` — regeneration is intentionally manual.
- Conventional Commits: `build(sqlc): ...` for Makefile, `docs(sqlc): ...` for CONTRIBUTING.md, `ci(sqlc): ...` for CI workflow.
- Branch: `chore/7-sqlc-ci-gate` from `main`. Never push to `main` directly.
- PR body must include `Closes #7`.

---

### Task 1: Add `make sqlc-check` Makefile target

**Files:**
- Modify: `Makefile` — add `sqlc-check` to `.PHONY` and add target after `sqlc`

**Interfaces:**
- Produces: `make sqlc-check` shell target, used in Task 2 (docs) and Task 3 (CI step comment)

- [ ] **Step 1: Create the branch**

```bash
git checkout -b chore/7-sqlc-ci-gate
```
Expected: `Switched to a new branch 'chore/7-sqlc-ci-gate'`

- [ ] **Step 2: Edit `.PHONY` in `Makefile`**

Find this line in `Makefile`:
```makefile
.PHONY: help bootstrap dev stop lint test build verify sqlc seed \
        api-lint api-test api-build web-lint web-typecheck web-test web-build compose-config
```

Replace with:
```makefile
.PHONY: help bootstrap dev stop lint test build verify sqlc sqlc-check seed \
        api-lint api-test api-build web-lint web-typecheck web-test web-build compose-config
```

- [ ] **Step 3: Add `sqlc-check` target after the `sqlc` target**

Find this block in `Makefile`:
```makefile
sqlc: ## Regenerate type-safe DB code from SQL (requires sqlc on PATH)
	cd $(API_DIR) && sqlc generate
```

Replace with:
```makefile
sqlc: ## Regenerate type-safe DB code from SQL (requires sqlc on PATH)
	cd $(API_DIR) && sqlc generate

sqlc-check: ## Verify generated DB code matches SQL sources (requires sqlc on PATH)
	cd $(API_DIR) && sqlc diff
```

- [ ] **Step 4: Verify `make help` lists the new target**

```bash
make help
```
Expected output includes a line containing both `sqlc-check` and the description `Verify generated DB code matches SQL sources`.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "build(sqlc): add sqlc-check target wrapping sqlc diff"
```
Expected: commit created, no errors.

---

### Task 2: Document sqlc workflow in CONTRIBUTING.md

**Files:**
- Modify: `CONTRIBUTING.md` — extend the Validate step (step 4) in the Workflow section

**Interfaces:**
- Consumes: `make sqlc-check` from Task 1

- [ ] **Step 1: Find and update the Validate step**

In `CONTRIBUTING.md`, find this block inside the **Workflow** section:
```markdown
4. **Validate** locally:
   ```sh
   make verify
   git diff --check
   ```
```

Replace with:
```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs(sqlc): document make sqlc and sqlc-check in contributing workflow"
```
Expected: commit created, no errors.

---

### Task 3: Add `sqlc diff` gate to the backend CI job

**Files:**
- Modify: `.github/workflows/ci.yml` — add sqlc install step and `sqlc diff` step to the `backend` job

**Interfaces:**
- Consumes: `sqlc.yaml` at `apps/api/sqlc.yaml` (working-directory: `apps/api` is already set via `defaults.run`)
- No database connection needed — `sqlc diff` is a pure file comparison

- [ ] **Step 1: Read the current CI step order in the `backend` job**

The current order is:
1. Checkout
2. Set up Go
3. Verify formatting (gofmt)
4. Vet
5. Test (with `FINWATCH_TEST_DATABASE_URL`)
6. Build

- [ ] **Step 2: Insert sqlc steps between "Set up Go" and "Verify formatting"**

In `.github/workflows/ci.yml`, find this block inside the `backend` job `steps:` list:
```yaml
      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version: "1.26"
          cache-dependency-path: apps/api/go.sum
      - name: Verify formatting (gofmt)
```

Replace with:
```yaml
      - name: Set up Go
        uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
        with:
          go-version: "1.26"
          cache-dependency-path: apps/api/go.sum
      - name: Install sqlc
        run: |
          curl -fsSL \
            https://github.com/sqlc-dev/sqlc/releases/download/v1.31.1/sqlc_1.31.1_linux_amd64.tar.gz \
            | tar -xzf - sqlc
          sudo mv sqlc /usr/local/bin/sqlc
          sqlc version
      - name: Check sqlc output is up to date
        run: sqlc diff
      - name: Verify formatting (gofmt)
```

Note: The `defaults.run.working-directory: apps/api` at the job level already scopes `run:` commands to `apps/api`, so `sqlc diff` runs in the same directory as `sqlc.yaml`.

- [ ] **Step 3: Validate the YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML valid')"
```
Expected: `YAML valid`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(sqlc): verify committed query code is up to date on every PR"
```
Expected: commit created, no errors.

---

### Task 4: Verify and open PR

**Files:**
- No new file changes — this task is verification + PR creation only.

- [ ] **Step 1: Run `make verify`**

```bash
make verify
```
Expected: all gates pass — Go fmt, vet, tests, build; web lint, typecheck, tests, build. No failures.

- [ ] **Step 2: Run `git diff --check`**

```bash
git diff --check
```
Expected: empty output (no whitespace errors, no conflict markers).

- [ ] **Step 3: Review the three commits on the branch**

```bash
git log main..HEAD --oneline
```
Expected output (most-recent first):
```
<sha> ci(sqlc): verify committed query code is up to date on every PR
<sha> docs(sqlc): document make sqlc and sqlc-check in contributing workflow
<sha> build(sqlc): add sqlc-check target wrapping sqlc diff
```

- [ ] **Step 4: Push and open PR**

```bash
git push -u origin chore/7-sqlc-ci-gate
gh pr create \
  --title "chore(sqlc): CI gate for stale generated query code (#7)" \
  --body "$(cat <<'EOF'
## Summary

- Adds `make sqlc-check` Makefile target wrapping `sqlc diff` for local use.
- Updates `CONTRIBUTING.md` Workflow step 4 to document `make sqlc` / `make sqlc-check` when SQL sources change.
- Adds a `sqlc diff` gate to the backend CI job: installs sqlc v1.31.1 (matching the committed generated-code header) and fails if `internal/platform/postgres/db/` is stale relative to `queries/*.sql`.

Closes #7

## Architecture

No application code changes. The sqlc query layer was already complete when this PR was opened: `queries/*.sql`, `internal/platform/postgres/db/*.go`, all three store packages, `make sqlc`, and `docs/operations/local-development.md` were all in place. This PR adds the optional CI gate from the issue scope and the accompanying discoverability improvements (Makefile shortcut, CONTRIBUTING.md note).

The `sqlc diff` step is inserted early in the backend job (after Go setup, before gofmt) so that stale generated code is caught before test and build steps run.

## Security considerations

- No secrets or credentials involved.
- The sqlc binary is fetched from the official GitHub release at the pinned version v1.31.1 using a tarball download verified by the release URL. Pinning prevents unexpected behaviour from version upgrades.
- `sqlc diff` performs file comparison only; it does not connect to any database.

## Testing evidence

- `make verify` passes locally (Go fmt, vet, tests, build; web lint, typecheck, tests, build).
- `git diff --check` produces no output.
- `sqlc diff` exits 0 against the current committed generated code (no drift).

## Known limitations

- The CI download URL is `linux_amd64` specific. Local `make sqlc-check` requires a platform-appropriate sqlc install (see `docs/operations/local-development.md`).
- `make verify` intentionally does not call `make sqlc` or `make sqlc-check` — regeneration is always a manual step so developers decide when to evolve the schema.

## Follow-up issues

None — this closes #7.
EOF
)"
```

---

## Self-Review

**1. Spec coverage (issue #7 acceptance criteria):**

| Criterion | Status |
|---|---|
| Generated, type-safe query code checked in; no ORM | Already done before this plan — not regressed |
| `make sqlc` documented in CONTRIBUTING/operations docs | Task 2 adds the regeneration workflow to CONTRIBUTING.md; local-development.md already covered it |
| `make verify` green | Task 4 Step 1 runs `make verify` and requires it to pass |
| Optional: CI check that generated code is up to date | Task 3 implements `sqlc diff` in CI |

**2. Placeholder scan:** No TBD, TODO, "implement later", "handle edge cases", or vague "add tests" language. Every step contains exact commands or exact text replacements.

**3. Type/name consistency:**
- `make sqlc-check` — consistent across Task 1 (Makefile target), Task 2 (CONTRIBUTING.md text), Task 3 (CI comment), Task 4 (PR body).
- `sqlc v1.31.1` — pinned consistently in Task 3 install step and PR security note.
- `apps/api/internal/platform/postgres/db/` and `apps/api/queries/` — consistent throughout.
- `chore/7-sqlc-ci-gate` — branch name consistent in Global Constraints and Task 1 Step 1.
