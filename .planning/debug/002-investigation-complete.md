---
status: resolved
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:30:00Z
---

# Investigation Complete

## Summary

Investigated 7 UAT issues across 5 debug sessions. Below is the consolidated root cause analysis with structured YAML entries ready to insert into the UAT's Gaps section.

---

## GAP 1 (Test 1): Overall E2E suite partial failures

**Root cause:** Aggregate of all 6 underlying root causes below. 5 of 7 test functions fail due to:
- MySQL adapter `Validate()`, `Explain()`, `Joins()` are stubs returning `ErrNotImplemented`
- Golden file normalization misses the per-test UUID database name field
- Synthetic workspace artifacts are missing required files (schema_map.json, join_graph.json)

**Debug sessions:**
- `.planning/debug/002-exitcode-validation.md`
- `.planning/debug/003-golden-normalization.md`
- `.planning/debug/004-unimplemented-adapters.md`
- `.planning/debug/005-validate-inconsistency.md`
- `.planning/debug/006-synthetic-workspace-artifacts.md`
- `.planning/debug/007-output-clarity.md`

**YAML gap entry:**
```yaml
- truth: "All ~60 sub-tests pass across 7 test functions"
  status: failed
  reason: "5 of 7 test functions fail due to 6 root causes: (a) MySQLAdapter.Validate/Explain/Joins are stubs returning ErrNotImplemented, (b) normalizeGoldenJSON doesn't normalize per-test UUID database name in schema field, (c) setupE2EWorkspace creates only empty schema.json, missing schema_map.json and join_graph.json, (d) output clarity is consequence of 5/7 failures"
  severity: major
  test: 1
  artifacts:
    - internal/db/mysql/adapter.go
    - test/mysql/mysql_golden_test.go
    - test/mysql/setup_test.go
  missing:
    - MySQL adapter Validate/Explain/Joins implementation in internal/db/mysql/adapter.go
    - Schema database name normalization in test/mysql/mysql_golden_test.go
    - Complete synthetic workspace artifacts in test/mysql/setup_test.go
```

## GAP 2 (Test 3): validate subcommand inconsistent between direct and flag tests

**Root cause:** Both `TestMySQLValidation` and `TestMySQLFlags/validate_basic` call the identical `RunValidate → PreflightForCommand → runValidateWithAdapter → adapter.Validate()` code path. `MySQLAdapter.Validate()` (internal/db/mysql/adapter.go:339-341) returns `db.ErrNotImplemented`. The reported inconsistency is a misattribution — both tests fail identically with `INVALID_SQL`. The root cause is the same as GAP 4/GAP 6 (Validate): the method is unimplemented.

In `runValidateWithAdapter` (internal/cli/run_validate.go:55-65), any error from `adapter.Validate()` is unconditionally mapped to `ErrCodeInvalidSQL`:
```go
result, err := adapter.Validate(ctx, query)
if err != nil {
    resp := format.NewErrorResponse[ValidateData](
        format.ErrCodeInvalidSQL,
        fmt.Sprintf("SQL validation failed: %v", err),
        ...
    )
}
```

Layer 1 DML/DCL safety checks (via `queryutil.ValidateSQLSafety`) work correctly and produce `UNSAFE_SQL` for INSERT/UPDATE/DELETE/GRANT. Layer 2 adapter validation is missing entirely.

**Debug session:** `.planning/debug/005-validate-inconsistency.md`

**YAML gap entry:**
```yaml
- truth: "SQL validation passes valid query through querylex validate subcommand"
  status: failed
  reason: "MySQLAdapter.Validate() (internal/db/mysql/adapter.go:339-341) is a stub returning db.ErrNotImplemented. ALL calls return 'SQL validation failed: not implemented in this adapter version' with error.code INVALID_SQL. There is no schema-aware validation to differentiate valid SQL from invalid references. Both direct validation test and flag test fail identically — the reported inconsistency is a misattribution."
  severity: major
  test: 3
  artifacts:
    - internal/db/mysql/adapter.go:339-341
    - internal/cli/run_validate.go:55-65
  missing:
    - MySQLAdapter.Validate() implementation that parses SQL, checks table/column existence against information_schema, and returns differentiated errors (TABLE_NOT_FOUND, COLUMN_NOT_FOUND, INVALID_SQL)
```

