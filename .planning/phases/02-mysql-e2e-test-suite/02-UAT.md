---
status: complete
phase: 02-mysql-e2e-test-suite
source:
  - 02-01-SUMMARY.md
  - 02-02-SUMMARY.md
started: 2026-06-04T04:55:00Z
updated: 2026-06-06T12:43:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Build compiles cleanly
expected: |
  Run `make build && make build-test`. Both bin/querylex and bin/e2e.test
  produced without errors.
result: pass

### 2. Validate differentiates error codes
expected: |
  Run against live MySQL:
  - `querylex validate "SELECT * FROM employees LIMIT 1"` → success
  - `querylex validate "INSERT INTO employees VALUES (1)"` → UNSAFE_SQL
  - `querylex validate "SELECT * FROM nonexistent_table"` → TABLE_NOT_FOUND
  - `querylex validate "SELECT nonexistent_col FROM employees"` → COLUMN_NOT_FOUND
result: pass

### 3. Explain returns structured JSON
expected: |
  Run `querylex explain "SELECT * FROM employees WHERE emp_no = 10001"` against live MySQL.
  Returns JSON with full_scan_tables, index_usage, estimated cost.
result: pass

### 4. Joins returns FK relationships
expected: |
  Run `./bin/querylex joins --table employees` against live MySQL.
  Returns FK edges referencing employees (dept_emp, dept_manager, titles, salaries).
result: pass

### 5. Golden file comparison is deterministic
expected: |
  Run `./bin/e2e.test -test.run "TestMySQLGolden|TestMySQLSnapshot" -test.v -update`
  to regenerate golden files. Then re-run without `-update`:
  `./bin/e2e.test -test.run "TestMySQLGolden|TestMySQLSnapshot" -test.v`
  Both exit 0, no golden file mismatch.
result: pass

### 6. Full E2E test suite passes
expected: |
  Run `make test-e2e-mysql`. All 7 test functions (~60 sub-tests) pass
  with exit code 0.
result: pass

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
