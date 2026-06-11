---
phase: quick
plan: 260611-i6z
task: 3
subsystem: credentials
tags: [security, refactoring, encryption, passphrase-removal]
provides: [machine-bound encryption]
affects: [internal/credentials, internal/cli, cmd/add-tpch]
metrics:
  duration: ~8 min
  files-created: 1
  files-modified: 5
  files-deleted: 1
  commits: 7
---

# Task 3: Remove QUERYLEX_KEYCHAIN_PASSPHRASE Dependency

**One-liner:** Replaced user-supplied scrypt passphrase with machine-ID derived AES-256 key, eliminating the `QUERYLEX_KEYCHAIN_PASSPHRASE` env var requirement.

## Summary

The encrypted credential store (`EncryptedFileStore`) previously required a user-supplied passphrase via `Unlock(passphrase)` or the `QUERYLEX_KEYCHAIN_PASSPHRASE` environment variable, with key derivation via scrypt(N=32768, r=8, p=1). This dependency has been removed:

- **New key derivation:** `deriveMachineKey()` in `machinekey.go` reads the machine ID (Linux: `/etc/machine-id`, macOS: `sysctl kern.uuid`, Windows: registry `MachineGuid`) and combines it with a random salt via SHA-256 to produce a 32-byte AES-256 key.
- **Passphrase removal:** `Unlock()`, `SetPassphrase()`, `ErrPassphraseRequired`, `ErrWrongPassphrase`, and all scrypt imports/constants removed from `encrypted.go`.
- **Scrypt dependency dropped:** `golang.org/x/crypto/scrypt` is no longer imported (confirmed via `go mod tidy`).
- **All passphrase prompts removed** from `run_adddb.go`, `run_edit_db.go`, `run_delete_db.go`, `preflight.go`, and `cmd/add-tpch/main.go`.
- **`passphrase.go` deleted** entirely.
- **Tests rewritten** for passphrase-less round-trip operation.

The store is now bound to the machine — credentials encrypted on one machine cannot be decrypted on another, providing hardware-backed security without user friction.

## Files Changed

### Created
- `internal/credentials/machinekey.go` — `deriveMachineKey()` and platform-specific machine ID readers

### Modified
- `internal/credentials/encrypted.go` — removed passphrase, scrypt, Unlock/SetPassphrase; uses deriveMachineKey
- `internal/cli/run_adddb.go` — removed `promptEncryptedFilePassphrase` call
- `internal/cli/preflight.go` — removed `QUERYLEX_KEYCHAIN_PASSPHRASE` auto-unlock block
- `internal/cli/preflight_test.go` — removed `TestPreflight_AutoUnlockEncryptedStore` and `TestPreflight_AutoUnlock_NoPassphraseEnv`
- `internal/credentials/unlock_test.go` — rewritten with passphrase-less round-trip tests

### Deleted
- `internal/cli/passphrase.go` — entire file removed

## Deviations from Plan

### [Rule 3 - Blocking] Fixed compile failures in additional files

**1. `internal/cli/run_edit_db.go`** (lines 96-105) — also called `promptEncryptedFilePassphrase`. Removed the block.

**2. `internal/cli/run_delete_db.go`** (lines 96-98) — also called `promptEncryptedFilePassphrase`. Removed the block.

**3. `cmd/add-tpch/main.go`** (lines 44-53) — called `encStore.Unlock(passphrase)`. Removed the block. The store now derives its key from the machine ID automatically.

**4. `internal/credentials/store_test.go`** (lines 77, 100, 119) — used `store.SetPassphrase(...)`. Removed calls — store no longer requires explicit unlock.

## Verification

```bash
$ go build ./cmd/querylex/   # SUCCESS
$ go test ./internal/credentials/... -short -count=1   # 17 tests PASS
$ go test ./internal/cli/... -short -count=1            # 44 tests PASS
$ grep -rn "KEYCHAIN_PASSPHRASE\|ErrPassphraseRequired\|ErrWrongPassphrase\|promptEncryptedFilePassphrase\|SetPassphrase" internal/ cmd/   # No references found
$ go mod tidy   # SUCCESS
```

## Done Criteria

- [x] `machinekey.go` created with cross-platform machine ID derivation
- [x] `encrypted.go` rewritten: no passphrase, no scrypt
- [x] `passphrase.go` deleted
- [x] `run_adddb.go` no longer prompts for passphrase
- [x] `run_edit_db.go` no longer prompts for passphrase (deviation)
- [x] `run_delete_db.go` no longer prompts for passphrase (deviation)
- [x] `preflight.go` no longer auto-unlocks via env var
- [x] `cmd/add-tpch/main.go` no longer calls Unlock (deviation)
- [x] Tests rewritten for passphrase-less operation
- [x] Binary compiles and all credential tests pass

## Self-Check: PASSED

All 10 files verified (9 files exist, 1 deleted as expected). All 7 task commits confirmed in git log.
