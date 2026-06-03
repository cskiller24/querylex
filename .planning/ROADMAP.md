# Roadmap: QueryLex

## Overview

QueryLex is a brownfield Go CLI tool that helps AI agents introspect database schemas, generate SQL from natural language, and optimize queries across 5 SQL engines. The core functionality already exists — this roadmap delivers a unified single-binary build, a comprehensive Docker-based E2E test infrastructure, and automated CI pipelines. Each phase builds on the last: infrastructure first (clean build + Docker Compose + test helpers), then MySQL E2E tests (proving the pattern), then full cross-engine expansion with CI automation.

## Phases

- [x] **Phase 1: Monorepo Cleanup + Docker Infrastructure** - Unify the build (single `querylex` binary), create compose.yaml with all 5 database services, build test helper package, and provision sample datasets (completed 2026-06-03)
- [x] **Phase 2: MySQL E2E Test Suite** - Comprehensive E2E tests for MySQL using real datasets with golden files, exit code verification, per-test isolation, and CLI flag combinatorics (completed 2026-06-03)
- [ ] **Phase 3: CI Automation + Cross-Engine Expansion** - GitHub Actions matrix workflow, E2E tests for remaining 4 database engines, EXPLAIN comparison suite, and AI mock server

## Phase Details

### Phase 1: Monorepo Cleanup + Docker Infrastructure
**Goal**: Single unified `querylex` binary build with Docker Compose infrastructure for E2E testing across all 5 database engines
**Mode**: mvp
**Depends on**: Nothing (first phase)
**Requirements**: BLD-01, BLD-02, BLD-03, BLD-04, BLD-05, DKR-01, DKR-02, DKR-03, DKR-04, DKR-05, DKR-06, DKR-07, DKR-08, HLP-01, HLP-02, HLP-03, HLP-04, HLP-05, HLP-06, DAT-01, DAT-02, DAT-03, DAT-04, DAT-05, DAT-06
**Success Criteria** (what must be TRUE):
  1. `go build ./...` produces a single `querylex` binary; standalone `querylex-add-db` and `querylex-stats` binaries no longer exist in goreleaser output or Makefile targets
  2. `docker compose --profile mysql up` starts a healthy MySQL 8.4 container on a random port with healthcheck passing, tmpfs storage, and pinned image version
  3. `make build-test` compiles the E2E test binary without errors; `make compose-up-mysql` provisions a reachable MySQL instance
  4. `test/testhelper.ConnectMySQL(t)` connects to live MySQL via env-var-driven DSN, retries with backoff, registers `t.Cleanup()` teardown
  5. Real-world sample datasets (Employees DB for MySQL, Chinook for SQLite, Pagila for PostgreSQL, Northwind for MSSQL) are loadable via `test/scripts/load-fixtures.sh`
**Plans**: 5 plans in 2 waves

Plans:
- [x] 01-01-PLAN.md — Remove standalone binary builds: goreleaser, Makefile, CI, .gitignore, root.go usage text, delete cmd/querylex-add-db/ and cmd/querylex-stats/
- [x] 01-02-PLAN.md — Create compose.yaml with 4 profiled database services (MySQL, PostgreSQL, MariaDB, MSSQL), healthchecks, tmpfs, random ports, memory limits, .env.example
- [x] 01-03-PLAN.md — Implement test/testhelper package: Connect*, WaitForPort, FixtureRunner, RunQuerylex, GenerateDBName
- [x] 01-04-PLAN.md — Create sample dataset download/load scripts (Employees, Chinook, Pagila, Northwind) and committed test fixtures
- [x] 01-05-PLAN.md — Add Makefile targets: compose-up-{db}, compose-down, build-test, test-e2e-{db}, resolve-port.sh

