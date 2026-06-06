---
status: passed
phase: 03-ci-automation-cross-engine-expansion
source:
  - 03-01-SUMMARY.md
  - 03-02a-SUMMARY.md
  - 03-02b-SUMMARY.md
  - 03-03-SUMMARY.md
  - 03-04-SUMMARY.md
  - 03-05-SUMMARY.md
started: 2026-06-06T12:48:55Z
updated: 2026-06-06T12:49:20Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server. Clear ephemeral state. Fresh build, fresh Docker containers, basic E2E test passes on clean state.
result: pass

### 2. GitHub Actions e2e.yml Exists and Structured
expected: `.github/workflows/e2e.yml` exists with 5-engine CI matrix (mysql, postgresql, mariadb, mssql, sqlite), fail-fast: false, always() cleanup, conditionals for SQLite (no Docker), and JUnit artifact upload.
result: issue
reported: "There are failing and passing CI / Build Check (macos-latest) (push) Successful in 45s"
severity: major

### 3. Makefile E2E Targets
expected: `make build-test` compiles all test packages. `make test-e2e-sqlite` runs SQLite tests without Docker. `make test-e2e-cross-engine` runs cross-engine validation. `make test-e2e-all` runs all 5 engines + cross-engine sequentially.
result: pass

### 4. PostgreSQL E2E Test Package Compiles
expected: `go test -tags e2e -c ./test/postgresql/` compiles without errors. `test/testdata/golden/postgresql/TestSchemaOutput.json` and `TestSnapshotOutput.json` exist as `{}` placeholders.
result: pass

### 5. MariaDB E2E Test Package Compiles
expected: `go test -tags e2e -c ./test/mariadb/` compiles without errors. Golden file placeholders exist for MariaDB.
result: pass

### 6. ConnectSQLite Helper Exists
expected: `test/testhelper/connect.go` has `ConnectSQLite` function that connects to a temp-file SQLite database without Docker.
result: pass

### 7. MSSQL E2E Test Package Compiles
expected: `go test -tags e2e -c ./test/mssql/` compiles without errors. Golden file placeholders exist for MSSQL.
result: pass

### 8. SQLite E2E Test Package Compiles
expected: `go test -tags e2e -c ./test/sqlite/` compiles without errors. Golden file placeholders exist for SQLite.
result: pass

### 9. Cross-Engine SQL Validation Matrix Compiles
expected: `go test -tags e2e -c ./test/cross-engine/` compiles without errors. Contains 12 table-driven sub-tests with per-engine dialect SQL maps.
result: pass

### 10. EXPLAIN Golden File Infrastructure
expected: `test/testdata/golden/{mysql,postgresql,mariadb,mssql,sqlite}/TestExplainOutput.json` all exist. Each engine golden test file has `Test{Engine}Explain` function with `normalizeEngineFields` volatile field normalization.
result: pass

### 11. AI Mock Server Has 3 Modes
expected: `test/testhelper/aimock.go` has `AIMockServer` type with success (200 + SQL), error (500 + error JSON), and retry (429 then 200) modes. Accessible via `StartAIMockServer`.
result: pass

### 12. AI E2E Tests Compile
expected: `go test -tags e2e -c ./test/ai/` compiles without errors. Tests cover SQL generation, retry on 429, and error handling on 500.
result: pass

### 13. Dockerfile.mssql with Build-Time Restore
expected: `Dockerfile.mssql` exists with multi-stage: start MSSQL, restore AdventureWorksLT2022.bak, shut down — baked-in data. SA_PASSWORD is a build ARG, not persisted. Restore script at `scripts/mssql-restore-adventureworks.sh`.
result: pass

### 14. MSSQL CI Image Build Job
expected: `.github/workflows/e2e.yml` has `build-mssql-image` job with buildx gha cache backend, pushing to ghcr.io. E2E matrix jobs pull pre-built MSSQL image.
result: pass

### 15. JUnit XML Output in CI
expected: CI gotestsum invocation uses `--junitfile-hide-empty-pkg --junitfile-hide-skipped-tests`. JUnit XML uploaded as artifact per matrix engine.
result: pass

### 16. E2E Documentation in README
expected: `README.md` has an `## E2E Testing` section covering prerequisites, local test commands, golden file workflow, CI pipeline overview, and engine addition guide.
result: pass

## Summary

total: 16
passed: 15
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "All CI checks pass on push/PR to main"
  status: failed
  reason: "User reported: There are failing and passing CI / Build Check (macos-latest) (push) Successful in 45s"
  severity: major
  test: 2
  root_cause: "CI workflow triggers in ci.yml and e2e.yml listened on branches: [main], but repo default branch is master. Workflows never triggered on push/PR to master."
  artifacts:
    - path: ".github/workflows/ci.yml"
      issue: "branches: [main] should be branches: [master]"
    - path: ".github/workflows/e2e.yml"
      issue: "branches: [main] should be branches: [master]"
  missing:
    - "Fix already applied in commit 438c10b: branches: [main] → branches: [master] in both ci.yml and e2e.yml; dead homebrew-tap/scoop-bucket jobs removed from release.yml"
  resolved: true
  resolution: "Branch refs updated from main to master in commit 438c10b"
