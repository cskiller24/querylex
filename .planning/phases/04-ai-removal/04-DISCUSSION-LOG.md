# Phase 4: AI Removal - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-07
**Phase:** 4-AI Removal
**Areas discussed:** Plan Breakdown

---

## Plan Breakdown

| Option | Description | Selected |
|--------|-------------|----------|
| Single plan | One plan handles everything — delete, edit, tidy, validate | ✓ |
| Two plans | Plan 1: Package + CLI deletion. Plan 2: Surgical edits + memory cleanup + validation | |
| Three plans | Plan 1: Package + CLI deletion. Plan 2: Surgical edits. Plan 3: Memory cleanup + validation | |
| You decide | Let the planner figure out the optimal breakdown | |

### Sub-decision: Deletion Order

| Option | Description | Selected |
|--------|-------------|----------|
| Delete ai/ package first | Compiler catches lingering references. Follows STATE.md guidance. | ✓ |
| Delete CLI handlers first | Remove run_sql.go, run_optimize.go, run_ai_config.go first, then ai/ | |

### Sub-decision: Build Validation Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Incremental builds | Run go build ./... after each major step | ✓ |
| Build only at the end | Make all changes, build once | |

### Sub-decision: go mod tidy / goreleaser Timing

| Option | Description | Selected |
|--------|-------------|----------|
| After all deletions + builds pass | go mod tidy, verify, goreleaser only at end | ✓ |
| Tidy incrementally | Tidy after ai/ deletion, then again at end | |

**User's choice:** Single plan
**Notes:** User selected single plan with ai/ deletion first, incremental builds, final go mod tidy and goreleaser.

---

## Agent's Discretion

No areas deferred to the agent.

## Deferred Ideas

None.
