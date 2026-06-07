# Roadmap: QueryLex

## Milestones

- ✅ **v1.0 MVP** — Phases 1-3 (shipped 2026-06-06)
- 🚧 **v1.1 Cleanup** — Phases 4-5 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-3) — SHIPPED 2026-06-06</summary>

- [x] Phase 1: Monorepo Cleanup + Docker Infrastructure (5/5 plans) — completed 2026-06-03
- [x] Phase 2: MySQL E2E Test Suite (2/2 plans) — completed 2026-06-04
- [x] Phase 3: CI Automation + Cross-Engine Expansion (6/6 plans) — completed 2026-06-03

</details>

### 🚧 v1.1 Cleanup (In Progress)

**Milestone Goal:** Remove AI-related code and E2E testing infrastructure to reduce maintenance burden and simplify the codebase.

- [x] **Phase 4: AI Removal** — Delete AI package, CLI handlers, root command registrations, surgical edits in 5 retained files, and memory dead code cleanup. Full build validation. (completed 2026-06-06)
- [ ] **Phase 5: E2E Infrastructure Removal** — Delete test/ directory, Docker Compose files, CI workflow, and E2E Makefile targets.

## Phase Details

### Phase 4: AI Removal
**Goal**: All AI-related code is removed from the codebase — the internal/ai/ package, AI CLI handlers and command registrations, AI code interleaved in retained files (preflight, error codes, credentials, passphrase), and dead embedding code in memory/. The project compiles, tests pass, and go.mod is clean.
**Depends on**: Nothing (v1.1 first phase)
**Requirements**: AIPKG-01, AIPKG-02, AIPKG-03, AIEDIT-01, AIEDIT-02, AIEDIT-03, AIEDIT-04, AIEDIT-05, MEMCLN-01
**Success Criteria** (what must be TRUE):
  1. `internal/ai/` directory does not exist (all 11 source + test files deleted)
  2. `internal/cli/run_sql.go`, `internal/cli/run_sql_test.go`, `internal/cli/run_optimize.go`, `internal/cli/run_ai_config.go`, `internal/cli/run_ai_config_test.go` do not exist
  3. `internal/rootcmd/root.go` has no `sqlCmd`, `optimizeCmd`, or `aiConfigCmd` variable definitions, `AddCommand` calls, or function imports
  4. `PreflightResult` struct has no `AIConfig` field; preflight sequence has no `ai.ResolveAIConfig` call
  5. `internal/format/error.go` contains no AI error code constants or their descriptions
  6. `internal/credentials/env.go` has no `envVarAIKey` constant, no `case "ai-key"` branch
  7. `internal/cli/passphrase.go` has no `kind == "ai"` branch
  8. `internal/credentials/factory_test.go` has no AI key env var or AI-related assertions
  9. `internal/memory/` has no `EmbeddingMetadata`, `EmbeddingVectors`, `SearchWithEmbeddings`, `cosineSimilarity`, `embeddingsActive`, or `EMBEDDINGS_UNAVAILABLE` references
  10. `go build ./...`, `go test ./... -short -count=1`, `go vet ./...` all pass
  11. `go mod tidy -v` removes only `github.com/sashabaranov/go-openai`; `go mod verify` passes
  12. `goreleaser build --snapshot --clean --single-target` succeeds

**Verification**:
```bash
! test -d internal/ai/ && \
! test -f internal/cli/run_sql.go && \
! test -f internal/cli/run_optimize.go && \
! test -f internal/cli/run_ai_config.go && \
! grep -q 'sqlCmd\|optimizeCmd\|aiConfigCmd' internal/rootcmd/root.go && \
! grep -q 'AIConfig\|ResolveAIConfig' internal/cli/preflight.go && \
! grep -q 'ErrCodeAI' internal/format/error.go && \
! grep -q 'envVarAIKey\|"ai-key"' internal/credentials/env.go && \
! grep -q 'kind == "ai"' internal/cli/passphrase.go && \
! grep -q 'ai_key\|AI_KEY' internal/credentials/factory_test.go && \
! grep -q 'EmbeddingMetadata\|EmbeddingVectors\|SearchWithEmbeddings\|cosineSimilarity\|embeddingsActive\|EMBEDDINGS_UNAVAILABLE' internal/memory/*.go && \
go build ./... && \
go test ./... -short -count=1 && \
go vet ./... && \
go mod tidy -v && \
go mod verify && \
goreleaser build --snapshot --clean --single-target 2>&1 | tail -5
```
**Plans**: 1 plan
**UI hint**: no

**Plans:**
- [x] 04-01-PLAN.md — Delete AI package, CLI handlers, root registrations, surgical edits in 5 retained files, memory dead code cleanup, full build validation

---

### Phase 5: E2E Infrastructure Removal
**Goal**: All E2E testing infrastructure is deleted — test directory, Docker Compose files, CI workflow, and Makefile targets. No stale references to deleted infrastructure remain. Build passes.
**Depends on**: Phase 4
**Requirements**: E2ERM-01, E2ERM-02, E2ERM-03, E2ERM-04
**Success Criteria** (what must be TRUE):
  1. `test/` directory hierarchy does not exist
  2. `compose.yaml` and `Dockerfile.mssql` do not exist
  3. `.github/workflows/e2e.yml` does not exist
  4. Makefile has no `test-e2e`, `compose-up`, `compose-down`, `build-test`, or `ci-setup` targets
  5. `.github/workflows/ci.yml` has no stale references to `compose`, `docker`, `testdata`, `golden`, or `e2e` paths
  6. `go build ./...` passes

**Verification**:
```bash
! test -d test/ && \
! test -f compose.yaml && \
! test -f Dockerfile.mssql && \
! test -f .github/workflows/e2e.yml && \
! grep -qE 'test-e2e|compose-up|compose-down|build-test|ci-setup' Makefile && \
! grep -qE 'compose|docker.*test|testdata|golden|e2e' .github/workflows/ci.yml && \
go build ./...
```
**Plans**: 1 plan
**UI hint**: no

**Plans:**
- [ ] 05-01-PLAN.md — Delete test/ directory, compose.yaml, Dockerfile.mssql, e2e.yml, Makefile targets, doc references; full build validation

---

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Monorepo Cleanup + Docker Infrastructure | v1.0 | 5/5 | Complete | 2026-06-03 |
| 2. MySQL E2E Test Suite | v1.0 | 2/2 | Complete | 2026-06-04 |
| 3. CI Automation + Cross-Engine Expansion | v1.0 | 6/6 | Complete | 2026-06-03 |
| 4. AI Removal | v1.1 | 1/1 | Complete   | 2026-06-06 |
| 5. E2E Infrastructure Removal | v1.1 | 0/1 | Not started | - |

---

*Archived milestone details: .planning/milestones/v1.0-ROADMAP.md*
*Archived requirements: .planning/milestones/v1.0-REQUIREMENTS.md*