## GAP 3 (Test 4): Golden file comparison not deterministic

**Root cause:** `normalizeGoldenJSON` (test/mysql/mysql_golden_test.go:29-53) normalizes:
- `meta.trace_id` → `"00000000-0000-0000-0000-000000000000"` ✓
- `meta.duration_ms` → `0` ✓
- `meta.active_database_id` → `nil` ✓
- `query_cost` → `0` via recursive `normalizeEngineFields` ✓

It does NOT normalize:
- `data.tables[].schema` — the database schema name per `SchemaTableDef.Schema` (run_schema.go:148-149)
- `data.schema.tables[].schema` — same field in the nested `SchemaResult` copy

Each per-test database is named by `testhelper.GenerateDBName()` (test/testhelper/dbname.go:12-18) which generates `e2e_<32-hex-random>` using `crypto/rand`. The MySQL adapter populates `TableInfo.Schema` from `information_schema.COLUMNS.TABLE_SCHEMA` (adapter.go:188-190), which is the random UUID. This field appears in the JSON output as `"schema"` and changes on every test run.

**Debug session:** `.planning/debug/003-golden-normalization.md`

**YAML gap entry:**
```yaml
- truth: "Golden file comparison produces deterministic output after -update"
  status: failed
  reason: "normalizeGoldenJSON normalizes trace_id, duration_ms, active_database_id, and query_cost, but does NOT normalize the database schema name field. Each test run creates a random per-test database (e2e_<32-hex> via testhelper.GenerateDBName), and this name appears in data.tables[].schema and data.schema.tables[].schema. Without normalizing this field, golden files generated on one run contain a UUID that won't match subsequent runs."
  severity: major
  test: 4
  artifacts:
    - test/mysql/mysql_golden_test.go:29-53 (normalizeGoldenJSON)
    - test/mysql/mysql_golden_test.go:67-88 (normalizeEngineFields)
    - test/testhelper/dbname.go:12-18 (GenerateDBName)
  missing:
    - Schema field normalization in normalizeGoldenJSON: replace per-test database names (e2e_*) with stable placeholder like "e2e_test_db"
```

## GAP 4 (Test 5): Wrong exit codes for TABLE_NOT_FOUND and COLUMN_NOT_FOUND

**Root cause:** `MySQLAdapter.Validate()` (internal/db/mysql/adapter.go:339-341) is a one-line stub returning `db.ErrNotImplemented`:
```go
func (a *MySQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
    return nil, db.ErrNotImplemented
}
```

The CLI handler `runValidateWithAdapter` (internal/cli/run_validate.go:55-65) maps ALL errors from `adapter.Validate()` to `ErrCodeInvalidSQL`. When the test runs `querylex validate "SELECT * FROM nonexistent_table_xyz123"`, the adapter returns `ErrNotImplemented` → wrapped as `ErrCodeInvalidSQL` with message "SQL validation failed: not implemented in this adapter version".

The error codes `ErrCodeTableNotFound` and `ErrCodeColumnNotFound` already exist in `internal/format/error.go` (lines 29, 31) but are never produced by the MySQL adapter path because the validation logic doesn't exist.

Layer 1 (DML/DCL safety scan via `queryutil.ValidateSQLSafety`) works correctly — the exit code tests for `UNSAFE_SQL` (INSERT/UPDATE/DELETE/GRANT) pass fine.

**Debug session:** `.planning/debug/002-exitcode-validation.md`

**YAML gap entry:**
```yaml
- truth: "Correct exit codes for TABLE_NOT_FOUND and COLUMN_NOT_FOUND"
  status: failed
  reason: "MySQLAdapter.Validate() (internal/db/mysql/adapter.go:339-341) is a stub returning db.ErrNotImplemented. runValidateWithAdapter (internal/cli/run_validate.go:55-65) maps ALL adapter errors to INVALID_SQL. There is no schema-aware validation to detect table/column existence. Error codes ErrCodeTableNotFound and ErrCodeColumnNotFound exist in internal/format/error.go but are never produced."
  severity: major
  test: 5
  artifacts:
    - internal/db/mysql/adapter.go:339-341
    - internal/cli/run_validate.go:55-65
    - internal/format/error.go:29-31
  missing:
    - MySQLAdapter.Validate() implementation that queries information_schema to check table existence, column existence, and reports differentiated error codes
```

