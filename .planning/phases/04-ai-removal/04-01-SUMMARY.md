---
phase: 04-ai-removal
plan: 01
subsystem: multiple
tags: go, cobra, credentials, memory, cleanup

requires:
  - Provides dependency graph context (no direct dependencies)
provides:
  - AI package (internal/ai/) completely deleted
  - AI CLI handler files deleted (run_sql, run_optimize, run_ai_config)
  - AI command registrations removed from root.go
  - AI references surgically removed from preflight, error codes, credentials env, passphrase, factory_test
  - AI embedding dead code removed from memory/ (index, scoring, search)
  - TestRenderStatsHuman_* tests migrated to run_stats_test.go
  - Full validation suite passing: build, test, vet, mod tidy, goreleaser
affects: [v1.1 documentation, project description updates]

tech-stack:
  added: []
  patterns: [Go build validation gate after each major step]
  removed:
    - github.com/sashabaranov/go-openai (removed by go mod tidy)

key-files:
  created:
    - internal/cli/run_stats_test.go (migrated TestRenderStatsHuman_* tests)
  modified:
    - internal/rootcmd/root.go (removed AI commands, updated descriptions)
    - internal/cli/preflight.go (removed AI import, AIPreflight struct, PreflightForAICommand)
    - internal/format/error.go (removed 4 AI error codes and descriptions)
    - internal/credentials/env.go (removed envVarAIKey, ai-key case, fallback, Available())
    - internal/cli/passphrase.go (removed kind=="ai" branch, simplified signature)
    - internal/cli/run_adddb.go (updated promptEncryptedFilePassphrase call)
    - internal/credentials/factory_test.go (setEnvForTest single param, 3 call sites)
    - internal/memory/index.go (removed EmbeddingMetadata, EmbeddingVectors)
    - internal/memory/scoring.go (removed embedding params, cosineSimilarity)
    - internal/memory/search.go (removed SearchWithEmbeddings, updated ComputeSimilarity call)
  deleted:
    - internal/ai/ directory (11 files)
    - internal/cli/run_sql.go, run_sql_test.go
    - internal/cli/run_optimize.go
    - internal/cli/run_ai_config.go, run_ai_config_test.go

key-decisions:
  - "kind parameter removed from promptEncryptedFilePassphrase — only caller (run_adddb.go) always passed 'database', no other callers exist"

patterns-established:
  - "Go build validation gate after each major deletion/editing step catches missing references immediately"

requirements-completed:
  - AIPKG-01
  - AIPKG-02
  - AIPKG-03
  - AIEDIT-01
  - AIEDIT-02
  - AIEDIT-03
  - AIEDIT-04
  - AIEDIT-05
  - MEMCLN-01

duration: 5min
completed: 2026-06-07
---

# Phase 4: AI Removal Summary

**Complete AI code removal: internal/ai/ package (11 files), 5 CLI handler files, AI command registrations, AI references in 5 retained files, and embedding dead code in memory/ — with full validation suite passing**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-07T02:31:35+08:00
- **Completed:** 2026-06-07T02:36:47+08:00
- **Tasks:** 3
- **Files modified:** 14 (16 deleted, 1 created, 31 total affected)

## Accomplishments
- Deleted entire `internal/ai/` package (11 files): client, chat, config, embed, prompt, tokens and their tests
- Deleted 5 AI CLI handler files: run_sql, run_optimize, run_ai_config and their tests
- Removed all AI command registrations from root.go (aiConfigCmd, sqlCmd, optimizeCmd) and updated descriptions
- Surgically removed AI references from 5 retained files: preflight.go, error.go, env.go, passphrase.go, factory_test.go
- Removed AI embedding dead code from memory package: EmbeddingMetadata struct, EmbeddingVectors field, cosineSimilarity function, SearchWithEmbeddings function
- Migrated TestRenderStatsHuman_* tests from run_sql_test.go to run_stats_test.go before deletion
- `go build ./...`, `go test ./... -short -count=1`, `go vet ./...`, `go mod tidy`, `go mod verify`, `goreleaser build --snapshot --clean --single-target` all pass

## Threat Surface

No new threat surface introduced — this phase only deleted code and performed surgical edits. No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries.

## Task Commits

Each task was committed atomically:

1. **Task 1: Delete AI package + CLI handlers + root command registrations + migrate tests** - `670a485` (feat)
2. **Task 2: Remove AI references from 5 retained files** - `707f890` (feat)
3. **Task 3: Memory embedding dead code cleanup + full validation** - `ec4ad08` (feat)

## Files Created/Modified
- `internal/cli/run_stats_test.go` - Migrated TestRenderStatsHuman_NoDatabases and TestRenderStatsHuman_WithDatabase tests
- `internal/rootcmd/root.go` - Removed AI command definitions/variables (aiConfigCmd, sqlCmd, optimizeCmd), removed AddCommand calls and flag registrations, updated Short/Long/Getting Started
- `internal/cli/preflight.go` - Removed ai import, AIPreflight struct, PreflightForAICommand function
- `internal/format/error.go` - Removed 4 AI error code constants and 4 ErrorCodeDescriptions entries
- `internal/credentials/env.go` - Removed envVarAIKey, "ai-key" case in Retrieve(), QUERYLEX_AI_KEY fallback read, simplified Available()
- `internal/cli/passphrase.go` - Removed kind=="ai" branch, QUERYLEX_AI_API_KEY reference, simplified altEnvVar; removed unused `kind` parameter
- `internal/cli/run_adddb.go` - Updated promptEncryptedFilePassphrase call (no kind argument)
- `internal/credentials/factory_test.go` - setEnvForTest takes single dbPass string, removed AI key set/unset logic, updated 3 call sites
- `internal/memory/index.go` - Removed EmbeddingMetadata struct and EmbeddingVectors field
- `internal/memory/scoring.go` - ComputeSimilarity now takes 4 lexical-only params; removed cosineSimilarity function
- `internal/memory/search.go` - Removed SearchWithEmbeddings function; updated Search comment and ComputeSimilarity call

## Decisions Made
- Removed `kind` parameter from `promptEncryptedFilePassphrase` since its only caller (`run_adddb.go`) always passed `"database"`. No other callers existed.

## Deviations from Plan
None - plan executed exactly as written.

One minor adjustment: the `"context"` import from `root.go` was removed during Task 2 build validation (Go compiler rejected unused import). This is a standard Go compilation fix, not a plan deviation — the plan's build step 2f naturally caught it.

## Issues Encountered
None.

## Known Stubs
None.

## Next Phase Readiness
- All AI code successfully removed from the codebase
- Full validation suite passes with zero errors
- Only remaining E2E/infra cleanup phases remain in Phase 4
- Ready for E2E infrastructure removal (next plan)

---
*Phase: 04-ai-removal*
*Completed: 2026-06-07*
