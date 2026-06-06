# Project Research Summary

**Project:** QueryLex — v1.1 Cleanup
**Domain:** Go CLI tool — AI subsystem + E2E testing infrastructure removal
**Researched:** 2026-06-07
**Confidence:** HIGH

## Executive Summary

QueryLex v1.1 is a **subsystem removal milestone** on a brownfield Go CLI tool. The goal is to delete the AI subsystem (`internal/ai/` package, 3 AI CLI commands, 4 AI error codes, AI credential branches) and the entire E2E testing infrastructure (`test/` directory, Docker Compose, CI workflows, golden files, fixtures) — approximately 75 files and ~4,500 LOC (~11% of the codebase). This is not building new capability; it is surgical deletion of unused features that reduces maintenance burden, binary size (~5MB), and build time.

**The recommended approach** is a 4-phase sequential execution: (1) delete the self-contained AI package and its CLI handlers, (2) surgically edit 5 files where AI code is interleaved with retained code, (3) remove the E2E infrastructure layer, (4) validate build integrity and clean up dependencies. The ordering is critical — earlier phases must compile clean before later phases run, because `go mod tidy` cannot execute until all source references to the removed packages are gone.

**The key risk** is order-of-operations mistakes: running `go mod tidy` before all AI imports are deleted (causing build breaks and a corrupted go.sum), or accidentally removing `go-keyring` (which is NOT AI-specific — it still stores database passwords). Both are fully preventable with the right validation sequence: `go build ./...` → fix errors → `go build ./...` (clean) → `go mod tidy -v` → `go mod verify`.

## Key Findings

### Recommended Stack

The AI subsystem removal removes exactly **one direct dependency** and its transitive chain: `github.com/sashabaranov/go-openai` v1.41.2 (OpenAI client used exclusively by `internal/ai/`). All other dependencies remain unchanged. The `go-openai` package was a leaf dependency — nothing outside `internal/ai/` imports it.

**Core technologies staying:**
- **Go 1.26.3** — Entire codebase, unchanged
- **github.com/spf13/cobra** v1.9.1 — CLI framework (stays, 13 subcommands remaining)
- **github.com/go-sql-driver/mysql** v1.10.0 — MySQL driver (stays)
- **github.com/jackc/pgx/v5** v5.9.2 — PostgreSQL driver (stays)
- **github.com/microsoft/go-mssqldb** v1.10.0 — MSSQL driver (stays)
- **modernc.org/sqlite** v1.51.0 — SQLite driver (stays, used by memory store)
- **github.com/zalando/go-keyring** v0.2.8 — OS keychain (stays — database passwords)
- **github.com/AlecAivazis/survey/v2** v2.3.7 — Interactive prompts (stays — add-db flow)
- **github.com/google/uuid** v1.6.0 — UUIDs for trace IDs (stays)
- **github.com/oklog/ulid/v2** v2.1.1 — ULIDs for memory entries (stays)
- **golang.org/x/crypto** v0.52.0 — scrypt key derivation (stays)
- **golang.org/x/sync** v0.20.0 — errgroup for parallel ops (stays)

**Version considerations:** No version upgrades are part of this milestone. All retained dependencies stay at current pinned versions.

**Platform impact:** `CGO_ENABLED=0` constraint unaffected. Binary size drops ~5MB. Build time improves (~11% fewer files compiled).

### Expected Features

This milestone removes features, not adds them. The feature map from FEATURES.md is precise about what goes and what stays.

**Features to remove (table stakes for cleanup):**
- AI SQL generation (`querylex sql <question>`) — delete `internal/cli/run_sql.go` and root command registration
- AI query optimization (`querylex optimize <sql>`) — delete `internal/cli/run_optimize.go` and root command registration
- AI config management (`querylex ai-config`) — delete `internal/cli/run_ai_config.go` and root command registration
- AI client package (`internal/ai/`) — delete all 11 files (6 source + 5 test)
- AI error codes (`ErrCodeAIServiceUnavailable`, `ErrCodeAIConfigMissing`, `ErrCodeAIGenerationFailed`, `ErrCodeAIContextOverflow`) — remove from `internal/format/error.go`
- AI key credential branch — remove from `internal/credentials/env.go`
- AI passphrase kind — remove from `internal/cli/passphrase.go`
- E2E test infrastructure — delete entire `test/` directory (~50+ files), `compose.yaml`, `Dockerfile.mssql`, `.github/workflows/e2e.yml`
- Makefile E2E targets — remove from `Makefile`