## GAP 5 (Test 7): Schema snapshot golden file not matching

**Root cause:** Same as GAP 3. `TestMySQLSnapshot` uses the identical `normalizeGoldenJSON` function. The snapshot output contains the full schema with `data.tables[].schema` fields populated with the random per-test UUID database name. Since `normalizeGoldenJSON` doesn't normalize these fields, the snapshot golden file generated with `-update` on one run won't match the next run.

Additionally, if any volatile MySQL metadata like `TABLE_COMMENT`, `COLUMN_COMMENT`, or other schema-version-dependent fields appear in the output, they would also cause mismatches, but the primary culprit is the un-normalized schema name field.

**Debug session:** `.planning/debug/003-golden-normalization.md` (same as GAP 3)

**YAML gap entry:**
```yaml
- truth: "Schema snapshot golden file matches after -update"
  status: failed
  reason: "Same root cause as Test 4 golden file issue. normalizeGoldenJSON doesn't normalize the database schema name field (data.tables[].schema and data.schema.tables[].schema). Each test run creates a random per-test database (e2e_<32-hex>), causing the snapshot output to differ across runs."
  severity: major
  test: 7
  artifacts:
    - test/mysql/mysql_golden_test.go:29-53
    - test/mysql/mysql_snapshot_test.go:45
  missing:
    - Schema field normalization in normalizeGoldenJSON (same fix as GAP 3)
    - Snapshot golden file regeneration after normalization fix
```

## GAP 6 (Test 8): CLI flag combinatorics failures

**Root cause:** Two independent sub-issues:

**Issue A — Unimplemented adapter methods:**
Three `MySQLAdapter` methods are stubs returning `db.ErrNotImplemented`:
1. `Explain` (line 335-336) → "Explain plan extraction failed: not implemented in this adapter version" with `ErrCodeExplainFailed`
2. `Validate` (line 339-341) → "SQL validation failed: not implemented in this adapter version" with `ErrCodeInvalidSQL`
3. `Joins` (line 557-559) → "Join extraction failed: not implemented in this adapter version" with `ErrCodeInternalError`

`Stats` (line 343-424) IS implemented via `information_schema.TABLES` queries and should work correctly.

**Issue B — Missing synthetic workspace artifacts:**
`setupE2EWorkspace` (test/mysql/setup_test.go:31-134) creates only:
- `schema/schema.json` with empty tables: `{"tables": []}`
- `schema/index_manifest.json`

Missing artifacts for disk-based commands:
- `schema/schema_map.json` — needed by `indexes` default path (no `--live` flag). `runIndexesFromDisk` (internal/cli/run_indexes.go:119-132) reads this file and returns `SCHEMA_PARSE_ERROR` when it doesn't exist.
- `schema/join_graph.json` — needed by `joins` fast path. `loadJoinGraphFromDisk` (internal/cli/run_joins.go:157-172) silently returns `false` when missing, falling back to `adapter.Joins()` which is unimplemented.
- `schema/schema_slim.json` — needed by `resolve` command. Currently created by separate `writeSchemaSlim` helper in the flags test, but not part of `setupE2EWorkspace`.

`resolve` works because `writeSchemaSlim` is called explicitly after `setupE2EWorkspace` in the flags test.

**Debug sessions:**
- `.planning/debug/004-unimplemented-adapters.md`
- `.planning/debug/006-synthetic-workspace-artifacts.md`

