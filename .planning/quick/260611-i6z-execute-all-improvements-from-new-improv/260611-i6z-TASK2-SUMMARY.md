# Task 2: Improve Memory Search Relevance

**Phase:** Quick Task
**Subsystem:** `internal/memory/`, `internal/cli/`

## Summary

Improved memory search relevance by adding SQL structure scoring, lowering the match threshold, adding FTS5 full-text search fallback, adding token frequency tracking to skip stop words, generating bigram keys for better phrase matching, and decaying entries older than 90 days from results.

## Changes

### 2a. SQL Structure Scoring (`internal/memory/scoring.go`)
- Added `computeSQLStructureScore()` that extracts table names from the saved entry's SQL via `FROM`/`JOIN` regex and scores them against schema tokens present in the input
- Reduced entity overlap weight from 0.45 to 0.30 to accommodate the new 0.15 weight for sqlStructureScore
- New formula: `0.30*entityScore + 0.15*sqlStructureScore + 0.25*intentScore + 0.20*filterScore + 0.10*recencyScore`
- Updated doc comment from 4 to 5 components

### 2b. Lowered Threshold (`internal/cli/run_memory.go`)
- Changed similarity threshold from `0.86` to `0.60` to surface more approximate matches
- Updated doc comment

### 2c. FTS5 Support (`internal/memory/store.go`, `internal/memory/search.go`)
- Created `entries_fts` FTS5 virtual table referencing the `entries` content table
- Added `AFTER INSERT`, `AFTER DELETE`, `AFTER UPDATE` triggers to keep FTS5 index in sync
- Added `searchFTS()` function that tokenizes input, builds an FTS5 OR query, and returns matching entries ranked by relevance
- When keyword index produces no candidates, FTS5 is used as fallback before scanning all entries

### 2d. Token Frequency Map (`internal/memory/index.go`, `internal/memory/search.go`)
- Added `TokenFrequency map[string]int` field to `MemoryIndex` struct
- Track per-token frequency in `RebuildIndex()`
- In `Search()`, skip tokens with `frequency > 50` during candidate collection to filter out high-frequency stop words

### 2e. Bigram Tokenization (`internal/memory/index.go`, `internal/memory/search.go`)
- In `RebuildIndex()`, generate bigram keys with format `"__bigram__<token_i> <token_i+1>"` and add to keyword index
- In `Search()`, generate bigrams from input and look them up for better phrase matching

### 2f. 90-Day Entry Decay (`internal/memory/search.go`)
- After scoring and before sorting, filter out entries with `last_used_at` older than 90 days
- Falls back to `updated_at` if `last_used_at` is empty
- Entries with no date at all are kept

## Files Modified

| File | Changes |
|------|---------|
| `internal/memory/scoring.go` | Added sqlStructureScore, updated weights and doc |
| `internal/cli/run_memory.go` | Lowered threshold 0.86 → 0.60, updated doc |
| `internal/memory/store.go` | Added FTS5 table and trigger creation |
| `internal/memory/index.go` | Added TokenFrequency, bigram generation |
| `internal/memory/search.go` | Added searchFTS, token frequency filtering, bigram lookup, 90-day decay |

## Verification

- `go build ./internal/memory/` — PASS
- `go vet ./internal/memory/` — PASS
- `go test ./internal/memory/... -short -count=1` — PASS (no test files)

## Deviations

None — all tasks executed as planned.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 2a | `37b307e` | feat(memory-improve): add sqlStructureScore with 0.15 weight |
| 2b | `82c7121` | feat(memory-improve): lower memory match threshold from 0.86 to 0.60 |
| 2c | `e706caa` | feat(memory-improve): add FTS5 virtual table and fallback search |
| 2d | `4db4f58` | feat(memory-improve): add token frequency map and skip high-frequency tokens |
| 2e | `79baf69` | feat(memory-improve): add bigram tokenization to keyword index and search |
| 2f | `921a4e3` | feat(memory-improve): decay entries older than 90 days from search results |
