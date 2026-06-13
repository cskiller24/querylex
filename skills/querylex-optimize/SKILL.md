---
name: querylex-optimize
description: Optimize SQL queries using explain plans, schema context, statistics, indexes, joins, and dialect-aware rewrite heuristics via the QueryLex CLI toolchain. Use this skill whenever the user asks to optimize, tune, speed up, or improve a SQL query's performance — even if they don't explicitly say "optimize". This includes phrases like "my query is slow", "this query takes forever", "how can I make this faster", "find the bottleneck", "analyze this explain plan", "can you rewrite this to run better", "this query is doing a full table scan", or "help me fix this slow query". Also use when the user pastes a SQL query and asks for performance analysis, or when they share an explain plan output and ask what's wrong. Do NOT use this skill for generating SQL from natural language (that's /querylex-sql) or for general schema exploration.
---

# querylex-optimize: SQL Optimization

Optimize a supplied SQL query by orchestrating the QueryLex CLI toolchain. The process checks the active database, validates the input SQL, fetches its explain plan, gathers schema/join/stats/index context for referenced tables, applies dialect-aware rewrite strategies, compares plans, and recommends indexes when rewrites are insufficient.

## Prerequisites

The `querylex` binary must be on `$PATH`. Verify with `which querylex`. The working database must be indexed (one-time `querylex add-db` setup). If the binary is not found, build it from the project root: `go build -o /usr/local/bin/querylex ./cmd/querylex`.

## Flags

| Flag | Behavior |
|------|----------|
| `--analyze` | Pass `--analyze` through to `querylex explain`. Warn the user the database may execute the query. |
| `--no-index` | The skill may rewrite SQL but must not recommend new indexes. |

## The Core Workflow

Execute these steps in order, one at a time. Every `querylex` subcommand returns a JSON envelope with `success`, `data`, `error`, `warnings`, and `meta`. Always check `success` first; if false, read `error.code` and `error.message`.

### Step 1: Receive the SQL

Capture the user's SQL query exactly as provided, along with any flags (`--analyze`, `--no-index`). This is the `original_sql` used in subsequent steps.

### Step 2: Active Database Preflight

Run `querylex workspace-stats` to get workspace status, active database ID, type, and indexing state.

Parse the JSON response:

| Condition | Action |
|-----------|--------|
| Command fails (no workspace) | Stop. Tell the user no Querylex workspace exists. Ask them to run `querylex add-db`. |
| Response error (corrupt workspace) | Stop. Report `WORKSPACE_STATE_INVALID`. Suggest restoring from backup or re-running setup. |
| No `active_database_id` | Stop. Show connected databases and ask user to select or add one. |
| Active DB not found among connected | Stop. Report stale workspace state. |
| Status `not_indexed` | Stop. Ask user to complete indexing first. |
| Status `indexing` | Show last-known progress. Proceed only if a previous manifest exists and user accepts stale context. |
| Status `index_failed` | Stop unless a previous manifest exists. If proceeding with stale artifacts, include a warning in every result. |
| Status `stale` | Proceed with a warning that schema/stats/joins/indexes may be outdated. |
| Status `indexed` | Proceed normally. |

Extract and store the database type (`type` field — mysql, postgresql, sqlite, mssql, mariadb) for dialect-aware rewriting.

### Step 3: Check Memory

Run `querylex memory "<original_sql>"` to check for a cached optimization.

- On error: log a warning and continue to step 4.
- On match (`match_found: true`, similarity >= 0.86): show the cached optimization with its explanation. Ask the user whether to use it or run fresh analysis. If they accept the cached result, stop. If they want fresh analysis, continue.
- On no match: continue to step 4.

### Step 4: Validate Original SQL

Run `querylex validate "<original_sql>"`.

- On failure: report the validation error and stop. Show the error code and message — fix the SQL syntax/table/column references before retrying optimization.
- On success: capture the `normalized_sql` from the response. This is the canonical form used for all remaining steps.

### Step 5: Get Explain Plan

Run `querylex explain "<normalized_sql>"` (with `--analyze` if the user passed that flag).

- On failure: report the error and stop.
- On success: save the execution plan and heuristics from the response. The `execution_plan` field includes normalized metrics (`estimated_total_cost`, `estimated_rows_examined`, `full_scan_tables`, `index_usage`, `sort_operations`, `temp_operations`, `join_operations`). The `heuristics` array contains detected issues like `FULL_TABLE_SCAN`, `NON_SARGABLE_PREDICATE`, `MISSING_INDEX`, `HIGH_COST_ESTIMATE`.

If `--analyze` was used, the plan also includes `actual_total_time_ms` and `actual_rows_examined` — these are the gold-standard comparison metrics.

### Step 6: Extract Referenced Tables

Parse the explain plan output and the normalized SQL to identify all tables referenced, including those inside CTEs, subqueries, derived tables, and views when resolvable. Build a list like `["lineitem", "orders", "customer"]`.

### Step 7: Fetch Context for Referenced Tables

Run these four commands sequentially. Each failure is recorded as missing context but does not automatically block optimization unless context coverage falls critically low.

