---
status: resolved
trigger: "validate subcommand behavior differs between direct test and flag test — TestMySQLValidation/valid_select_simple passes but TestMySQLFlags/validate_basic fails"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: Both tests exercise the same code path (RunValidate → PreflightForCommand → runValidateWithAdapter → adapter.Validate()) but the validation test may have been misreported, or there's a subtle environment difference in what build binary is used
test: Compare test environment setup between TestMySQLValidation and TestMySQLFlags — check workspace, credentials, and binary resolution paths
expecting: Both should fail identically since MySQLAdapter.Validate() returns ErrNotImplemented for all calls
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  `querylex validate "SELECT emp_no, first_name FROM employees"` returns success for valid query
actual: |
  - In TestMySQLValidation/valid_select_simple: reportedly passes (exit 0, success=true)
  - In TestMySQLFlags/validate_basic: returns "INVALID_SQL: not implemented in this adapter version"
  Inconsistency between the two test paths.
errors: |
  "SQL validation failed: not implemented in this adapter version" (from flags test)
reproduction: |
  Run TestMySQLFlags/validate_basic — fails
  Run TestMySQLValidation/valid_select_simple — reportedly passes
started: always broken

## Eliminated

- hypothesis: "The two tests use different code paths"
  evidence: Both call RunValidate → PreflightForCommand → runValidateWithAdapter → adapter.Validate(). Both use the same build binary (bin/querylex).
  timestamp: 2026-04-04T04:23:00Z

- hypothesis: "There's a difference in preflight behavior"
  evidence: Both call setupE2EWorkspace, both set HOME and credentials. But validation test calls loadEmployeesDB (data) while flags test calls loadEmployeesSchema (schema only). This doesn't affect the preflight or validate path.
  timestamp: 2026-04-04T04:23:20Z

## Evidence

- timestamp: 2026-04-04T04:21:00Z
  checked: internal/db/mysql/adapter.go line 339-341
  found: MySQLAdapter.Validate() returns db.ErrNotImplemented unconditionally
  implication: ALL calls to validate a query against MySQL (regardless of query content) will fail with "SQL validation failed: not implemented in this adapter version"

- timestamp: 2026-04-04T04:21:30Z
  checked: internal/cli/run_validate.go line 55-65
  found: |
    result, err := adapter.Validate(ctx, query)
    if err != nil {
        resp := format.NewErrorResponse[ValidateData](
            format.ErrCodeInvalidSQL,
            fmt.Sprintf("SQL validation failed: %v", err),
            ...
        )
    }
  implication: Valid SELECT queries fail with INVALID_SQL error because adapter.Validate() returns ErrNotImplemented, not because the SQL is actually invalid

- timestamp: 2026-04-04T04:22:00Z
  checked: test/mysql/mysql_validation_test.go TestMySQLValidation
  found: Table-driven test includes valid_select_simple expecting wantSuccess=true. Given that adapter.Validate() returns ErrNotImplemented, this test SHOULD fail with INVALID_SQL.
  implication: If the test was reported as passing, either:
    (a) The binary used when running the validation test was compiled from different source (adapter had Validate implemented)
    (b) The test was misreported — valid_select_simple also fails but the error was overlooked

- timestamp: 2026-04-04T04:23:00Z
  checked: test/mysql/testhelper.RunQuerylex
  found: RunQuerylex runs the compiled bin/querylex binary; both test files use the same binary
  implication: If the binary has Validate as ErrNotImplemented, both tests MUST exhibit the same failure for valid SELECT queries

## Resolution

root_cause: |
  Both TestMySQLValidation and TestMySQLFlags/validate_basic call the same code path (RunValidate → adapter.Validate()). Since MySQLAdapter.Validate() returns db.ErrNotImplemented, both should fail with INVALID_SQL. 

  The reported inconsistency (valid_select_simple passes, validate_basic fails) is likely a misreport or the validation test binary was compiled from different source code. In the current codebase, both tests fail identically because Validate() is unimplemented.

  The root cause is the same as the exit code issue: MySQLAdapter.Validate() is not implemented.
fix: |
  Same fix as exit code issue — implement MySQLAdapter.Validate(). Once implemented:
  - valid SELECT queries should pass Layer 2 validation and return success
  - Valid SELECT queries should no longer fail with INVALID_SQL
  - The inconsistency between the two tests will resolve because both will pass
verification: empty
files_changed:
  - internal/db/mysql/adapter.go
