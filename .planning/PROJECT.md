# QueryLex

## What This Is

QueryLex is a context-aware SQL introspection and optimization CLI tool for AI agents. It provides a comprehensive command interface that helps agents introspect database schemas, validate SQL, analyze query plans, and maintain workspace state across multiple database connections. It exposes a structured JSON envelope for all deterministic commands.

Shipped v1.0 with a unified single-binary build, Docker Compose E2E infrastructure across 5 database engines, and comprehensive E2E test suites. Shipped v1.1 with AI-related code and E2E testing infrastructure removed, reducing maintenance burden and simplifying the codebase for long-term maintainability.

## Core Value

AI agents can reliably introspect any supported database, generate correct SQL from natural language descriptions, and optimize queries — all through a single CLI tool with structured machine-readable output.

## Current State

**Shipped:** v1.1 Cleanup (2026-06-07) — AI code and E2E testing infrastructure removed.
**Previous:** v1.0 MVP (2026-06-06) — Unified build, Docker Compose E2E infrastructure, cross-engine test suites.

## Requirements

### Validated

- ✓ Database indexing pipeline (Louvain-based domain clustering, join graph inference, schema map, terminology templates) — v1.0
- ✓ Natural language resolver (multi-pass deterministic matching against schema metadata) — v1.0
- ✓ SQL validation (rejects DML/DCL, resolves tables/columns against schema) — v1.0
- ✓ EXPLAIN plan support across 5 database engines (MySQL, MariaDB, PostgreSQL, SQL Server, SQLite) — v1.0
- ✓ Query optimization analysis (context-weighted heuristics, rewrite strategies) — v1.0
- ✓ SQLite-backed memory store (similarity scoring, intent classification, recency decay) — v1.0
- ✓ Multi-backend credential management (OS keychain, encrypted file, env vars) — v1.0
- ✓ Workspace state management (atomic writes, file locking, crash recovery) — v1.0
- ✓ JSON-only deterministic commands with standard envelope format — v1.0
- ✓ Shell completions (bash, zsh, fish, PowerShell) — v1.0
- ✓ Response format specification with error code catalog — v1.0
- ✓ Explain cache (TTL-based with schema/stats/index invalidation) — v1.0
- ✓ Single-binary build — v1.0
- ✓ AI package removal (internal/ai/ — 11 files, 5 CLI handlers, root command registrations) — v1.1
- ✓ AI surgical edits (preflight, error codes, credentials env, passphrase, factory_test) — v1.1
- ✓ Memory dead code cleanup (embedding metadata, vectors, cosine similarity, search) — v1.1
- ✓ E2E infrastructure removal (test/ directory, compose.yaml, Dockerfile.mssql, e2e.yml) — v1.1
- ✓ Makefile E2E target cleanup and documentation updates — v1.1

### Active

- [ ] Schema migration or diff tooling
- [ ] Query logging and audit trail

### Out of Scope

- Adding new database engines beyond the current 5 — not requested
- GUI or web interface — CLI-only, AI agent focus
- Real-time query monitoring or dashboard — out of scope
- ORM generation or code-first schema design — not part of the vision
- Performance regression benchmarks — deferred, no benchmark suite needed
- Adding AI functionality back — intentional removal, reverse requires explicit decision
- E2E test infrastructure — intentionally removed in v1.1

## Context

- Brownfield Go project with extensive existing functionality
- Module path: `github.com/cskiller24/querylex`
- Codebase: 200+ files, ~27,800 LOC (after v1.1 cleanup removed ~11,700 LOC)
- Tech stack: Go, Cobra CLI, go-sql-driver/mysql, pgx, go-mssqldb, modernc.org/sqlite
- Build via goreleaser with ldflags version injection
- All subcommands available under single `querylex` binary
- Codebase mapping complete (`.planning/codebase/`)
- Target users: AI agents consuming structured JSON output
- Shipped v1.0 with unified build, Docker E2E infra, cross-engine test suites
- Shipped v1.1 with AI code removed, E2E infrastructure removed, codebase simplified

## Constraints

- **Language**: Go (must stay Go, no language migration)
- **Database parity**: All 5 databases must remain supported with equal capability
- **Backward compatibility**: JSON envelope format and command interface must not break existing consumers
- **CGO_ENABLED=0**: Build constraint for portability (already configured in goreleaser)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Remove standalone binaries | `add-db` and `workspace-stats` already work as subcommands; standalone wrappers add build complexity | ✓ Delivered |
| Docker-compose for E2E | Simplifies spinning up all 5 databases for integration tests | ✓ Delivered |
| Real-world + custom test data | Real datasets catch edge cases; custom fixtures target specific code paths | ✓ Delivered |
| AI agents as primary users | Shapes output format (structured JSON), command design, and documentation | ✓ Good |
| MySQL-first E2E testing | Simplest path, proves the pattern before expanding | ✓ Delivered |
| Per-test database isolation | UUID-based names, t.Cleanup drop — prevents data leakage | ✓ Delivered |
| CI matrix-per-DB pattern | Avoids Docker resource exhaustion; fail-fast disabled | ✓ Delivered |
| AI mock server (httptest.Server) | Instead of real API calls for deterministic testing | ✓ Delivered |
| AI package removal | Delete internal/ai/, 5 CLI handlers, root command registrations | ✓ v1.1 |
| AI surgical edits | Remove AI references from 5 retained files | ✓ v1.1 |
| Memory dead code cleanup | Remove embedding metadata, vectors, cosine similarity | ✓ v1.1 |
| E2E infrastructure removal | Delete test/, compose.yaml, e2e.yml, Makefile targets | ✓ v1.1 |
| Incremental build validation | Run `go build ./...` after each major deletion step | ✓ v1.1 — caught unused import in root.go |
## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---

*Last updated: 2026-06-07 after v1.1 milestone completion*
