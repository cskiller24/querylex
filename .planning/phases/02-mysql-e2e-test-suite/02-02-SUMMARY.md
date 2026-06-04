---
phase: 02-mysql-e2e-test-suite
plan: 02
subsystem: testing
tags: [e2e, mysql, golden-file, adapter, validate, explain, joins, workspace-artifacts]

# Dependency graph
requires:
  - phase: 02-mysql-e2e-test-suite/plan-01
    provides: E2E test framework + golden file infrastructure + workspace helpers

provides:
  - MySQLAdapter.Validate() with DML keyword scanning + EXPLAIN-based validation
  - Differentiated error codes (TABLE_NOT_FOUND, COLUMN_NOT_FOUND, UNSAFE_SQL) in run_validate.go
  - MySQLAdapter.Explain() with FORMAT=JSON parsing and ANALYZE mode (TREE output)
  - MySQLAdapter.Joins() querying information_schema.KEY_COLUMN_USAGE for FK edges
  - Per-test DB name normalization (e2e_<hex> → e2e_test_db) in normalizeGoldenJSON
  - schema_map.json and join_graph.json workspace artifacts in setupE2EWorkspace

affects: [03-ci-automation-cross-engine-expansion]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MySQL EXPLAIN FORMAT=JSON recursive parsing with walkExplainNode"
    - "information_schema.KEY_COLUMN_USAGE FK joins with composite FK detection"
    - "Dual-layer validation: client-side keyword scan + server-side EXPLAIN"
    - "Golden file per-test DB name normalization via regex on raw string"

key-files:
  created: []
  modified:
    - "internal/db/mysql/adapter.go" — Validate, Explain, Joins implementations + dmlKeywords/isDML + explain JSON types/parsing + float64Ptr/int64Ptr
    - "internal/cli/run_validate.go" — Differentiated error code pass-through (TABLE_NOT_FOUND, COLUMN_NOT_FOUND, UNSAFE_SQL)
    - "internal/db/adapter_test.go" — Updated tests to match new implemented method behavior
    - "test/mysql/mysql_golden_test.go" — e2e_<hex> DB name normalization in normalizeGoldenJSON
    - "test/mysql/setup_test.go" — schema_map.json + join_graph.json artifact creation, updated manifest

key-decisions:
  - "MySQL EXPLAIN (FORMAT=JSON) for basic explain, EXPLAIN ANALYZE for runtime metrics with fallback to basic"
  - "Error code differentiation in run_validate.go via string pattern matching (doesn't exist → TABLE_NOT_FOUND, Unknown column → COLUMN_NOT_FOUND, DML/DCL → UNSAFE_SQL)"
  - "Empty join_graph.json with metadata (table_count=6) sufficient for stats artifact scan"
  - "E2E DB name regex pattern e2e_[0-9a-f]{32,} applied pre-JSON-parse for reliable normalization across all nesting depths"

patterns-established:
  - "MySQL adapter uses information_schema.KEY_COLUMN_USAGE with DATABASE() scope for FK detection"
  - "Validate follows dual-layer pattern: client-side keyword check (isDML) + server-side EXPLAIN"
  - "Explain follows adapter pattern: connection guard → deadline context → query → parse → return"

requirements-completed: [MYS-02, MYS-03, MYS-04, MYS-05, MYS-06, MYS-08]

# Metrics
duration: 3min
completed: 2026-06-04
---

# Phase 02 Plan 02: MySQL Adapter Method Implementations + E2E Artifact Fixes

**MySQLAdapter.Validate/Explain/Joins implementations replacing stubs, differentiated error codes in run_validate.go, golden file DB name normalization, and workspace artifact population for all 7 E2E test functions**

## Performance

- **Duration:** 3 min
- **Started:** 2026-06-04T04:45:58Z
- **Completed:** 2026-06-04T04:48:52Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- **MySQLAdapter.Validate()** with dual-layer validation: client-side DML keyword scan (returns UNSAFE_SQL) + server-side EXPLAIN-based validation (returns TABLE_NOT_FOUND, COLUMN_NOT_FOUND, or INVALID_SQL from run_validate.go mapping)
- **MySQLAdapter.Explain()** with FORMAT=JSON mode (recursive tree walk for full scans, index usage, sort/temp ops) and ANALYZE mode (TREE output in DialectRaw with FORMAT=JSON fallback)
- **MySQLAdapter.Joins()** querying information_schema.KEY_COLUMN_USAGE for FK edges with composite FK detection, grouped by CONSTRAINT_NAME
- **run_validate.go** differentiated error code pass-through: pattern-matches adapter error strings to assign TABLE_NOT_FOUND, COLUMN_NOT_FOUND, or UNSAFE_SQL codes
- **normalizeGoldenJSON** now replaces per-test database names (e2e_<32+ hex chars>) with stable "e2e_test_db" before JSON parsing, making golden file comparisons deterministic across runs
- **setupE2EWorkspace** now creates schema_map.json (6 employees DB tables with PK columns) and join_graph.json (empty edge list with metadata); manifest checksums include both new artifacts

## Task Commits

Each task was committed atomically:

1. **Task 1: MySQLAdapter.Validate() + run_validate.go error codes** - `733d2a4` (feat)
2. **Task 2: MySQLAdapter.Explain() + Joins()** - `8d8f5c7` (feat)
3. **Task 3: Golden normalization + workspace artifacts** - `034d878` (test)