```bash
querylex schema --table table1 --table table2 ...
querylex joins --table table1 --table table2 ...
querylex stats --table table1 --table table2 ...
querylex indexes --table table1 --table table2 ...
```

- **Schema**: On error, warn and continue. Missing schema makes column-level rewrites risky.
- **Joins**: On error, stop only if the SQL requires joining tables that cannot be connected. Otherwise warn and continue.
- **Stats**: On error, warn and continue. Missing stats reduce confidence in join-order and filter-placement decisions.
- **Indexes**: On error, warn and continue. Missing index metadata blocks index recommendations.

### Step 8: Compute Context Coverage

Score your available context:

| Signal | Weight | Notes |
|--------|--------|-------|
| Explain plan available | 35 | Always present if step 5 succeeded |
| Schema for referenced tables | 25 | Count as available only if ALL referenced tables had schema |
| Join graph available | 15 | Count as available if joins command succeeded for the table set |
| Statistics available | 15 | Count as available if stats succeeded for at least half of referenced tables |
| Index metadata available | 10 | Count as available if indexes succeeded for at least half of referenced tables |

Coverage = sum of weights for available signals (max 100).

- **80-100**: full context. Proceed with all rewrite strategies and index recommendations.
- **50-79**: partial context. May rewrite SQL but avoid high-risk structural rewrites (join reordering, subquery-to-join transformations) unless plan evidence strongly supports them. Include context warnings.
- **35-49**: plan-only context. Only suggest low-risk rewrites (non-sargable fixes, narrowing projections). Do NOT recommend indexes or restructure joins.
- **<35**: insufficient context. Return unable-to-optimize listing which context sources failed. Stop.

### Step 9: Analyze the Plan

Examine the explain plan and heuristics for these issue classes. The explain command already flags many of these, but you should also look for patterns the heuristics may miss:

| Code | Description | What to do |
|------|-------------|------------|
| `FULL_TABLE_SCAN` | Large table scanned without selective predicate or usable index | Add selective predicate, make predicate sargable, or recommend an index |
| `NON_SARGABLE_PREDICATE` | Indexed column wrapped in a function (`DATE(col)`, `YEAR(col)`, `LOWER(col)`) | Move transform to the constant side: `col >= 'start' AND col < 'end'` |
| `UNBOUNDED_SELECT_STAR` | `SELECT *` against large or joined tables | Project only required columns |
| `JOIN_ORDER_RISK` | Large table joined before selective filters or smaller driving tables | Reorder joins or push filters into subqueries/CTEs |
| `CARTESIAN_JOIN` | Join has no predicate or low-confidence inferred relationship | Add or correct join predicate |
| `LOW_SELECTIVITY_INDEX` | Chosen index has poor cardinality for predicate | Prefer a more selective existing index or recommend composite |
| `MISSING_COMPOSITE_INDEX` | Query filters/orders/groups by columns no existing index covers | Recommend a composite index (if rewrites are insufficient) |
| `SORT_OR_TEMP_PRESSURE` | Plan uses expensive sort, temp table, or filesort | Align ORDER BY/GROUP BY with an index, or reduce input rows earlier |
| `CORRELATED_SUBQUERY_RISK` | Subquery executes per outer row with high estimated loops | Rewrite as join, derived table, or CTE |
| `OFFSET_PAGINATION_RISK` | Large OFFSET requires scanning skipped rows | Recommend keyset pagination |

For each issue found, note its severity (high/medium/low), which tables are affected, and the specific evidence from the plan.

### Step 10: Generate and Validate Rewrites

Apply up to three rewrite strategies in order. Each must use a distinct approach. Stop a strategy early if it cannot preserve semantics — never invent columns, relationships, or business rules.

**Strategy 1: Predicate and Projection Rewrite**
- Make filters sargable: unwrap functions on indexed columns by moving transformations to the constant side
- Remove unused columns from SELECT list
- Simplify redundant predicates

**Strategy 2: Join and Subquery Rewrite**
- Reorder joins so smaller/more-selective tables drive
- Convert correlated subqueries to JOIN, derived table, or CTE
- Push filters closer to their source tables
- Remove unnecessary derived tables

**Strategy 3: Aggregation and Shape Rewrite**
- Use CTEs instead of repeated subqueries
- Pre-aggregate in a derived table before joining to large tables
- Use window functions instead of self-joins when semantically equivalent
- Replace `NOT IN` with `NOT EXISTS` or anti-join when appropriate

For complex queries involving multiple tables or subqueries, try at least 2 distinct variants of each applicable strategy before declaring it exhausted. For example, within Strategy 2, try both an optimizer hint (like `FORCE INDEX`) and a derived-table approach (pre-filtering in a subquery) before moving to Strategy 3. Single-table queries with one clear fix can skip this — apply the obvious fix directly.

For each strategy:
1. Write the rewritten SQL
2. Validate it: `querylex validate "<rewritten SQL>"`
3. If validation fails, fix and retry up to 2 more times (3 total attempts per strategy)
4. If validation succeeds, run `querylex explain "<rewritten SQL>"` (with same `--analyze` flag as original)
5. Compare plans against the original
6. **Always record the validation result** — note whether it passed and capture the normalized SQL. You will mention this in the final output so the user knows the rewrite was tested.

