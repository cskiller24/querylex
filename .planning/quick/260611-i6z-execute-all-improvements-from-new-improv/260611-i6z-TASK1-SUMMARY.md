# Task 1: Remove Embeddings Dead Code — Summary

**Objective:** Remove the `addEmbeddingsWarning()` function and all embedding references from the codebase.

## Changes Made

### `internal/cli/run_save.go`
- Replaced `warning := addEmbeddingsWarning()` with `var warning []format.Warning` — the variable continues to accumulate warnings from the index-rebuild path.
- Deleted the entire `addEmbeddingsWarning()` function definition (17 lines) that returned a hardcoded `EMBEDDINGS_UNAVAILABLE` warning.

### `internal/cli/run_memory.go`
- Removed `resp.Warnings = addEmbeddingsWarning()` assignment at line 94 — the conditional `warning` append from `memory.Search()` continues to work.

### `internal/cli/run_history.go`
- Removed `resp.Warnings = addEmbeddingsWarning()` assignment at line 117 — same pattern as `run_memory.go`.

### `AGENTS.md`
- Removed `internal/ai/embed.go` and "and embeddings" from the `go-openai` dependency description.
- Removed the `QUERYLEX_AI_EMBEDDING_MODEL` config variable entry.
- Changed "embeddings, prompt builders" to "prompt builders" in the `ai` component description.
- Changed "embedding scoring" to "keyword scoring" in the `memory` component description.

## Verification
- `go build ./cmd/querylex/` — compiles cleanly
- `grep -rn "addEmbeddingsWarning\|EMBEDDINGS_UNAVAILABLE"` in CLI files — zero matches
- `grep -rn "embedding\|EMBEDDING\|embed.go"` in `AGENTS.md` — zero matches

## Commit
- `feea744` — `fix: remove addEmbeddingsWarning dead code and embedding references`
