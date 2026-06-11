# Quick Task 260611-i6z: Execute all improvements from new-improvements.md

**Status:** complete
**Date:** 2026-06-11
**Commit:** 3c58a8b

## Tasks Completed

| # | Category | Status | Commits |
|---|----------|--------|---------|
| 1 | Remove Embeddings Dead Code | ✅ | `feea744` |
| 2 | Improve Memory Search Relevance | ✅ | `37b307e`, `82c7121`, `e706caa`, `4db4f58`, `79baf69`, `921a4e3` |
| 3 | Remove QUERYLEX_KEYCHAIN_PASSPHRASE Dependency | ✅ | `e8f7735`, `6d50fd9`, `bebfd4c`, `1c97a69`, `a7d96f7`, `e965761`, `d365609` |
| 4 | Improve add-db Command Progress Reporting | ✅ | (included in task 3 commits) |
| 5 | Improve workspace-stats DB Availability Check | ✅ | `2fe1553` |
| 6 | Add DB Management Commands (edit-db, delete-db, list-dbs) | ✅ | `f19534c` |

## Verification

- `go build ./cmd/querylex/` — ✅
- `go vet ./...` — ✅
- `go test ./... -short -count=1` — all pass ✅
- No `addEmbeddingsWarning` / `EMBEDDINGS_UNAVAILABLE` references remain ✅
- No `KEYCHAIN_PASSPHRASE` references remain in code ✅
- Threshold changed from 0.86 to 0.60 ✅
- `edit-db`, `delete-db`, `list-dbs` visible in `--help` ✅
- `Connectivity` field in workspace-stats output ✅
