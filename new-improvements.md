# Memory Search Improvements

## Remove Embeddings (Dead Code)

Embeddings are not and will never be used in QueryLex. The codebase references them in several places — remove all of it:

- **`internal/cli/run_save.go:117-125`** — Remove the `addEmbeddingsWarning()` function entirely.
- **`internal/cli/run_save.go:82`** — Remove the call to `addEmbeddingsWarning()`.
- **`internal/cli/run_memory.go:94`** — Remove the call to `addEmbeddingsWarning()`.
- **`internal/cli/run_history.go:117`** — Remove the call to `addEmbeddingsWarning()`.
- **`AGENTS.md`** — Remove references to `internal/ai/embed.go`, embedding models (`QUERYLEX_AI_EMBEDDING_MODEL`), and "embedding scoring" in the memory component description.
- **`internal/memory/scoring.go`** — The file already uses only lexical-only scoring; no changes needed here, but double-check there's no stale embedding reference.

## Improve Memory Search Relevance

### Problem
The current memory search (`internal/memory/search.go`) uses a keyword-based index (`memory_index.json`) with 4-component lexical scoring (`internal/memory/scoring.go`). The similarity threshold in `internal/cli/run_memory.go:66` is 0.86, which is too strict for many real queries. The keyword index has no ranking mechanism beyond token presence.

### Proposed Changes

1. **Add SQL query similarity to scoring** (`internal/memory/scoring.go`):
   - Parse the input for table/column names that match indexed schema tokens.
   - Add a `sqlStructureScore` component that compares table patterns in the query against tables used in saved entries (extract table names from saved SQL via regex).
   - Weight this at 0.15 (reducing entity overlap from 0.45 to 0.30).

2. **Lower the memory match threshold** (`internal/cli/run_memory.go:66`):
   - Change from 0.86 to 0.60. Observing real usage suggests that well-matched queries often score in the 0.60-0.85 range when input phrasing differs from saved input.

3. **Add FTS5 full-text search as a secondary pass** (`internal/memory/store.go`):
   - Create an `entries_fts` virtual table in the memory SQLite store.
   - Populate it with the `input` column of each entry.
   - Use FTS5 `MATCH` as a fallback when the keyword index produces no candidates, or as a boost factor in scoring.
   
4. **Improve the keyword index** (`internal/memory/index.go`):
   - Add a `tokenFrequency` map to the `MemoryIndex` struct that tracks how many times each token appears across entries.
   - When collecting candidates, skip tokens with frequency > 50 (stop words in the domain).
   - This prevents common words (like "show", "find", "all", "the") from flooding the candidate set.

5. **Add bigram tokenization** (`internal/memory/index.go:84-98`):
   - Generate bigrams from the input in addition to unigrams.
   - Store them in the keyword index with a special prefix (e.g., `__bigram__`).
   - This helps match multi-word concepts like "order date" vs "order" + "date" individually.

6. **Decay old entries** (`internal/memory/search.go`):
   - After scoring, apply a recency boost (already present at 0.10 weight).
   - Additionally, filter out entries with `last_used_at` older than 90 days from the top 5 results before returning.

## Remove QUERYLEX_KEYCHAIN_PASSPHRASE Dependency

The `QUERYLEX_KEYCHAIN_PASSPHRASE` env var should not be required when running querylex commands. If the user added a database via `add-db` (which interactively collects a passphrase and stores credentials), subsequent `querylex memory`, `querylex workspace-stats`, etc. should work without any env var.

**Problem:** The encrypted credential store (`EncryptedFileStore`) stays locked after creation. Every querylex command that needs credentials must re-unlock it — either via `QUERYLEX_KEYCHAIN_PASSPHRASE` env var or an interactive terminal prompt. This breaks non-interactive usage (scripts, CI, AI agents) unless the passphrase is smuggled via env var.

**Fix:** Derive the encryption key from a stable machine secret instead of a user-supplied passphrase. This means the encrypted file store is always unlockable on the same machine without any user interaction:

- On Linux, read `/etc/machine-id` (or `/var/lib/dbus/machine-id`).
- On macOS, read `kern.uuid` via sysctl.
- On Windows, read `MachineGuid` from the registry.
- Hash the machine secret with a random salt (per file write) using SHA-256 to produce the AES-256-GCM key.
- No more `scrypt`, no `Unlock()`/`SetPassphrase()`, no passphrase field on the store — it's always ready.

Existing encrypted files encrypted with the old passphrase-based key will need to be re-created (users re-run `add-db`). The file format stays the same (`[salt][nonce][ciphertext]`), only the key derivation changes.