### Phase 2: MySQL E2E Test Suite
**Goal**: Comprehensive, isolated E2E test suite for MySQL using real datasets, golden files, and per-test database isolation
**Mode**: mvp
**Depends on**: Phase 1
**Requirements**: MYS-01, MYS-02, MYS-03, MYS-04, MYS-05, MYS-06, MYS-07, MYS-08
**Success Criteria** (what must be TRUE):
  1. `go test -tags e2e ./test/mysql/` passes all MySQL E2E tests against a live Docker container with real Employees DB data
  2. Golden file tests verify every deterministic JSON output matches expected envelope format (status, version, error, timestamp fields)
  3. Exit code tests confirm: 0 for successful commands, specific error codes (ERR_CONNECT, ERR_INVALID_SQL, etc.) for failure cases
  4. Each test creates its own UUID-named database and drops it via `t.Cleanup()` — zero data leakage between test runs
  5. CLI flag combinatorics tests for `sql` and `optimize` subcommands pass across valid/invalid flag combinations and output formats
**Plans**: 4 plans in 2 waves

Plans:
- [x] 02-01-PLAN.md — Infrastructure foundation: credential auto-unlock, workspace setup helper, schema extraction + isolation tests (MYS-01, MYS-07)
- [x] 02-02-PLAN.md — Golden file JSON output verification with normalization + exit code matrix (MYS-02, MYS-03, MYS-05)
- [x] 02-03-PLAN.md — Schema-aware SQL validation + full schema snapshot golden file (MYS-06, MYS-08)
- [x] 02-04-PLAN.md — CLI flag combinatorics for 7 deterministic subcommands + AI subcommand flag tests (MYS-04)

### Phase 3: CI Automation + Cross-Engine Expansion
**Goal**: Automated CI pipeline running E2E tests for all 5 database engines, with cross-engine validation, EXPLAIN comparison, and AI mock server
**Mode**: mvp
**Depends on**: Phase 2
**Requirements**: CI-01, CI-02, CI-03, CI-04, CI-05, CI-06, CI-07, XDB-01, XDB-02, XDB-03, XDB-04, XDB-05, ADV-01, ADV-02, ADV-03
**Success Criteria** (what must be TRUE):
  1. GitHub Actions workflow runs E2E tests for all 5 databases as a matrix, with `fail-fast: false`, `always()` cleanup, and container log dump on failure
  2. `make test-e2e-all` passes for MySQL, PostgreSQL, MariaDB, MSSQL, and SQLite — each with golden file comparisons
  3. Cross-engine SQL validation matrix confirms the same SQL works correctly across all 5 supported engines with engine-specific quoting and syntax
  4. EXPLAIN plan comparison suite validates plan output structure per engine (MySQL FORMAT=JSON, PostgreSQL text, MSSQL XML SHOWPLAN_XML)
  5. AI mock server enables deterministic E2E tests for AI-powered features without real API calls; pre-built MSSQL Docker image with AdventureWorksLT works in CI
**Plans**: 5 plans in 3 waves

Plans:
- [ ] 03-01-PLAN.md — GitHub Actions e2e.yml with 5-engine CI matrix, fail-fast: false, always cleanup, Makefile test-e2e-all target
- [ ] 03-02-PLAN.md — Cross-engine E2E test packages for PostgreSQL, MariaDB, MSSQL, SQLite (33 new files, each with 7-test coverage)
- [ ] 03-03-PLAN.md — Cross-engine SQL validation matrix (12+ patterns) and EXPLAIN plan golden files for all 5 engines
- [ ] 03-04-PLAN.md — AI mock HTTP server (3 modes), AI E2E tests, Dockerfile.mssql with AdventureWorksLT pre-loaded
- [ ] 03-05-PLAN.md — Docker layer caching for MSSQL image, JUnit XML output via gotestsum, E2E dev workflow documentation

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Monorepo Cleanup + Docker Infrastructure | 5/5 | Complete   | 2026-06-03 |
| 2. MySQL E2E Test Suite | 4/4 | Complete   | 2026-06-03 |
| 3. CI Automation + Cross-Engine Expansion | 0/5 | Not started | - |