### Step 11: Determine if a Rewrite is Better

A rewrite is considered better when ANY of these conditions is true:

| Condition | Threshold |
|-----------|-----------|
| `--analyze` actual runtime improved | >= 10% faster WITHOUT increasing rows examined by >20% |
| Estimated total cost improved | >= 15% lower |
| Estimated rows examined improved | >= 25% lower WITHOUT introducing sorts/temp tables/cartesian joins |
| Severe qualitative issue removed | E.g., full table scan eliminated on large table, AND no metric regresses by >10% |

If `--analyze` metrics conflict with estimated metrics, actual runtime wins.

### Step 12: Handle Each Outcome

**If a rewrite is better:**
1. Save it: `querylex save "<original_sql>" "<optimized_sql>"`
2. Present the optimized SQL in a code block with the dialect-appropriate language tag
3. Show plan comparison: cost/rows before → after, which metrics improved
4. Explain each change and why it helped
5. Mention that the rewrite was validated with `querylex validate` and passed
6. Note any context warnings (stale artifacts, missing stats, etc.)

**If no rewrite is better and `--no-index` is NOT set:**
1. Analyze the plan bottlenecks that rewrites couldn't address
2. If a bottleneck maps to a missing or inadequate index on a table large enough to justify it, recommend a dialect-appropriate `CREATE INDEX` statement
3. The recommendation must name the affected query predicate, join, sort, or grouping
4. Always include a warning to test outside production first

**If no rewrite is better and `--no-index` IS set, or no index would help:**
Return unable-to-optimize:
- Best validated attempt (if any) with why it wasn't better
- Attempt log for all strategies tried
- Context coverage score and missing context warnings
- Dialect-aware next steps (e.g., for MySQL: review partitioning, `innodb_buffer_pool_size`; for PostgreSQL: `ANALYZE` freshness, `work_mem`; etc.)

### Final Output Format

Present results concisely:

1. **If optimized**: show the optimized SQL in a code block, then a brief before/after comparison, then what changed and why.
2. **If index recommended**: show the `CREATE INDEX` statement, the target predicate/join, and expected plan impact.
3. **If unable-to-optimize**: show what was tried, why it didn't help, and next steps.

Do not include low-value system warnings (like `MEMORY_INDEX_STALE`, `EMBEDDINGS_UNAVAILABLE`) in the final output. Only mention things that materially affect correctness, such as a stale index or missing statistics that forced a fallback decision.

## Command Reference

All commands return JSON envelopes.

```bash
# Check workspace status
querylex workspace-stats

# Check memory for cached optimization
querylex memory "the original SQL"

# Validate SQL
querylex validate "SELECT ..."

# Get explain plan (estimated)
querylex explain "SELECT ..."

# Get explain plan (with execution)
querylex explain "SELECT ..." --analyze

# Fetch schema for tables
querylex schema --table table1 --table table2

# Fetch join paths
querylex joins --table table1 --table table2

# Fetch table statistics
querylex stats --table table1 --table table2

# Fetch index information
querylex indexes --table table1 --table table2

# Save optimized query to memory
querylex save "original SQL" "optimized SQL"
```

### Parsing JSON Responses

Every querylex response follows this envelope:

```json
{
  "success": true/false,
  "data": { ... },
  "error": {"code": "ERROR_CODE", "message": "...", "retryable": true/false},
  "warnings": [{"code": "WARN_CODE", "message": "..."}],
  "meta": {"trace_id": "...", "protocol_version": "1.0.0", "duration_ms": 42}
}
```

Key data fields per command:
- **workspace-stats**: `data.active_database_id`, `data.connected_databases[]` (each has `.id`, `.name`, `.type`, `.status`, `.indexing_progress`)
- **memory**: `data.match_found`, `data.similarity`, `data.entry` (with `.input`, `.sql`)
- **validate**: `data.valid`, `data.normalized_sql`, `data.read_only`
- **explain**: `data.execution_plan` (`.estimated_total_cost`, `.estimated_rows_examined`, `.full_scan_tables[]`, `.index_usage[]`, `.sort_operations`, `.temp_operations`, `.join_operations[]`), `data.heuristics[]` (each has `.code`, `.severity`, `.detail`), `data.analyze`
- **schema**: `data.tables[]` (each has `.table`, `.schema`, `.columns[]` with `.name`, `.type`, `.nullable`, `.primary_key`)
- **joins**: `data.joins[]` (each has `.source`, `.target`, `.columns[][]`, `.confidence`, `.source_type`)
- **stats**: `data.tables[]` (each has `.table`, `.row_count`, `.data_length`, `.index_length`, `.statistics_updated`)
- **indexes**: `data.tables[]` (each has `.indexes[]` with `.name`, `.type`, `.unique`, `.primary`, `.visible`, `.columns[]`)
- **save**: `data.saved`, `data.entry` (with `.id`, `.input`, `.sql_hash`)