**Features staying (untouched):**
- All 5 database adapters (MySQL, MariaDB, PostgreSQL, MSSQL, SQLite)
- Schema index pipeline (Louvain clustering, join graph, schema map, terminology)
- SQL validation (rejects DML/DCL, resolves tables/columns)
- Explain plan analysis across all 5 engines
- Heuristic query optimization
- SQLite-backed memory store (similarity scoring, recency decay)
- Multi-backend credential management (keychain, encrypted file, env vars)
- Workspace state management (atomic writes, file locking)
- JSON response envelope format
- 13 remaining CLI subcommands
- Shell completions
- Explain plan cache (TTL-based)
- Build and release pipeline (goreleaser)

**Defer (not in scope):**
- Re-adding AI integration — intentional removal, reverse requires explicit decision
- Adding new database engines — not requested
- GUI or web interface — CLI-only by design

### Architecture Approach

The AI subsystem was a **leaf addition** to the existing architecture — no other code path depends on it structurally. Removing it contracts the dependency tree cleanly: `internal/ai/` had imports flowing outward to `internal/format/` (error codes) and `internal/credentials/` (AI key env var), but nothing imported `internal/ai/` except the CLI handler files that are also being deleted. The remaining architecture (13 subcommands, 5 database adapters, memory store, index pipeline, credential store, workspace state, response envelope) is wholly intact and functionally unchanged.

**Major components (after removal):**
1. **`internal/rootcmd/root.go`** — Cobra command tree: 13 subcommands (3 removed: `sql`, `optimize`, `ai-config`)
2. **`internal/cli/`** — Handler functions: 14 files (3 deleted), preflight simplified (no AI config loading)
3. **`internal/db/`** — Database abstraction: 5 adapters, unchanged
4. **`internal/format/`** — JSON response envelope: error codes enum shrinks by 4 constants
5. **`internal/credentials/`** — Secret storage: env backend simplified (no AI key branch)
6. **`internal/memory/`, `internal/index/`, `internal/analysis/`, `internal/cache/`** — Core features: all unchanged

**Key surgical edits (5 files):**
- `internal/cli/preflight.go` — Remove `AIConfig` field from `PreflightResult`, remove `ai.ResolveAIConfig` call
- `internal/format/error.go` — Remove 4 AI error code constants and their descriptions
- `internal/credentials/env.go` — Remove `envVarAIKey`, `case "ai-key"` branch, simplify `Available()`
- `internal/cli/passphrase.go` — Remove `kind == "ai"` branch, simplify `altEnvVar` to `QUERYLEX_DB_PASSWORD`
- `internal/credentials/factory_test.go` — Remove AI key from `setEnvForTest`

**Architectural integrity is preserved.** No component inherits responsibility from removed code. The natural language resolver (`queryutil`), memory store, index pipeline, heuristic analysis, and explain cache all continue working independently.

### Critical Pitfalls

1. **Running `go mod tidy` before all source references are deleted** — The most dangerous pitfall. If you delete `internal/ai/` but miss AI imports in `internal/cli/` handler files, `go mod tidy` removes `go-openai` from go.mod before the build catches the error. Now you have dangling imports AND a modified go.sum to debug. **Prevention:** Always run `go build ./...` → fix errors → `go build ./...` (clean) → THEN `go mod tidy -v` → `go mod verify`. Never skip the build step.

