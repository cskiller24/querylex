# Phase 4: AI Removal — Context

**Gathered:** 2026-06-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Delete the `internal/ai/` package and all AI-related code interleaved in retained files across the codebase. The phase covers:

1. **Delete `internal/ai/`** — 11 files (5 source + 5 test + 1 config): client, chat, embed, prompt, tokens, config
2. **Delete AI CLI handlers** — `internal/cli/run_sql.go`, `internal/cli/run_sql_test.go`, `internal/cli/run_optimize.go`, `internal/cli/run_ai_config.go`, `internal/cli/run_ai_config_test.go`
3. **Remove AI command registrations** — `sqlCmd`, `optimizeCmd`, `aiConfigCmd` var definitions, `AddCommand` calls, and function imports from `internal/rootcmd/root.go`
4. **Surgical edit preflight.go** — Remove `AIConfig` field from `PreflightResult`, remove `ai.ResolveAIConfig` call, remove `PreflightForAICommand` function and `AIPreflight` struct entirely, remove unused imports
5. **Remove AI error codes** — 4 constants (`ErrCodeAIServiceUnavailable`, `ErrCodeAIConfigMissing`, `ErrCodeAIGenerationFailed`, `ErrCodeAIContextOverflow`) and their descriptions from `internal/format/error.go`
6. **Remove AI key handling** — `envVarAIKey` constant and `case "ai-key"` branch from `internal/credentials/env.go`
7. **Remove AI passphrase branch** — `kind == "ai"` branch from `internal/cli/passphrase.go`
8. **Clean factory_test.go** — Remove AI key env var setup/teardown from `internal/credentials/factory_test.go`
9. **Memory dead code cleanup (MEMCLN-01)** — Remove `EmbeddingMetadata` struct, `EmbeddingVectors` map field, `SearchWithEmbeddings` function, `cosineSimilarity` function, `embeddingsActive` parameter branch, `EMBEDDINGS_UNAVAILABLE` warning from `internal/memory/`
10. **Dependency cleanup** — Remove `github.com/sashabaranov/go-openai` from go.mod
11. **Full build validation** — go build, go vet, go test, go mod tidy/verify, goreleaser snapshot

**NOT in scope for this phase:** E2E infrastructure removal (Phase 5), credential store restructuring (unchanged), schema indexing pipeline changes (unchanged), heuristic analysis changes (unchanged).

</domain>

<decisions>
## Implementation Decisions

### Deletion Strategy
- **D-01: Batch deletion** — Delete all AI files (`internal/ai/` + CLI handlers) in one command pass before editing retained files. The AI package is entirely isolated; the resulting compiler errors in retained files are well-understood (root.go command vars, preflight.go imports and AIConfig field) and fixed in a single subsequent pass. Incremental file-by-file deletion adds overhead without safety benefit.

### Dependency Cleanup Timing
- **D-02: Build-first, then tidy** — Remove `github.com/sashabaranov/go-openai` from go.mod ONLY after `go build ./...` passes. Never run `go mod tidy` before build passes — STATE.md explicitly warns that premature tidy may incorrectly remove `github.com/zalando/go-keyring` (still used by `credentials/keychain.go`). Sequence: build passes → `go mod tidy -v` → confirm only go-openai removed → `go mod verify`.

### Retained-File Cleanup Order
- **D-03: Edit all retained files in a single pass** — After AI files are deleted, edit all 5 retained files (root.go, preflight.go, error.go, env.go, passphrase.go) and memory/ together in one edit pass. No interleaved reordering needed — none of these edits depend on each other.

### Verification Order
- **D-04: vet before test** — Run `go vet ./...` before `go test ./...`. Full sequence: `go build ./...` → `go vet ./...` → `go test ./... -short -count=1` → `go mod tidy -v` + `go mod verify` → `goreleaser build --snapshot --clean --single-target`.

### PreflightForAICommand Removal
- **D-05: Full removal** — After AI CLI handlers are deleted, no remaining code calls `PreflightForAICommand` or references `AIPreflight`. Remove the function, the struct, and the import from preflight.go entirely.

