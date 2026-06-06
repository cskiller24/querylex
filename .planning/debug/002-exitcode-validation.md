---
status: resolved
trigger: "table_not_found/column_not_found exit code tests return INVALID_SQL instead of TABLE_NOT_FOUND/COLUMN_NOT_FOUND"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: MySQLAdapter.Validate() returns ErrNotImplemented, causing runValidateWithAdapter to wrap ALL adapter errors as ErrCodeInvalidSQL — preventing TABLE_NOT_FOUND/COLUMN_NOT_FOUND differentiation
test: Trace error code flow from TestMySQLExitCodes table_not_found/column_not_found through call stack
expecting: Both err paths map to ErrCodeInvalidSQL because MySQLAdapter.Validate() is a stub
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  `querylex validate "SELECT * FROM nonexistent_table_xyz123"` returns error.code "TABLE_NOT_FOUND"
  `querylex validate "SELECT nonexistent_col_xyz123 FROM employees"` returns error.code "COLUMN_NOT_FOUND"
actual: |
  Both return error.code "INVALID_SQL" with message "SQL validation failed: not implemented in this adapter version"
errors: |
  SQL validation failed: not implemented in this adapter version
reproduction: |
  Run `querylex validate "SELECT * FROM nonexistent_table_xyz123"` against MySQL with employees schema
started: always broken (MySQL adapter Validate was never implemented)

## Eliminated

- hypothesis: "The validate method is partially implemented but missing table/column detection"
  evidence: Read internal/db/mysql/adapter.go line 339-341 — Validate() is a one-line stub returning ErrNotImplemented
  timestamp: 2026-06-04T04:22:00Z

## Evidence

- timestamp: 2026-06-04T04:21:00Z
  checked: internal/db/mysql/adapter.go Validate method
  found: |
    func (a *MySQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
        return nil, db.ErrNotImplemented
    }
  implication: MySQL adapter has ZERO schema-aware validation implementation

- timestamp: 2026-06-04T04:21:30Z
  checked: internal/cli/run_validate.go runValidateWithAdapter
  found: |
    result, err := adapter.Validate(ctx, query)
    if err != nil {
        resp := format.NewErrorResponse[ValidateData](
            format.ErrCodeInvalidSQL,
            fmt.Sprintf("SQL validation failed: %v", err),
            ...
        )
    }
  implication: ALL errors from adapter.Validate() are mapped to ErrCodeInvalidSQL — no distinction between syntax errors, table-not-found, or column-not-found

- timestamp: 2026-06-04T04:22:00Z
  checked: internal/format/error.go
  found: ErrCodeTableNotFound and ErrCodeColumnNotFound are defined as constants — they exist but are never produced by the MySQL adapter path
  implication: Error codes exist in format package; only the adapter implementation and CLI routing are missing

- timestamp: 2026-06-04T04:23:00Z
  checked: internal/cli/run_validate.go Layer 1 (DML/DCL safety check)
  found: queryutil.ValidateSQLSafety runs first and correctly returns UNSAFE_SQL for INSERT/UPDATE/DELETE
  implication: Layer 1 works fine. The gap is Layer 2 (schema-aware adapter validation) only.

## Resolution

root_cause: |
  MySQLAdapter.Validate() is a stub returning db.ErrNotImplemented. The CLI handler runValidateWithAdapter maps ALL adapter errors to ErrCodeInvalidSQL without distinction. No schema-aware validation exists to detect table existence, column existence, or SQL syntax errors against the live MySQL schema.
fix: Implement MySQLAdapter.Validate() with schema-aware query validation that returns differentiated errors:
  - TABLE_NOT_FOUND when referenced tables don't exist
  - COLUMN_NOT_FOUND when referenced columns don't exist
  - INVALID_SQL for actual syntax/parse errors
verification: empty
files_changed:
  - internal/db/mysql/adapter.go
