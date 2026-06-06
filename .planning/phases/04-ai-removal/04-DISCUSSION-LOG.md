# Phase 4: AI Removal - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-07
**Phase:** 4-AI Removal
**Areas discussed:** Deletion Strategy, Dependency Cleanup Timing, Retained-File Cleanup Order, Verification Order, PreflightForAICommand Removal
**Mode:** `--auto` (fully autonomous — recommended option selected for each area)

---

## Deletion Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Batch deletion | Delete all AI files in one command pass, then fix retained files | ✓ |
| Incremental deletion | Delete file by file, compiling after each | |

**Selection:** Batch deletion (recommended default)
**Rationale:** AI package is entirely isolated; compiler errors in retained files are predictable. Incremental adds overhead without safety benefit.

---

## Dependency Cleanup Timing

| Option | Description | Selected |
|--------|-------------|----------|
| Build-first, then tidy | Remove go-openai only after `go build ./...` passes | ✓ |
| Tidy-first | Run `go mod tidy` as soon as AI files are deleted | |

**Selection:** Build-first, then tidy (recommended default)
**Rationale:** STATE.md explicitly warns that premature tidy may remove go-keyring. Build-first guarantees only go-openai is cleaned up.

---

## Retained-File Cleanup Order

| Option | Description | Selected |
|--------|-------------|----------|
| Single pass | Edit all 5 retained files + memory in one edit pass | ✓ |
| Sequential passes | Edit one file at a time, compiling after each | |

**Selection:** Single pass (recommended default)
**Rationale:** None of the retained-file edits depend on each other; no benefit to interleaving.

---

## Verification Order

| Option | Description | Selected |
|--------|-------------|----------|
| vet before test | go build → go vet → go test → go mod tidy → goreleaser | ✓ |
| test before vet | go build → go test → go vet → go mod tidy → goreleaser | |

**Selection:** vet before test (recommended default)
**Rationale:** Vet catches compilation issues faster than running the full test suite.

---

## PreflightForAICommand Removal

| Option | Description | Selected |
|--------|-------------|----------|
| Full removal | Delete function, AIPreflight struct, and import entirely | ✓ |
| Keep stub | Leave empty function returning nil (paranoid safety) | |

**Selection:** Full removal (recommended default)
**Rationale:** No remaining callers after AI CLI handler deletion. Keeping a stub adds dead code.

---

## the agent's Discretion

- Exact edit ordering within the retained-files single pass — no cross-file dependencies, planner can decide.

## Deferred Ideas

None — discussion stayed within phase scope.

---

*Phase: 4-AI Removal*
*Discussion logged: 2026-06-07*
