# Phase 4: AI Removal - Context

**Gathered:** 2026-06-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Delete all AI-related code from the QueryLex codebase: the `internal/ai/` package (11 source + test files), AI CLI handler files (run_sql.go, run_optimize.go, run_ai_config.go), AI root command registrations (sqlCmd, optimizeCmd, aiConfigCmd), AI code interleaved in 5 retained files (preflight, error codes, credentials env, passphrase, factory_test), and AI-embedding dead code from `internal/memory/`. Full build validation after all deletions.

**9 requirements covered:** AIPKG-01, AIPKG-02, AIPKG-03, AIEDIT-01, AIEDIT-02, AIEDIT-03, AIEDIT-04, AIEDIT-05, MEMCLN-01

**Out of scope (addressed in Phase 5):** E2E infrastructure removal (test/ directory, Docker Compose, CI workflow, Makefile targets).
</domain>

<decisions>
## Implementation Decisions

### Plan Breakdown
- **D-01:** Single plan covering all 4 work types + build validation. No need to split into multiple plans — the phase is linear and well-understood.
- **D-02:** Deletion order: (1) delete `internal/ai/` directory first, then (2) delete AI CLI handler files + root command registrations, then (3) surgical edits to 5 retained files, then (4) memory dead code cleanup.
- **D-03:** Incremental build validation — run `go build ./...` after each major step (after ai/ deletion, after CLI handler deletion, after surgical edits). Catches issues immediately at the point of introduction.
- **D-04:** `go mod tidy -v` and `goreleaser build --snapshot --clean --single-target` run only after all deletions are complete and all intermediate builds pass. Never tidy before build passes.

### Critical Ordering Constraints
- AI package MUST be deleted before surgical edits — compiler catches lingering references to deleted package.
- `go mod tidy` must NOT be run before `go build ./...` passes — premature tidy may incorrectly remove the `go-keyring` dependency (which is still needed by credentials/keychain.go).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements
- `.planning/REQUIREMENTS.md` §AI Package Removal, §AI Surgical Edits, §Memory Dead Code Cleanup — Detailed requirements (AIPKG-01–03, AIEDIT-01–05, MEMCLN-01) with specific files and symbols to delete
- `.planning/ROADMAP.md` §Phase 4 — Success criteria (12 items), verification script

### Project Context
- `.planning/STATE.md` §Accumulated Context — Ordering constraints (AI package before surgical edits, go mod tidy after build)
- `.planning/PROJECT.md` §Constraints — Build constraints (CGO_ENABLED=0), backward compatibility requirements

### Codebase Architecture
- `.planning/codebase/ARCHITECTURE.md` §Component Responsibilities — Shows where AI code connects: ai/ package, CLI handlers, root command registrations, interleaved code in preflight/credentials/passphrase
- `.planning/codebase/STACK.md` §Key Dependencies — `github.com/sashabaranov/go-openai` is the dependency to be removed by go mod tidy
- `.planning/codebase/CONVENTIONS.md` §Package Organization — Directory listing confirms internal/ai/ entry point for deletion

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Cobra command pattern** in `internal/rootcmd/root.go` — AI commands (sqlCmd, optimizeCmd, aiConfigCmd) follow the same structure as retained commands. Removal follows the established pattern: delete variable definitions, AddCommand calls, and function imports.
- **Preflight pattern** in `internal/cli/preflight.go` — PreflightForAICommand is the target for deletion. The preflight layer has a documented copy-paste anti-pattern (3 variants), but this phase only removes the AI variant — not a refactor.
- **Error code pattern** in `internal/format/error.go` — AI error codes (ErrCodeAIServiceUnavailable, etc.) follow the same sentinel-error pattern as DB error codes. Removal: delete the constants and their ErrorCodeDescriptions entries.

### Established Patterns
- **Preflight → execute → respond** — Every command handler follows this pattern. AI commands are the only ones using PreflightForAICommand; removing them eliminates the only caller.
- **Blank-import init pattern** — AI package is not blank-imported (unlike db adapters). It is imported directly by CLI handler files being deleted.
- **Survey/v2 usage** — Used by both run_ai_config.go (delete target) and prompts.go (retained for add-db). Survey dependency is retained; no migration needed.

### Integration Points (Files to Modify)
- `internal/cli/run_sql.go` — Delete entire file
- `internal/cli/run_sql_test.go` — Delete entire file
- `internal/cli/run_optimize.go` — Delete entire file
- `internal/cli/run_ai_config.go` — Delete entire file
- `internal/cli/run_ai_config_test.go` — Delete entire file
- `internal/rootcmd/root.go` — Remove sqlCmd, optimizeCmd, aiConfigCmd definitions, AddCommand calls, and function imports
- `internal/cli/preflight.go` — Remove AIConfig field from PreflightResult, remove ai.ResolveAIConfig call
- `internal/format/error.go` — Remove 4 AI error code constants and their descriptions
- `internal/credentials/env.go` — Remove envVarAIKey constant, case "ai-key" branch from Retrieve() and Available()
- `internal/cli/passphrase.go` — Remove `kind == "ai"` branch
- `internal/credentials/factory_test.go` — Remove AI key env var and AI-related assertions
- `internal/memory/*.go` — Remove EmbeddingMetadata, EmbeddingVectors, SearchWithEmbeddings, cosineSimilarity, embeddingsActive, EMBEDDINGS_UNAVAILABLE

</code_context>

<specifics>
## Specific Ideas

- Build validation follows the "incremental compile check" pattern: build after each major deletion step catches issues immediately where they occur.
- Single plan approach keeps coordination simple — no inter-plan dependencies to manage within the phase.
- The verification script from ROADMAP.md §Phase 4 should be the plan's final step.
</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.
</deferred>

---

*Phase: 4-AI Removal*
*Context gathered: 2026-06-07*
