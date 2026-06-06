---
status: resolved
trigger: "stats/explain/validate/joins subcommands fail with 'not implemented in this adapter version' in flag combinatorics test"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: Three MySQL adapter methods (Explain, Validate, Joins) are stubs returning ErrNotImplemented, causing subcommands to fail. Stats IS implemented and should work.
test: Check each method in MySQLAdapter
expecting: |
  Explain → ErrNotImplemented
  Validate → ErrNotImplemented
  Joins → ErrNotImplemented
  Stats → implemented (will work)
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  All 7 deterministic subcommands (schema, stats, indexes, explain, validate, joins, resolve) work with synthetic workspace
actual: |
  explain, validate, joins fail with "not implemented in this adapter version" error messages
  stats, schema, indexes, resolve behaviors depend on preflight and workspace artifacts
errors: |
  Explain: "Explain plan extraction failed: not implemented in this adapter version"
  Validate: "SQL validation failed: not implemented in this adapter version"
  Joins: "Join extraction failed: not implemented in this adapter version"
reproduction: |
  Run `querylex explain "SELECT 1"` or `querylex validate "SELECT 1"` or `querylex joins --table employees` against MySQL
started: always broken (these methods were never implemented for MySQL)

## Eliminated

- hypothesis: "Preflight is failing before reaching the adapter method"
  evidence: PreflightForCommand succeeds for Schema (which calls PreflightForCommand first)
  timestamp: 2026-04-04T04:22:00Z

## Evidence

- timestamp: 2026-04-04T04:21:00Z
  checked: internal/db/mysql/adapter.go line 335-336
  found: |
    func (a *MySQLAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
        return nil, db.ErrNotImplemented
    }
  implication: Explain is a stub. No MySQL EXPLAIN FORMAT=JSON implementation.

- timestamp: 2026-04-04T04:21:10Z
  checked: internal/db/mysql/adapter.go line 339-341
  found: |
    func (a *MySQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
        return nil, db.ErrNotImplemented
    }
  implication: Validate is a stub. No MySQL schema-aware validation.

- timestamp: 2026-04-04T04:21:20Z
  checked: internal/db/mysql/adapter.go line 557-559
  found: |
    func (a *MySQLAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
        return nil, db.ErrNotImplemented
    }
  implication: Joins is a stub. No MySQL FK/inferred join detection.

- timestamp: 2026-04-04T04:22:00Z
  checked: internal/db/mysql/adapter.go line 343-424
  found: Stats IS implemented — queries information_schema.TABLES for row counts, data length, index length
  implication: stats subcommand should work via adapter (but RunStatsTables also depends on PreflightForCommand succeeding)

- timestamp: 2026-04-04T04:22:30Z
  checked: internal/cli/run_explain.go line 73-83 and run_joins.go line 50-60 and run_validate.go line 55-65
  found: All three handlers catch adapter errors and return their respective error codes (ErrCodeExplainFailed, ErrCodeInternalError, ErrCodeInvalidSQL) with the error message including "not implemented in this adapter version"
  implication: The "not implemented" message propagates from db.ErrNotImplemented through each CLI handler's error wrapping

## Resolution

root_cause: |
  Three MySQLAdapter methods are stubs returning db.ErrNotImplemented:
  1. Explain (line 335-336) — no EXPLAIN FORMAT=JSON implementation
  2. Validate (line 339-341) — no schema-aware validation
  3. Joins (line 557-559) — no foreign key / join path detection
  
  Stats (line 343-424) IS implemented via information_schema.TABLES queries. The UAT report groups stats with the "not implemented" failures but stats has a working adapter method.
fix: |
  Implement three MySQL adapter methods:
  - Explain: Run EXPLAIN FORMAT=JSON, parse output into db.ExplainPlan
  - Validate: Parse query, check table/column existence against information_schema, differentiate TABLE_NOT_FOUND/COLUMN_NOT_FOUND/INVALID_SQL
  - Joins: Query information_schema.KEY_COLUMN_USAGE for FK relationships, build JoinEdge result
verification: empty
files_changed:
  - internal/db/mysql/adapter.go
