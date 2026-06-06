# QueryLex

## What This Is

QueryLex is a context-aware SQL generator and optimizer CLI tool for AI agents. It provides a comprehensive command interface that helps agents introspect database schemas, generate SQL from natural language, optimize queries through explain-plan analysis, and maintain workspace state across multiple database connections. It exposes a structured JSON envelope for all deterministic commands and integrates with OpenAI-compatible APIs for AI-powered features.

Shipped v1.0 with a unified single-binary build, Docker Compose E2E infrastructure across 5 database engines (MySQL, PostgreSQL, MariaDB, MSSQL, SQLite), comprehensive E2E test suites with golden file validation, cross-engine SQL validation, EXPLAIN plan comparison, AI mock server for deterministic testing, and GitHub Actions CI automation.

## Core Value

AI agents can reliably introspect any supported database, generate correct SQL from natural language descriptions, and optimize queries — all through a single CLI tool with structured machine-readable output.

## Requirements

### Validated

- ✓ Database indexing pipeline (Louvain-based domain clustering, join graph inference, schema map, terminology templates) — existing
- ✓ Natural language resolver (multi-pass deterministic matching against schema metadata) — existing
- ✓ SQL validation (rejects DML/DCL, resolves tables/columns against schema) — existing
- ✓ EXPLAIN plan support across 5 database engines (MySQL, MariaDB, PostgreSQL, SQL Server, SQLite) — existing
- ✓ Query optimization analysis (context-weighted heuristics, rewrite strategies) — existing
- ✓ AI integration (OpenAI client with configurable provider, model, embeddings, token budgeting) — existing
- ✓ SQLite-backed memory store (similarity scoring, intent classification, recency decay) — existing
- ✓ Multi-backend credential management (OS keychain, encrypted file, env vars) — existing
- ✓ Workspace state management (atomic writes, file locking, crash recovery) — existing
- ✓ JSON-only deterministic commands with standard envelope format — existing
- ✓ Shell completions (bash, zsh, fish, PowerShell) — existing
- ✓ Response format specification with error code catalog — existing
- ✓ Explain cache (TTL-based with schema/stats/index invalidation) — existing
- ✓ Single-binary build — v1.0
- ✓ Docker Compose E2E infrastructure with real database containers — v1.0
- ✓ Real-world sample datasets for each supported database — v1.0
- ✓ Custom minimal test fixtures for targeted edge cases — v1.0
- ✓ Comprehensive E2E test suite covering all CLI subcommands — v1.0 (MySQL; cross-engine in Phase 3)

### Active

- [ ] E2E test maturity — make all engine suites pass reliably in CI on every commit
- [ ] Performance regression benchmarks — separate benchmark suite
- [ ] Full 17×5×3 test matrix on every commit
- [ ] Real AI API integration tests (nightly scheduled, not CI-blocking)
- [ ] Documentation improvements — user-facing docs for AI agent consumers

### Out of Scope

- Adding new database engines beyond the current 5 — not requested
- GUI or web interface — CLI-only, AI agent focus
- Real-time query monitoring or dashboard — out of scope for v1
- ORM generation or code-first schema design — not part of the vision

## Context

- Brownfield Go project with extensive existing functionality
- Module path: `github.com/cskiller24/querylex`
- Codebase: 213+ files, ~39,520 LOC
- Tech stack: Go, Cobra CLI, go-sql-driver/mysql, pgx, go-mssqldb, modernc.org/sqlite, go-openai
- Build via goreleaser with ldflags version injection
- All subcommands available under single `querylex` binary
- Codebase mapping complete (`.planning/codebase/`)
- Target users: AI agents consuming structured JSON output
- Shipped v1.0 milestone with unified build, Docker E2E infra, MySQL + cross-engine test suites, CI automation

## Constraints

- **Language**: Go (must stay Go, no language migration)
- **Database parity**: All 5 databases must remain supported with equal capability
- **Backward compatibility**: JSON envelope format and command interface must not break existing consumers
- **CGO_ENABLED=0**: Build constraint for portability (already configured in goreleaser)
- **Docker**: E2E testing must use docker-compose for database provisioning

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

---
*Last updated: 2026-06-06 after v1.0 milestone*
