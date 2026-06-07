# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-06-06
**Phases:** 3 | **Plans:** 13 | **Sessions:** ~6

### What Was Built

- Unified single-binary build (removed standalone `querylex-add-db` and `querylex-stats`)
- Docker Compose E2E infrastructure across 5 database engines with healthchecks, tmpfs, random ports
- Test helper package with Connect\* functions, port wait, retry/backoff, per-test DB isolation
- Sample dataset provisioning scripts (Employees DB, Chinook, Pagila, Northwind) + custom fixtures
- MySQL E2E test suite (7 test functions, ~60 sub-tests) with golden file validation
- MySQL adapter gap closure — implemented Validate/Explain/Joins with differentiated error codes
- Cross-engine E2E test packages for PostgreSQL, MariaDB, MSSQL, SQLite
- Cross-engine SQL validation matrix (12 patterns × 5 engines)
- EXPLAIN plan golden file infrastructure per engine
- AI mock server for deterministic E2E testing of AI features
- GitHub Actions CI with 5-engine matrix, Docker caching, JUnit XML output
- Pre-built MSSQL Docker image with AdventureWorksLT for faster CI

### What Worked

- **Brownfield-first approach**: The project had extensive existing Go code; infrastructure-first (Phase 1: build + Docker) was the right starting point
- **MySQL-first E2E strategy**: Proving the pattern with one engine (MySQL) before expanding to 4 others avoided premature complexity
- **Docker Compose with profiles**: Clean multi-engine provisioning without over-engineering — each engine is a profile, testing is a targeted `compose up`
- **Golden file pattern**: Normalizing volatile fields (trace IDs, timestamps, random DB names) makes deterministic comparison possible
- **Per-test DB isolation**: UUID-named databases with `t.Cleanup()` drop prevents data leakage without complex setup/teardown

### What Was Inefficient

- **Debug session metadata cleanup**: 7 debug sessions from Phase 2 investigation work were left with stale "investigating" status — required manual cleanup at milestone close
- **REQUIREMENTS.md traceability drift**: Requirements were implemented across phases but the traceability table was never updated from "Pending" to "Complete" — caught only at milestone archival
- **UAT gap tracking**: Phase 3 UAT identified a branch reference issue (CI workflows listening on `main` instead of `master`) which was fixed but the UAT status was never updated to "passed"
- **Quick task artifacts**: Quick task 260606-jkr had both PLAN.md and SUMMARY.md but audit tool still flagged it — indicates a tooling metadata sync gap

### Patterns Established

- **Phase structure**: CONTEXT.md → PLAN.md → execute → SUMMARY.md works well for brownfield Go projects
- **Blueprint**: Phase 1 infra → Phase 2 single-engine → Phase 3 cross-expansion+CI pattern is reusable for multi-engine projects
- **UAT early**: Running UAT at phase boundaries catches issues (branch naming, CI triggers) that otherwise go unnoticed
- **MVP mode for test phases**: Phase 2 used MVP mode (RED test → GREEN infra → complete coverage) which accelerated iteration

### Key Lessons

1. Update REQUIREMENTS.md traceability table immediately when a requirement is shipped — it becomes painful to reconstruct at milestone close
2. Mark debug sessions as "resolved" as soon as the fix is verified — stale metadata accumulates quickly
3. Quick tasks need a secondary status indicator beyond SUMMARY.md frontmatter — the audit tool relies on frontmatter parsing but detected a "missing" status anyway (possibly a frontmatter parsing edge case with leading dashes in directory names)
4. The infra-first, expand-later pattern (Phase 1 build + Docker → single engine → cross-engine) is an effective template for similar Go CLI projects with multiple backends

### Cost Observations

- Model mix: N/A (not tracked for this milestone)
- Sessions: ~6 execution sessions + 1 milestone completion session
- Notable: First milestone completed entirely via GSD workflow automation

## Milestone: v1.1 — Cleanup

**Shipped:** 2026-06-07
**Phases:** 2 | **Plans:** 2 | **Sessions:** ~2

### What Was Built

- Deleted entire `internal/ai/` package (11 files), 5 AI CLI handlers, and root command registrations
- Surgically removed AI references from 5 retained files (preflight, error codes, credentials, passphrase, factory_test)
- Removed AI embedding dead code from memory package (EmbeddingMetadata, cosineSimilarity, SearchWithEmbeddings)
- Deleted entire `test/` directory hierarchy (~74 files across 10 subdirectories)
- Removed Docker Compose (`compose.yaml`), MSSQL Dockerfile, and E2E CI workflow
- Cleaned Makefile (15 E2E/compose targets), README.md (55 lines), AGENTS.md, preflight.go

### What Worked

- **Incremental build validation gate**: Running `go build ./...` after each major deletion step caught the unused `"context"` import in root.go immediately — same pattern as v1.0 Phase 4
- **Simple two-phase scope**: AI removal first, E2E removal second — minimal interdependency, clean separation
- **Full pipeline validation at the end**: Build + test -short + vet + mod tidy + mod verify + goreleaser all pass with zero errors
- **Same pattern for both phases**: Delete → build validate → next delete → full pipeline validated the incremental approach

### What Was Inefficient

- **Audit tool filename mismatch**: The audit tool expected `SUMMARY.md` but quick tasks use `{quick_id}-SUMMARY.md` naming — caught and fixed at milestone close (same issue as v1.0 retrospective)
- **REQUIREMENTS.md traceability never updated**: All 13 requirements shipped but traceability table still showed "Pending" — same issue identified in v1.0 retrospective (lesson not applied)
- **No UAT or verification beyond build validation**: Phases were simple deletions but formal UAT/verification docs existed — acceptable for removal scope

### Patterns Established

- **Deletion-first cleanup pattern**: For removal phases, incremental `go build ./...` gates after each step provides immediate feedback
- **Binary audit fix renamed**: Renamed `{quick_id}-SUMMARY.md` to `SUMMARY.md` to match audit tool convention — document this pattern for future quick tasks

### Key Lessons

1. Update REQUIREMENTS.md traceability table immediately when a requirement ships — this is the second milestone where it was left as "Pending"
2. Quick task SUMMARY.md files should be named `SUMMARY.md` (not `{quick_id}-SUMMARY.md`) to match the audit tool's filename convention
3. Incremental build validation is effective for deletion phases — apply to all future cleanup/removal work

### Cost Observations

- Sessions: ~2 execution sessions + 1 milestone completion session
- Notable: Entire milestone was pure deletion — zero new code added beyond migrated tests

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | ~7 | 3 | Initial GSD workflow adoption |
| v1.1 | ~3 | 2 | Cleanup milestone — AI + E2E removal |

### Cumulative Quality

| Milestone | Tests | LOC | Zero-Dep Additions |
|-----------|-------|-----|-------------------|
| v1.0 | ~100+ E2E sub-tests | ~39,520 | All dependencies via go.mod |
| v1.1 | 0 E2E (removed), ~50 unit | ~27,800 | go-openai removed |

### Top Lessons (Verified Across Milestones)

1. Keep REQUIREMENTS.md traceability in sync with shipped work — do it per-plan, not at milestone close (still not applied after v1.1 — needs workflow enforcement)
2. Close UAT and debug sessions immediately when fixes land — stale metadata compounds
3. Quick task SUMMARY.md files should be named `SUMMARY.md`, not `{quick_id}-SUMMARY.md` — audit tool expects the former
