---
phase: "260611-i6z"
plan: "TASK4"
subsystem: "cli"
tags: ["add-db", "progress", "ux"]
requires: []
provides: ["add-db-step-indicators"]
affects: ["internal/cli/run_adddb.go"]
tech-stack: {}
key-files:
  created: []
  modified: ["internal/cli/run_adddb.go"]
decisions: []
metrics:
  duration: "0m (pre-committed in 1c97a69)"
  completed: "2026-06-11"
---

# Task 4: Improve add-db Progress Reporting

**One-liner:** Added `[1/5]` through `[5/5]` step-indicator progress messages (stderr) to `RunAddDB()` with `[FAIL]` fallbacks and periodic progress polling from `index_status.json`.

## Summary

All required progress-reporting changes were already present in HEAD commit `1c97a69` (part of a prior TASK3 refactoring). Verification confirms the implementation matches the plan spec:

- `progressStep(step, total, message)` helper prints structured `[N/5]` messages to stderr
- `[1/5] Connecting to <type> at <host>:<port>... Connected.` — printed after `adapter.Connect()` succeeds (line 118)
- `[2/5] Storing credentials... Credentials stored.` — printed after `credStore.Store()` succeeds (line 79)
- `[3/5] Fetching schema...` — printed before `index.NewPipeline` (line 218)
- `[4/5] Indexing schema... N%` — goroutine with `time.NewTicker(2s)` polls `index.ReadIndexStatus` during pipeline execution (lines 224–239)
- `[5/5] Workspace saved.` — printed after workspace update completes (line 268)
- `[FAIL] <step>: <error>` — printed for connection (line 110), credential storage (line 70), and indexing (line 243) failures
- All output goes to `os.Stderr`, preserving JSON envelope on stdout

## Verification

```bash
$ go build ./cmd/querylex/
# → exit 0, no errors
```

## Commits

No new commits needed — all changes were already present in `1c97a69`.

## Deviations

None — the plan was already fully implemented.