### agent's Discretion
- The exact edit ordering within the single retained-files pass is left to the planner — none of the 5 retained files have cross-file edit dependencies.

### Folded Todos
None — no matching todos found.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — Requirement definitions AIPKG-01–03, AIEDIT-01–05, MEMCLN-01 (all scoped to Phase 4)
- `.planning/ROADMAP.md` §Phase 4 — Goal, dependency, success criteria (12 enumerated items), verification script

### Codebase Maps
- `.planning/codebase/ARCHITECTURE.md` — Component dependency graph (AI layer depends on cli/ via root.go and preflight.go)
- `.planning/codebase/STRUCTURE.md` — File layout for all affected directories
- `.planning/codebase/STACK.md` — Dependency stack showing go-openai usage

### Project State
- `.planning/STATE.md` — Contains critical warning about go-keychain retention risk; session continuity info
- `.planning/PROJECT.md` — Milestone goal, key decisions, constraints

### Affected Source Files (read before planning)
- `internal/rootcmd/root.go` — 3 AI command vars, 3 AddCommand calls, 3 function imports
- `internal/cli/preflight.go` — `AIConfig` field in `PreflightResult` (line 42), `ai.ResolveAIConfig` call (line 380), `PreflightForAICommand` (line 324), `AIPreflight` struct (line 317)
- `internal/format/error.go` — 4 AI error code constants (lines 56-63), 2 AI descriptions (lines 94-95)
- `internal/credentials/env.go` — `envVarAIKey` constant (line 26), `case "ai-key"` branch (line 44), `Available()` override (line 72)
- `internal/cli/passphrase.go` — `kind == "ai"` branch (line 31)
- `internal/credentials/factory_test.go` — `setEnvForTest` with aiKey param (lines 124-138), AI key assertions (line 83)
- `internal/memory/index.go` — `EmbeddingMetadata` struct (line 14), `EmbeddingVectors` field (line 26)
- `internal/memory/scoring.go` — `cosineSimilarity` (line 50), `embeddingsActive` branch in `ComputeSimilarity` (line 40)
- `internal/memory/search.go` — `SearchWithEmbeddings` (line 170), `embeddingsActive` references (lines 261-272)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Factory test pattern** (`internal/credentials/factory_test.go:setEnvForTest`) — The existing test helper sets DB and AI env vars; the AI parameter must be removed and any test case that depends on AI-only env setup must be updated.
- **root.go AddCommand pattern** — AI commands are registered in the same block as all other commands (lines 381-383); removal follows the established convention of grouping plus the convention of removing the var definitions (lines 284-312), the AddCommand calls, and the function-level imports.

### Established Patterns
- **Error code pattern** (`internal/format/error.go`) — ErrorCode constants with descriptions map. AI codes are 4 of 24 total; remove just the AI entries without changing the surrounding pattern.
- **preflight.go preflight variants** — Three preflight functions (`PreflightForCommand`, `PreflightForMemoryCommand`, `PreflightForAICommand`). Architecture analysis flags this as an anti-pattern (duplicated workspace loading logic). The AI variant removal should NOT introduce `PreflightForCommand` changes — just remove the AI variant cleanly.

### Integration Points
- **root.go** is the single point where AI commands are wired into the cobra tree — removing 3 command defs, 3 AddCommand calls, and 2 function imports (`cli.RunSQLGeneration`, `cli.RunOptimize`: root.go imports these; `cli.RunAIConfig` is called directly via `cli.RunAIConfig()`)
- **preflight.go** is where `ai` package import lives — removing it also removes the `AIConfig` coupling from `PreflightResult`
- **No transitive dependencies** — deleting AI files only breaks root.go, preflight.go, and the deleted CLI handler files. No other internal packages import `internal/ai`.

</code_context>

<specifics>
## Specific Ideas

No specific implementation preferences beyond the decisions above — standard batch-delete approach applies.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 4-AI Removal*
*Context gathered: 2026-06-07*
