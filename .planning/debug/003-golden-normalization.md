---
status: resolved
trigger: "Golden file normalization not producing stable output across runs — passes with -update but fails on re-run"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: normalizeGoldenJSON does not normalize the database schema name field which contains a random per-test UUID (e2e_<32hex>) that changes every run
test: Trace the schema output JSON structure and identify un-normalized volatile fields
expecting: The "schema" field in table definitions contains the per-test database name which varies per run
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  After running with -update, golden file matches on subsequent re-run without -update
actual: |
  -update generates golden file from current run's output. Re-run without -update fails because the normalized output differs from the golden file.
errors: |
  output mismatch (-want +got)
reproduction: |
  1. go test -tags e2e -run TestMySQLGolden -update
  2. go test -tags e2e -run TestMySQLGolden (no -update)
  3. Second run fails with mismatch
started: always broken (normalization was incomplete from the start)

## Eliminated

- hypothesis: "trace_id normalization is insufficient"
  evidence: normalizeGoldenJSON line 40 correctly sets trace_id to "00000000-0000-0000-0000-000000000000"
  timestamp: 2026-04-04T04:22:00Z

- hypothesis: "duration_ms normalization is insufficient"
  evidence: normalizeGoldenJSON line 41 correctly sets duration_ms to 0
  timestamp: 2026-04-04T04:22:10Z

- hypothesis: "MySQL volatile fields (query_cost) not normalized"
  evidence: normalizeEngineFields recursively normalizes query_cost to 0
  timestamp: 2026-04-04T04:22:20Z

## Evidence

- timestamp: 2026-04-04T04:21:00Z
  checked: test/testhelper/dbname.go GenerateDBName
  found: |
    func GenerateDBName() string {
        buf := make([]byte, 16)
        rand.Read(buf)
        return fmt.Sprintf("e2e_%x", buf)
    }
  implication: Each test run creates a database with a unique random name like "e2e_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"

- timestamp: 2026-04-04T04:21:30Z
  checked: internal/cli/run_schema.go SchemaTableDef struct
  found: |
    type SchemaTableDef struct {
        Schema      string               `json:"schema"`
        ...
    }
  And line 149: Schema: t.Schema
  implication: The table's database/schema name is included in the JSON output as "schema" field

- timestamp: 2026-04-04T04:22:00Z
  checked: internal/db/mysql/adapter.go line 188-190
  found: |
    ta = &tableAccum{
        info: db.TableInfo{
            Schema: r.TableSchema,
            ...
        },
    }
  implication: MySQL adapter populates Schema from information_schema.COLUMNS.TABLE_SCHEMA which equals the database name — the random per-test UUID

- timestamp: 2026-04-04T04:22:30Z
  checked: test/mysql/mysql_golden_test.go normalizeGoldenJSON lines 38-43
  found: normalizeGoldenJSON normalizes trace_id, duration_ms, active_database_id only
  implication: The "schema" field in table definitions is NOT normalized

- timestamp: 2026-04-04T04:23:00Z
  checked: test/mysql/mysql_golden_test.go normalizeEngineFields lines 67-88
  found: normalizeEngineFields only handles query_cost recursively
  implication: No normalization of database/schema names in the schema output tree

- timestamp: 2026-04-04T04:23:30Z
  checked: internal/format/response.go ResponseMeta
  found: protocol_version is hardcoded "1.0.0" — stable. active_database_id is "e2e-test-db" from setupE2EWorkspace — stable.
  implication: trace_id, duration_ms, active_database_id are already handled. Schema field is the missing piece.

## Resolution

root_cause: |
  normalizeGoldenJSON normalizes trace_id, duration_ms, active_database_id, and query_cost, but does NOT normalize the database schema name in the table definitions. Each test run creates a unique per-test database (e2e_<random-hex>), and this name appears in:
  - data.tables[].schema (SchemaTableDef.Schema)
  - data.schema.tables[].schema (nested SchemaResult.Tables[].Schema)
  
  Without normalizing this field, golden files generated on one run contain the random database name, which won't match subsequent runs with different random names.
fix: Add schema name normalization to normalizeGoldenJSON that replaces the per-test UUID database name with a stable value like "e2e_test_db". The normalization should handle both the top-level SchemaTableDef.schema field and the nested SchemaResult.schema field in the SchemaResult copy.
verification: empty
files_changed:
  - test/mysql/mysql_golden_test.go
  - test/testdata/golden/mysql/TestSchemaOutput.json (regenerate after fix)
  - test/testdata/golden/mysql/TestSnapshotOutput.json (regenerate after fix)