**Files to modify:**
- **`internal/credentials/machinekey.go`** (new) — Cross-platform `deriveMachineKey(salt) ([]byte, error)` using machine ID + SHA-256.
- **`internal/credentials/encrypted.go`** — Remove `passphrase` field, `Unlock()`, `SetPassphrase()`, `scrypt` import; use `deriveMachineKey(salt)` in `getDerivedKey()` instead.
- **`internal/cli/passphrase.go`** — Delete entire file (no passphrase prompts needed).
- **`internal/cli/run_adddb.go:68-78`** — Remove the `promptEncryptedFilePassphrase` call block.
- **`internal/cli/preflight.go:171-185`** — Remove the auto-unlock block entirely.
- **`internal/credentials/unlock_test.go`** — Rewrite tests to work without passphrase (store/retrieve round-trips, tamper detection).
- **`internal/cli/preflight_test.go:330-413`** — Remove passphrase-dependent tests.
- **`cmd/setup-test-db/main.go:36-37`** — Remove `passphrase` and `os.Setenv("QUERYLEX_KEYCHAIN_PASSPHRASE", ...)`.

## Improve add-db Command Progress Reporting

After the user inputs credentials, the `add-db` command goes through several steps (connecting, storing credentials, running initial schema indexing) but provides no visibility into what's happening. The user is left staring at a blank terminal wondering if it's stuck.

**Proposed changes:**
- **`internal/cli/run_add_db.go`** — Add a progress reporter that prints each step as it executes:
  1. "Connecting to <database_type> at <host>:<port>..." → "Connected."
  2. "Storing credentials..." → "Credentials stored."
  3. "Fetching schema..." (with table count as it discovers them)
  4. "Indexing schema (this may take a moment)..." with phase indicators from `internal/index/pipeline.go`
  5. "Workspace saved."
- Use a spinner or step counter (`[1/5]`, `[2/5]`, etc.) so the user sees forward motion.
- On failure, show which step failed with a clear error message (not just a raw stack trace).
- **`cmd/querylex-add-db/main.go`** — Ensure the progress output goes to stderr so it doesn't interfere with JSON output on stdout.

## Improve workspace-stats Database Availability Check

The `workspace-stats` command reads the workspace manifest but never actually pings the databases to confirm they're reachable. A database listed as "active" might be down, unreachable, or have stale credentials.

**Proposed changes:**
- **`internal/cli/run_workspace_stats.go`** — During workspace stats collection, add a lightweight connectivity check for each registered database:
  1. Attempt a short timeout connection (e.g., 3s `Ping()` using each adapter's DSN).
  2. Report status per database: `online`, `unreachable`, `auth_failed`, `timeout`.
  3. If a database is unreachable, include the reason in the stats output (e.g., "connection refused", "invalid password").
  4. Cache the result for a short period (e.g., 30s) so repeated `workspace-stats` calls don't hammer the database.
- **`internal/db/adapter.go`** — Add a `TestConnect(ctx context.Context, dsn string, timeout time.Duration) error` method to the `Adapter` interface for lightweight connectivity checks without a full `Connect()` + `Ping()` cycle.

## Add DB Management Commands (edit / delete / list)

There is currently no way to edit an existing database entry or remove one via the CLI. Users must manually edit `database.json` or re-run `add-db` with the same name (which may orphan old credential references).

**Proposed new commands:**

### `querylex edit-db <id>`
- **`internal/cli/run_edit_db.go`** — Re-prompts the user for each field (DSN, database type, name, etc.) with the current value shown as default. Only saves changed fields.
- **`internal/state/workspace.go`** — Add `UpdateDatabase(id string, updates DatabaseEntry) error` to the `WorkspaceStore` interface.
- Credentials re-stored if changed; old credential reference cleaned up.

### `querylex delete-db <id>`
- **`internal/cli/run_delete_db.go`** — Delete a database entry by ID.
- **`internal/state/workspace.go`** — Add `DeleteDatabase(id string) error` to the `WorkspaceStore` interface.
- Clean up associated artifacts (schema cache, index files, memory entries for that database).
- Prompt for confirmation unless `--force`/`-y` flag is passed.
- **`internal/credentials/store.go`** — Add `Delete(reference CredentialReference) error` to clean up stored credentials when a database is removed.

### `querylex list-dbs`
- **`internal/cli/run_list_dbs.go`** — List all registered databases with their ID, type, host, status, and last connected timestamp.
- **`internal/rootcmd/root.go`** — Register all three new subcommands.


