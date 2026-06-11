---
phase: task6
plan: add-db-management-commands
subsystem: cli, state, rootcmd
tags: [db-management, edit-db, delete-db, list-dbs]
requires: []
provides: [edit-db, delete-db, list-dbs commands]
affects: [internal/state/workspace.go, internal/cli, internal/rootcmd]
tech-stack:
  added: []
  patterns: [interactive-edit-prompt, confirm-prompt]
key-files:
  created:
    - internal/cli/run_edit_db.go
    - internal/cli/run_delete_db.go
    - internal/cli/run_list_dbs.go
  modified:
    - internal/cli/prompts.go (added PromptDatabaseEdit, PromptConfirm)
    - internal/state/workspace.go (added UpdateDatabase, DeleteDatabase, ErrDatabaseNotFound)
    - internal/rootcmd/root.go (registered three new commands)
decisions: []
metrics:
  duration: 15m
  completed_date: "2026-06-11"
---

# Task 6: Add DB Management Commands

Implements edit-db, delete-db, and list-dbs commands for managing database connections in the QueryLex workspace.

## Verification

```bash
go build ./cmd/querylex/
go vet ./...
go run ./cmd/querylex/ --help | grep -E "edit-db|delete-db|list-dbs"
```

All three commands are visible in the `--help` output and handle arguments/flags correctly.

## Summary of Changes

### 6a. WorkspaceStore additions (`internal/state/workspace.go`)

- **`UpdateDatabase(id, entry)`** — Replaces an entry in ConnectedDatabases by ID. Returns `ErrDatabaseNotFound` if not found.
- **`DeleteDatabase(id)`** — Removes entry from ConnectedDatabases, clears active if needed. Returns `ErrDatabaseNotFound` if not found (unlike `RemoveDatabase` which is a no-op).
- **`ErrDatabaseNotFound`** — New sentinel error for missing database ID lookups.

### 6b. Credential store `Delete` ✅ (already implemented)

- `KeychainStore.Delete` — removes from OS keychain
- `EncryptedFileStore.Delete` — removes from encrypted file
- `EnvStore.Delete` — returns error (env vars are read-only)

### 6c. `internal/cli/run_edit_db.go`

- **`RunEditDB(id string)`** — Interactive editor for database connections
  - Loads workspace and finds entry by ID
  - Loads database config with current values
  - Prompts user with current values as defaults (name, host, port, database, username)
  - Empty password = keep existing; non-empty = store new, delete old
  - Updates both database.json and workspace entry (preserving status/progress)

### 6d. `internal/cli/run_delete_db.go`

- **`RunDeleteDB(id string, force bool)`** — Removes database connection entirely
  - Loads workspace, finds entry by ID
  - If not `--force`, prompts for confirmation via `PromptConfirm`
  - Deletes credential from store on confirmation
  - Removes from workspace via `DeleteDatabase`
  - Cleans up artifacts: `os.RemoveAll(~/.querylex/<id>)`

### 6e. `internal/cli/run_list_dbs.go`

- **`RunListDBs()`** — Lists all connected databases with details
  - Types: `ListDBsData` (Databases, Count), `DBListItem` (ID, Name, Type, Host, Port, Database, Username, SSLMode, Status, IsActive)
  - Reads `database.json` for each entry to show connection details
  - Marks active database with `is_active`
  - Results sorted alphabetically by name

### 6f. Command registration (`internal/rootcmd/root.go`)

- `list-dbs` — no arguments, no flags
- `edit-db <id>` — single argument (database ID)
- `delete-db <id>` — single argument, `--force`/`-y` flag
- All commands follow the standard pattern: `start := time.Now()`, `outputResponse(resp)`, `os.Exit(1)` on failure

### Prompt additions (`internal/cli/prompts.go`)

- **`PromptDatabaseEdit(current *DBConnectionConfig)`** — Re-prompts with current values as defaults; password field allows empty to preserve existing
- **`PromptConfirm(message, defaultYes)`** — Generic yes/no confirmation prompt using `survey.Confirm`

## Deviations from Plan

None — plan executed as written.

## Self-Check: PASSED

- Files exist: ✅ `internal/cli/run_edit_db.go`, `run_delete_db.go`, `run_list_db.go`
- Commands visible in `--help`: ✅
- Build succeeds: ✅ `go build ./cmd/querylex/`
- Vet passes: ✅ `go vet ./cmd/querylex/ ./internal/cli/ ./internal/rootcmd/ ./internal/state/`
