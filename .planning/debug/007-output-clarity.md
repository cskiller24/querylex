---
status: resolved
trigger: "Poor signal-to-noise ratio — 5 of 7 test functions fail, making it hard to see test status at a glance"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: The output clarity issue is a CONSEQUENCE of the other 6 root causes, not a standalone bug. With 5 of 7 test functions failing, the PASS/FAIL output is diluted by noise from known failures. Fixing the underlying issues will automatically improve signal-to-noise.
test: Assess whether the output format itself is correct (PASS/FAIL per sub-test) and whether the signal improves once other issues are fixed
expecting: Output format is correct; fixing other root causes will make signal better
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  Running `make test-e2e-mysql` produces clear PASS/FAIL output with overall summary
actual: |
  The -test.v output shows PASS/FAIL per sub-test correctly, but with 5 of 7 test functions failing, the signal-to-noise ratio is poor.
errors: |
  (none — this is a readability concern, not an error)
reproduction: |
  Run `make test-e2e-mysql` — output is correct format but flooding with failures
started: always (since 5/7 tests fail from the start)

## Eliminated

- hypothesis: "The output format itself is wrong"
  evidence: The `-test.v` flag shows clear PASS/FAIL per sub-test. The format is correct.
  timestamp: 2026-04-04T04:22:00Z

## Evidence

- timestamp: 2026-04-04T04:21:00Z
  checked: Makefile test-e2e-mysql target
  found: Runs `go test -tags e2e -v ./test/mysql/` with -test.v flag
  implication: Output includes PASS/FAIL for each sub-test — format is correct

- timestamp: 2026-04-04T04:22:00Z
  checked: UAT test results
  found: 2 of 7 test functions pass (TestMySQLSchema, TestMySQLIsolation); 5 fail
  implication: The poor signal-to-noise is entirely due to the number of failures, not test output format

- timestamp: 2026-04-04T04:23:00Z
  checked: UAT severity classification
  found: This gap is classified as "minor" severity
  implication: This is the least critical issue; it's a consequence of other gaps

## Resolution

root_cause: |
  Not a standalone bug. The output clarity issue is purely a consequence of 5 of 7 test functions failing due to the other 6 root causes. Once those are fixed:
  1. TestMySQLExitCodes (exit codes) → MySQLAdapter.Validate() implemented
  2. TestMySQLValidation (validation matrix) → same adapter fix
  3. TestMySQLGolden (golden schema) → schema field normalization 
  4. TestMySQLSnapshot (golden snapshot) → schema field normalization
  5. TestMySQLFlags (flag combinatorics) → unimplemented adapters + workspace artifacts
  
  The output format itself (-test.v) is correct and clear.
fix: |
  No direct fix needed. This issue self-resolves when the 6 other root causes are addressed. Optionally:
  - Add an overall summary line at the end of `make test-e2e-mysql`: "X/Y tests passed"
  - Consider CI-level status reporting rather than changing test output format
verification: empty
files_changed: []