**YAML gap entry:**
```yaml
- truth: "All 7 deterministic CLI subcommands work with synthetic workspace"
  status: failed
  reason: "Two independent root causes: (a) MySQLAdapter.Explain(), Validate(), Joins() are stubs returning ErrNotImplemented — explain/validate/joins subcommands fail directly. (b) setupE2EWorkspace only creates empty schema.json, missing schema_map.json (needed by indexes disk path) and join_graph.json (needed by joins fast path). resolve works only because writeSchemaSlim is called separately. Stats adapter IS implemented and should work."
  severity: major
  test: 8
  artifacts:
    - internal/db/mysql/adapter.go:335-336 (Explain stub)
    - internal/db/mysql/adapter.go:339-341 (Validate stub)
    - internal/db/mysql/adapter.go:557-559 (Joins stub)
    - test/mysql/setup_test.go:94-126 (insufficient artifacts)
    - test/mysql/mysql_flags_test.go:273-357 (writeSchemaSlim band-aid)
    - internal/cli/run_indexes.go:119-132 (reads schema_map.json)
    - internal/cli/run_joins.go:157-172 (reads join_graph.json)
  missing:
    - Implement MySQLAdapter.Explain() with EXPLAIN FORMAT=JSON
    - Implement MySQLAdapter.Validate() with schema-aware validation
    - Implement MySQLAdapter.Joins() with FK relationship detection
    - Create schema_map.json in setupE2EWorkspace
    - Create join_graph.json in setupE2EWorkspace
    - Integrate writeSchemaSlim into setupE2EWorkspace
    - Populate schema.json with non-empty table definitions matching loaded schema
```

## GAP 7 (Test 9): Test output clarity at a glance

**Root cause:** Not a standalone bug. This is a **secondary consequence** of the 6 other root causes above. With 5 of 7 test functions failing, the `-test.v` output is flooded with failure messages from:
- All 12 sub-tests in `TestMySQLValidation` (except DML/DCL) fail due to unimplemented Validate
- All golden file sub-tests fail due to schema name normalization gap
- Flag combinatorics tests for explain/validate/joins fail due to unimplemented adapters
- Indexes tests fail due to missing schema_map.json

The output format itself (`-test.v` with PASS/FAIL per sub-test) is correct and appropriate. The signal improves once the other root causes are resolved.

Additionally, some test errors show raw JSON output in error messages rather than cleaned-up assertions, which adds noise. This is a structural issue with how `t.Errorf` formats diffs in the flag tests and golden tests.

**Debug session:** `.planning/debug/007-output-clarity.md`

**YAML gap entry:**
```yaml
- truth: "Clear test output at a glance"
  status: failed
  reason: "Consequence of 5/7 test functions failing due to other root causes. The -test.v format is correct (PASS/FAIL per sub-test), but failures from 6 root causes flood output. This self-resolves when underlying issues are fixed. Minor severity: output format is correct, just noisy."
  severity: minor
  test: 9
  artifacts: []
  missing:
    - Fix the 6 other root causes — output clarity is a secondary effect
    - Optionally: add overall pass/fail summary line to make test-e2e-mysql target
```

---

## Cross-Reference: Root Causes → Gaps

| Root Cause | Files | Affected Gaps |
|---|---|---|
| MySQLAdapter.Validate() returns ErrNotImplemented | internal/db/mysql/adapter.go:339-341, internal/cli/run_validate.go:55-65 | Gap 2 (Test 3), Gap 4 (Test 5), Gap 6 (Test 8) |
| MySQLAdapter.Explain() returns ErrNotImplemented | internal/db/mysql/adapter.go:335-336 | Gap 6 (Test 8) |
| MySQLAdapter.Joins() returns ErrNotImplemented | internal/db/mysql/adapter.go:557-559 | Gap 6 (Test 8) |
| normalizeGoldenJSON missing schema field normalization | test/mysql/mysql_golden_test.go:29-53 | Gap 3 (Test 4), Gap 5 (Test 7) |
| setupE2EWorkspace creates only empty schema.json | test/mysql/setup_test.go:94-126 | Gap 6 (Test 8) |
| setupE2EWorkspace missing schema_map.json, join_graph.json | test/mysql/setup_test.go | Gap 6 (Test 8) |
| Output clarity is consequence, not root cause | — | Gap 7 (Test 9) |

## Priority Order for Fix

1. **Implement MySQLAdapter.Validate()** — fixes 3 gaps (Tests 3, 5, partial 8). Most impactful single change.
2. **Implement MySQLAdapter.Explain()** — fixes Test 8 explain subcommand
3. **Implement MySQLAdapter.Joins()** — fixes Test 8 joins subcommand
4. **Add schema name normalization** — fixes Tests 4 and 7 golden files
5. **Populate synthetic workspace artifacts** — fixes Test 8 indexes/resolve reliability