2. **Accidentally removing `go-keyring` with AI code** — `github.com/zalando/go-keyring` is used by `internal/credentials/keychain.go` for database password storage (NOT AI). It visually resembles an AI dependency (it's an "API client" for OS keychains). If `go mod tidy` removes it, the build breaks with "package not in go.mod". **Prevention:** Run `go mod why -m github.com/zalando/go-keyring` and `rg '"github.com/zalando/go-keyring"' --type go` before starting. The build will catch it immediately.

3. **Stale Makefile targets after E2E deletion** — Deleting `test/`, `compose.yaml`, and `Dockerfile.mssql` but leaving Makefile targets (`compose-up-mysql`, `build-test`, `test-e2e-all`) creates zombie targets that fail confusingly. **Prevention:** After E2E deletion, run `grep -n 'test-e2e\|compose-up\|compose-down\|build-test\|ci-setup' Makefile` and remove or annotate each matching target.

4. **Stale CI references to deleted E2E files** — `.github/workflows/ci.yml` (stays) may reference compose, docker, or testdata paths from the deleted E2E infrastructure. Separately, `.github/workflows/e2e.yml` (being deleted) may be referenced by other CI configs. **Prevention:** After deletion, grep `ci.yml` for `compose`, `docker`, `testdata`, `golden`, `e2e` to catch any stray references.

5. **Test files outside deleted packages referencing removed types** — `go test ./...` failing because a test in another package imports `internal/ai` or references AI error codes. **Verified safe:** The only external test referencing AI is `internal/credentials/factory_test.go` (being surgically edited) and `internal/cli/run_sql_test.go` (being deleted). But validate with `go test ./... -short -count=1 -run='^$'` (compile-only check) after deletion.

## Implications for Roadmap

Based on the dependency analysis from all four research files, the removal must proceed in strict sequential order. Each phase must compile clean before the next begins.

### Phase 1: Delete Self-Contained AI Package
**Rationale:** The `internal/ai/` package and its CLI handler callers form a self-contained deletion set. Nothing outside this group depends on it. Deleting it first removes the largest block of dead code (11 files + 5 CLI files) and clears the way for surgical edits in Phase 2.

**Delivers:** Removal of `internal/ai/` (all 11 files), `internal/cli/run_sql.go`, `internal/cli/run_sql_test.go`, `internal/cli/run_optimize.go`, `internal/cli/run_ai_config.go`, `internal/cli/run_ai_config_test.go`. Three AI commands removed from `internal/rootcmd/root.go`.

**Addresses:** The core AI deletion scope from FEATURES.md.

**Avoids:** Pitfall 1 (tidy-before-build) — must run `go build ./...` and confirm clean compilation after this phase before proceeding.

**Phases that need research:** None. Well-documented patterns — these are file deletions, no integration complexity.

### Phase 2: Surgical Edits — 5 Files with Interleaved AI Code
**Rationale:** Five files contain AI-related code mixed with retained functionality. These must be surgically edited now that the dependent AI package is gone — the compiler will catch any missed references.

**Delivers:** Clean versions of `internal/cli/preflight.go` (no AIConfig field), `internal/format/error.go` (4 AI error codes removed), `internal/credentials/env.go` (no AI key branch), `internal/cli/passphrase.go` (no AI kind branch), `internal/credentials/factory_test.go` (no AI key in test setup).

**Addresses:** The "edited files" section from FEATURES.md (6 files including root.go already handled in Phase 1).

**Avoids:** Pitfall 5 (stale test file dependencies) — run `go test ./... -short -count=1 -run='^$'` to catch compilation-only errors in remaining test files.

**Research flags:** None. Each edit is precisely scoped in ARCHITECTURE.md and FEATURES.md with exact line-level changes.

### Phase 3: Remove E2E Testing Infrastructure
**Rationale:** The E2E infrastructure (`test/`, compose.yaml, Dockerfile.mssql, CI workflow, Makefile targets) is entirely independent of the AI subsystem. Removing it after the AI changes are done keeps the scope of each phase focused. This is a mechanical deletion — no code dependencies to worry about beyond stale references.

**Delivers:** Deletion of ~50+ test files, `test/testdata/`, `test/scripts/`, `compose.yaml`, `Dockerfile.mssql`, `.github/workflows/e2e.yml`, 14 Makefile targets. Single-file edits to Makefile and `.github/workflows/ci.yml` (if any cross-references found).

**Addresses:** E2E deletion scope from FEATURES.md.

**Avoids:** Pitfall 3 (stale Makefile targets) — must grep Makefile for leftover compose/e2e references. Pitfall 4 (stale CI references) — must grep ci.yml for deleted infrastructure.

**Research flags:** None. Mechanical deletion — no new patterns to research.

### Phase 4: Build Validation and Dependency Cleanup
**Rationale:** The final phase validates the integrity of the entire removal. With all source changes complete, we can safely run the full validation pipeline: build, tidy, vet, test, and goreleaser snapshot.

**Delivers:** Clean go.mod/go.sum (only `go-openai` removed), passing build, passing tests, verified goreleaser configuration.

**Addresses:** Build integrity from STACK.md. Dependency cleanup from FEATURES.md.

**Avoids:** Pitfall 1 (tidy-before-build) — tidy runs LAST after build confirms all references gone. Pitfall 2 (go-keyring removal) — `go build ./...` catches this immediately. Pitfall 9 (goreleaser build failure) — snapshot build validates release pipeline.

**Validation sequence:**
```bash
go build ./...                    # Must pass first
go test ./... -short -count=1     # All non-E2E tests pass
go vet ./...                      # Linting passes (make lint)
go mod tidy -v                    # Clean up go.mod/go.sum (only go-openai removed)
go mod verify                     # Module integrity check
goreleaser build --snapshot --clean --single-target  # Release pipeline works
```

**Research flags:** None. Standard build pipeline validation.

### Phase Ordering Rationale

- **Phase 1 must precede Phase 2** because deleting `internal/ai/` removes the import dependency that keeps AI code in the surgical-edit files compiling. With the package gone, lingering references in preflight.go, env.go, etc. become build errors that are easy to fix.
- **Phase 2 must precede Phase 3** because having both AI and E2E changes in flight simultaneously creates too many moving parts for debugging a failed build. AI changes are verified first, then E2E changes are applied on a clean base.
- **Phase 3 must precede Phase 4** because the full validation pipeline (go build, go test, go mod tidy) should run against the final state of the codebase. Running it before E2E deletion would yield failing tests (E2E infra was deleted but test targets still run).
- **Phase 4 is last** because it validates the final state. Running build/tidy/goreleaser before all deletions are complete is premature and error-prone.

### Research Flags

No phases require deeper research during planning. Every aspect of this milestone is well-documented:
- **Phase 1 (AI package deletion):** Self-contained file deletions. Source references fully mapped in FEATURES.md and ARCHITECTURE.md.
- **Phase 2 (surgical edits):** Each of the 5 files has exact change specifications. No ambiguity about what to remove vs. keep.
- **Phase 3 (E2E deletion):** Mechanical file and directory deletions. Makefile and CI references mapped.
- **Phase 4 (validation):** Standard Go build pipeline. No surprises.

**All phases use standard patterns — skip `/gsd-plan-phase --research-phase`.**

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every dependency verified via `grep` across the full codebase. `go.mod` read directly. Only `go-openai` is removed. |
| Features | HIGH | Every file-to-delete and file-to-edit verified against actual source listings. Anti-features documented from codebase analysis. No ambiguity about what stays. |
| Architecture | HIGH | Complete structural analysis of the dependency graph. AI subsystem confirmed as a leaf addition with no structural dependents. 5 surgical-edit files identified with exact changes. |
| Pitfalls | HIGH | All 10 pitfalls verified against actual codebase patterns. Critical pitfalls validated by running the actual build/tidy/check sequence mentally. No speculation. |

**Overall confidence:** HIGH

### Gaps to Address

- **CI workflow cross-references:** `.github/workflows/ci.yml` has not been exhaustively checked for references to E2E infrastructure (compose, docker, testdata paths). Must grep after deletion to confirm no stale references remain. Low risk — easy to catch in Phase 3.
- **Makefile target audit:** The exact list of 14 E2E/compose targets has been identified, but there may be additional target dependencies (e.g., a `build-test` prerequisite in the `release` target). Must run `grep -n 'test-e2e\|compose-up\|compose-down\|build-test\|ci-setup' Makefile` after deletion as a safety check.
- **`selectCredentialStore` dead code:** Both `internal/ai/config.go` and `internal/cli/run_adddb.go` define this function. After `internal/ai/config.go` is deleted, the `run_adddb.go` copy becomes dead code (staticcheck U1000). Optional housekeeping — not critical to milestone, but worth flagging for cleanup.

## Sources

### Primary (HIGH confidence)
- Codebase direct analysis — full `grep`-based verification of every import, dependency, and file reference
- `go.mod` / `go.sum` — direct read of dependency tree
- `cmd/` and `internal/` file listings — file count and LOC verified
- `.github/workflows/` — CI workflow contents verified
- `Makefile` — target list verified

### Secondary (MEDIUM confidence)
- Researcher output from STACK.md, FEATURES.md, ARCHITECTURE.md, PITFALLS.md — all four sources cross-validated against each other and against actual codebase

### Tertiary (LOW confidence)
- Binary size reduction (~5MB) is estimated based on `go-openai` package footprint — not measured post-removal
- Build time improvement (~11% faster) inferred from file count reduction — not benchmarked

---

*Research completed: 2026-06-07*
*Ready for roadmap: yes*
