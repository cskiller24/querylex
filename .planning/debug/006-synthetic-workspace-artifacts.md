---
status: resolved
trigger: "Synthetic workspace artifacts in setupE2EWorkspace are insufficient for schema-dependent subcommands (indexes, joins, resolve via disk)"
created: 2026-06-04T04:20:00Z
updated: 2026-06-04T04:25:00Z
---

## Current Focus

hypothesis: setupE2EWorkspace creates only schema.json (with empty tables []), missing schema_map.json, join_graph.json, and populated schema.json needed by disk-based subcommands
test: Map each disk-based subcommand to the artifact it requires
expecting: indexes (no --live) needs schema_map.json, joins tries join_graph.json, resolve needs schema_slim.json
next_action: Return root cause findings as YAML

## Symptoms

expected: |
  All 7 deterministic subcommands work with the synthetic workspace created by setupE2EWorkspace
actual: |
  - indexes (without --live): "Failed to read schema_map.json: ..." (SCHEMA_PARSE_ERROR)
  - joins: Falls back to adapter.Joins() which returns ErrNotImplemented
  - resolve: Works because writeSchemaSlim creates schema_slim.json
  - schema: Works because it uses the adapter directly (not disk artifacts)
  - stats: Works because it uses adapter.Stats() which is implemented
errors: |
  joins: "Join extraction failed: not implemented in this adapter version"
  indexes (disk): "Failed to read schema_map.json: ..."
reproduction: |
  setupE2EWorkspace creates empty schema.json, then run `querylex indexes --table employees` or `querylex joins --table employees`
started: always broken (synthetic artifacts were minimal from the start)

## Eliminated

- hypothesis: "Stats fails because of missing artifacts"
  evidence: RunStatsTables calls PreflightForCommand then adapter.Stats() directly — no disk artifacts needed. Stats is implemented and should work.
  timestamp: 2026-04-04T04:22:00Z

- hypothesis: "Schema fails because of missing artifacts"
  evidence: RunSchema calls PreflightForCommand then adapter.Schema() directly — no disk artifacts needed. Schema is implemented and works.
  timestamp: 2026-04-04T04:22:10Z

## Evidence

- timestamp: 2026-04-04T04:21:00Z
  checked: test/mysql/setup_test.go setupE2EWorkspace lines 94-126
  found: Creates only schema.json ({"tables":[]}) and index_manifest.json. Missing: schema_map.json, join_graph.json, schema_slim.json, domain_map.json, terminologies.md.
  implication: Only 2 of 7 expected artifacts are created. The empty schema.json means even that artifact has no table data.

- timestamp: 2026-04-04T04:21:30Z
  checked: internal/cli/run_indexes.go runIndexesFromDisk (no --live path) lines 119-132
  found: |
    schemaMapPath := filepath.Join(dbDir, "schema", "schema_map.json")
    schemaMapData, err := os.ReadFile(schemaMapPath)
    if err != nil {
        resp := format.NewErrorResponse[IndexData](
            format.ErrCodeSchemaParseError,
            fmt.Sprintf("Failed to read schema_map.json: %v", err),
            ...
        )
    }
  implication: Default indexes path (without --live) reads schema_map.json which doesn't exist → SCHEMA_PARSE_ERROR

- timestamp: 2026-04-04T04:22:00Z
  checked: internal/cli/run_joins.go loadJoinGraphFromDisk lines 157-172
  found: |
    func loadJoinGraphFromDisk(activeDBID string) (*index.JoinGraphResult, bool) {
        joinGraphPath := filepath.Join(home, ".querylex", activeDBID, "schema", "join_graph.json")
        data, err := os.ReadFile(joinGraphPath)
        if err != nil { return nil, false }
    }
  implication: loadJoinGraphFromDisk can't find join_graph.json → returns false → falls back to adapter.Joins() which is not implemented → "not implemented in this adapter version"

- timestamp: 2026-04-04T04:22:30Z
  checked: internal/cli/run_stats.go knownArtifactPaths (lines 44-52)
  found: Expected artifacts: schema/schema.json, schema/schema_slim.json, schema/join_graph.json, schema/schema_map.json, domain_map.json, schema/domain_map.json, terminologies.md
  implication: The workspace health check will report 5 of 7 artifacts as "missing" because only schema.json and index_manifest.json are created

- timestamp: 2026-04-04T04:23:00Z
  checked: test/mysql/mysql_flags_test.go writeSchemaSlim (lines 273-357)
  found: The test helper writes schema_slim.json separately — this is a band-aid fixing the resolve command but not called for all tests
  implication: schema_slim.json should be part of setupE2EWorkspace, not a per-test helper

## Resolution

root_cause: |
  setupE2EWorkspace creates a synthetic workspace with only 2 artifacts:
  1. schema/schema.json — but with empty tables: {"tables":[]}
  2. schema/index_manifest.json — checksum for the empty schema

  Missing artifacts that disk-based commands depend on:
  - schema/schema_map.json — needed by indexes (no --live) command
  - schema/join_graph.json — needed by joins fast path
  - schema/schema_slim.json — needed by resolve command (currently created by separate writeSchemaSlim)
  - domain_map.json, schema/domain_map.json, terminologies.md — needed by other indexing-dependent features

  Additionally, schema.json has an empty tables array, so even commands reading from it have no table data.
fix: |
  Enhance setupE2EWorkspace to create realistic artifacts:
  1. Populate schema.json with actual table definitions matching Employees DB (employees, departments, dept_emp, dept_manager, titles, salaries)
  2. Generate schema_map.json from the table definitions for indexes command
  3. Generate join_graph.json with FK relationships between tables for joins fast path
  4. Move writeSchemaSlim logic into setupE2EWorkspace (or call it from within)
  
  Alternative approach: Generate artifacts by running the actual index pipeline against the live database during setup, then saving the artifacts to the workspace directory.
verification: empty
files_changed:
  - test/mysql/setup_test.go
  - test/mysql/mysql_flags_test.go (may remove writeSchemaSlim if merged into setupE2EWorkspace)