**Plan metadata:** (pending metadata commit)

## Files Created/Modified

- **internal/db/mysql/adapter.go** — 931 lines total (was 579). Added: dmlKeywords + isDML; Validate() with DML check + EXPLAIN validation; Explain() with FORMAT=JSON parsing + ANALYZE TREE mode + walkExplainNode + mysqlExplainRoot/types + parseCost; Joins() with information_schema KEY_COLUMN_USAGE + composite FK detection; float64Ptr/int64Ptr helpers
- **internal/cli/run_validate.go** — Added "strings" import; replaced undifferentiated ErrCodeInvalidSQL with TABLE_NOT_FOUND/COLUMN_NOT_FOUND/UNSAFE_SQL mapping from adapter error strings
- **internal/db/adapter_test.go** — Updated TestAdapterMethods_ConcreteTypes and TestAdapterMethods_Implemented to match new non-stub method behavior
- **test/mysql/mysql_golden_test.go** — Added "regexp" import; added e2eDBPattern normalization (e2e_<hex> → e2e_test_db) before JSON unmarshal
- **test/mysql/setup_test.go** — Added "db" import; added WriteSchemaMap for 6-table SchemaMap with PK columns; added join_graph.json with empty edges and metadata; updated manifest checksums and TableCount

## Decisions Made

- **Dual-layer validation:** Client-side DML keyword scan returns UNSAFE_SQL immediately without DB connection. Server-side EXPLAIN-based validation detects schema errors (missing tables/columns). This matches PostgreSQL/SQLite adapter patterns.
- **Error differentiation in CLI layer:** String pattern matching in run_validate.go (not in the adapter) keeps the adapter database-agnostic and error-format-agnostic — the CLI layer owns the error code mapping.
- **EXPLAIN ANALYZE fallback:** If ANALYZE mode fails (MySQL < 8.0.18), gracefully falls back to FORMAT=JSON with a warning. The ANALYZE TREE output is stored in DialectRaw since it's not structured JSON.
- **Empty join_graph.json:** Sufficient for CLI subcommand success (joins command returns empty edges + warning; stats artifact scan passes). Full join graph from real FK discovery is populated by the indexing pipeline, not needed for E2E tests.
- **Regex applied pre-JSON-parse:** Normalizing database names on raw string before JSON unmarshal ensures ALL occurrences at any nesting depth are replaced — more reliable than post-parse tree walking.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated adapter_test.go to match implemented method behavior**
- **Found during:** Task 1 (Validate implementation)
- **Issue:** TestAdapterStubs_ConcreteTypes and TestAdapterStubs_ErrNotImplemented expected ErrNotImplemented from all three stubbed methods; Validate now returns a valid result without connection, Explain/Joins return ErrConnectionFailed
- **Fix:** Replaced both test functions with new tests reflecting implemented behavior: Validate returns success without connection; Explain/Joins return connection error
- **Files modified:** internal/db/adapter_test.go
- **Verification:** `go test -count=1 -run "TestAdapterMethods|TestDatabaseType|TestFactoryRegistration" ./internal/db/` passes
- **Committed in:** 733d2a4 (Task 1) and 8d8f5c7 (Task 2)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Test update was directly caused by replacing stubs with implementations — not scope creep. Tests now correctly validate the new behavior.

## Issues Encountered

None — all implementation and test updates completed cleanly.

## Threat Surface Scan

No new threat surface introduced beyond what was documented in the plan's threat model:
- EXPLAIN queries are read-only introspection (T-02-08 accept)
- 30s context deadline caps EXPLAIN execution (T-02-09 mitigate)
- e2e_<hex> regex pattern is specific to per-test UUID databases (T-02-10 mitigate)
- Synthetic checksums for CI-only test workspaces (T-02-11 accept)

## User Setup Required

None — no external service configuration required. Changes are entirely source code.

## Next Phase Readiness

- All 3 MySQL adapter methods (Validate, Explain, Joins) are fully implemented
- Golden file infrastructure now handles per-test DB name drift
- Workspace artifacts support all CLI subcommands in E2E tests
- Ready for full E2E test run against live MySQL to confirm all 7 test functions pass
- Phase 3 (CI Automation + Cross-Engine Expansion) can proceed with validated MySQL adapter patterns

## Self-Check: PASSED

- [x] internal/db/mysql/adapter.go — Validate/Explain/Joins implemented, no ErrNotImplemented stubs
- [x] internal/cli/run_validate.go — Differentiated error code pass-through added with "strings" import
- [x] internal/db/adapter_test.go — Updated tests pass
- [x] test/mysql/mysql_golden_test.go — regexp import + e2e_<hex> DB name normalization
- [x] test/mysql/setup_test.go — db import + schema_map.json + join_graph.json artifacts + manifest update
- [x] `go vet ./internal/db/mysql/ ./internal/cli/ ./test/mysql/` passes
- [x] `go build ./internal/db/mysql/ && go build ./internal/cli/` passes
- [x] `go test -tags e2e -c -o /dev/null ./test/mysql/` compiles
- [x] `make build && make build-test` produces bin/querylex and bin/e2e.test
- [x] 3 commits exist in git log (feat/feat/test)
- [x] No stub patterns (ErrNotImplemented) remain in adapter.go

---

*Phase: 02-mysql-e2e-test-suite*
*Plan: 02*
*Completed: 2026-06-04*
