---
name: querylex-optimize
description: Optimize SQL queries using explain plans, schema context, statistics, indexes, joins, and dialect-aware rewrite heuristics via the QueryLex CLI toolchain. Use this skill whenever the user asks to optimize, tune, speed up, or improve a SQL query's performance — even if they don't explicitly say "optimize". This includes phrases like "my query is slow", "this query takes forever", "how can I make this faster", "find the bottleneck", "analyze this explain plan", "can you rewrite this to run better", "this query is doing a full table scan", or "help me fix this slow query". Also use when the user pastes a SQL query and asks for performance analysis, or when they share an explain plan output and ask what's wrong. Do NOT use this skill for generating SQL from natural language (that's /querylex-sql) or for general schema exploration.
---

# querylex-optimize: SQL Query Optimization

Optimize a supplied SQL query by orchestrating the QueryLex CLI toolchain. The process validates the SQL, analyzes its execution plan, gathers schema/statistics/index/join context, generates alternative rewrites using three distinct strategies, validates each rewrite, compares plans, and saves the best optimization to memory.

## Prerequisites

The `querylex` binary must be on `$PATH`. Verify with `which querylex`. The working database must be indexed (one-time `querylex add-db` setup). If the binary is not found, build it from the project root: `go build -o /usr/local/bin/querylex ./cmd/querylex`.

## User Interface

The user provides a SQL query to optimize. They may optionally include:

- `--analyze` — Run `EXPLAIN ANALYZE` (actually executes the query) for runtime metrics. Warn the user that the database may execute the query before proceeding. Use this flag when the user explicitly requests it.
- `--no-index` — Do not recommend new indexes. Only rewrite the SQL. If no rewrite improves the query, return the unable-to-optimize result without suggesting indexes.

## Hard Gates — When to Stop Immediately

These are non-negotiable failure conditions. When any of these occur, stop the workflow and report the failure to the user. Do not attempt to continue, fall back, or work around. The goal is a definitive answer, not a best-effort guess.

| Gate | Condition | Required Action |
|------|-----------|-----------------|
| **Credential failure** | `workspace-stats` returns `CREDENTIAL_UNAVAILABLE` or `CONNECTION_FAILED` | Stop. Tell the user: "Credentials are unavailable or the database connection failed. Run `querylex workspace-stats --human` to diagnose. You may need to re-add the database with `querylex add-db`." |
| **No active database** | `workspace-stats` returns `active_database_id: null` or no connected databases | Stop. Tell the user: "No active database is configured. Run `querylex add-db` to set up a database connection, or use `querylex workspace-stats` to select an existing one." |
| **Workspace corrupted** | `workspace-stats` fails with `WORKSPACE_STATE_INVALID` or parse error | Stop. Report the file `$HOME/.querylex/querylex.json` may be malformed. Suggest restoring from backup or re-running `querylex add-db`. |
| **Database not indexed** | Active database status is `not_indexed` | Stop. Tell the user: "The active database has not been indexed. Run `querylex add-db` to complete indexing. Indexing provides the schema, join, and index metadata needed for optimization." |
| **Validate failure** | `querylex validate` returns `data.valid: false` on the original SQL | Stop. The original SQL is not valid against the database schema — optimization is not meaningful. Report the validation error and suggest fixes. |
| **Explain failure** | `querylex explain` fails on the original SQL (timeout, connection error, plan extraction failure) | Stop. Without a baseline plan, there is nothing to compare against. Report: "Could not obtain an execution plan for this query. The database may be unreachable or the query may be invalid. Check: `querylex explain \"your SQL\"`." If the error is `EXPLAIN_FAILED`, suggest checking database permissions (`SHOW EXPLAIN` or equivalent). |
| **All context sources fail** | Schema, stats, indexes, and joins ALL return errors | Stop. Report: "No context could be gathered — the database appears unreachable. All four context commands (schema, stats, indexes, joins) failed. Check your database connection and try again." |
| **All rewrite strategies fail validation** | All 3 rewrite strategies produce SQL that fails `querylex validate` after 3 retries each | Stop. Report each strategy's failure log. Tell the user: "None of the three optimization strategies produced valid SQL. This may indicate the optimizations conflict with schema constraints. Manual review is needed." |
| **Timeout** | Any `querylex` command takes longer than 45 seconds | Stop. Report which command timed out. Tell the user: "Command `<command>` timed out after 45 seconds. The database may be under heavy load or the query may be too complex. Try with a simpler query or check database health." |
| **Resolve failure** | `querylex validate` cannot resolve table or column references (`TABLE_NOT_FOUND`, `COLUMN_NOT_FOUND`) | Stop. The query references objects that don't exist in the schema. Optimization requires valid table/column references. Report which tables or columns are missing. |

These gates exist because optimization without reliable infrastructure produces misleading results. A wrong optimization is worse than no optimization — it wastes the user's time (they deploy a broken query) and erodes trust in the tool. If any gate fails, give the user a clear path to fix it before retrying.

## The 12-Step Workflow

Execute these steps in order. After every `querylex` command, check how long it took via `meta.duration_ms`. If any command exceeds 45000ms (45 seconds), trigger the timeout gate and stop.

All `querylex` commands return JSON envelopes with `success`, `data`, `error`, `warnings`, and `meta` fields. Check `success` first; if false, read `error.code` and `error.message`.

### Step 1: Receive the SQL and Flags

Capture the user's SQL query and note any flags (`--analyze`, `--no-index`). This SQL is referred to as the **original SQL**. Normalize line breaks and whitespace but preserve the query structure.

### Step 2: Active Database Preflight

Run `querylex workspace-stats` to get the status of all connected databases and the active database.

Parse the JSON response:
- If `success: false`, check `error.code`:
  - `CREDENTIAL_UNAVAILABLE` → trigger **Credential failure** gate. Stop.
  - `CONNECTION_FAILED` → trigger **Credential failure** gate. Stop.
  - `WORKSPACE_STATE_INVALID` → trigger **Workspace corrupted** gate. Stop.
  - Any other error → stop and report it.
- If `success: true`:
  - If `data.active_database_id` is `null` or `data.connected_databases` is empty → trigger **No active database** gate. Stop.
  - Identify the active database's `type` (mysql, postgresql, mssql, sqlite). This determines the SQL dialect for all subsequent work.
  - Identify each database's `status` and handle per the table below.

Handle each indexing status per this table:

| Status | Action |
|--------|--------|
| `indexed` | Proceed normally. |
| `stale` | Proceed with a warning that artifacts may be outdated. |
| `indexing` | Show last-known progress. Proceed only if a previous successful manifest exists; if not, trigger **Database not indexed** gate. Stop. |
| `not_indexed` | Trigger **Database not indexed** gate. Stop. |
| `index_failed` | Stop unless a previous successful manifest exists. If proceeding, include a warning about stale artifacts in every result. If no manifest exists, trigger **Database not indexed** gate. Stop. |
| Workspace missing or malformed | Trigger **Workspace corrupted** gate. Stop. |

Tell the user which active database is being used: "Using database: `<name>` (`<type>`)"

### Step 3: Check Memory for Cached Optimization

Run `querylex memory "<original SQL>"` to check whether this query (or a very similar one) has already been optimized.

Parse the JSON response:
- On timeout (>45s): log a warning ("Memory check timed out — continuing without cache") and continue to step 4.
- On error (`success: false`): log a warning and continue to step 4. Memory is an optimization, not a prerequisite.
- On strong match (`data.match_found: true`, similarity >= 0.86): show the cached optimized SQL, explain why it matched (similarity score, matching reason), and ask the user whether they want to use the cached result or request fresh analysis.
  - If user accepts cached: skip to the end. Do not save — it's already in memory.
  - If user wants fresh: continue to step 4.
- On no match: continue to step 4.

### Step 4: Validate the Original SQL

Run `querylex validate "<original SQL>"`.

Parse the JSON response:
- If `success: false`:
  - `TABLE_NOT_FOUND` or `COLUMN_NOT_FOUND` → trigger **Resolve failure** gate. The query references objects that don't exist. Stop.
  - `UNSAFE_SQL` → return the error and stop. The query contains DML/DCL statements.
  - `INVALID_SQL` → trigger **Validate failure** gate. Stop.
  - Any other error → trigger **Validate failure** gate. Stop.
- If `data.valid: true`: record `data.normalized_sql` as the **normalized original SQL** — use this for all subsequent steps. Record `data.tables[]` as the set of referenced tables.
- If timeout (>45s) → trigger **Timeout** gate. Stop.

### Step 5: Explain the Original SQL

If `--analyze` was specified, run `querylex explain "<original SQL>" --analyze`. Otherwise run `querylex explain "<original SQL>"`.

The `--analyze` flag passes through to the database engine and may actually execute the query. Warn the user about this before running.

Parse the JSON response:
- If `success: false`:
  - `EXPLAIN_FAILED` → trigger **Explain failure** gate. Stop.
  - Timeout (>45s) → trigger **Timeout** gate. Stop.
  - Any connection or credential error → trigger **Credential failure** gate. Stop.
- If `success: true`: extract `data.execution_plan` as the **baseline plan**. Note any `data.heuristics[]` warnings — these are clues about what to optimize.

### Step 6: Extract Referenced Tables

Use `data.tables[]` from step 4 (the validate response) as the set of referenced tables. Include tables inside CTEs, subqueries, and derived tables. Build a variable `TABLES` (list of table names) and `TABLES_JSON` (the same as a JSON array string).

### Step 7: Fetch Context for Referenced Tables

Run these four commands, targeting all tables from step 6. Run them sequentially — if earlier commands timeout, stop immediately rather than continuing.

```
querylex schema --tables-json '<TABLES_JSON>'
querylex joins --tables-json '<TABLES_JSON>'
querylex stats --tables-json '<TABLES_JSON>'
querylex indexes --tables-json '<TABLES_JSON>'
```

Track each command's result: `schema_ok`, `joins_ok`, `stats_ok`, `indexes_ok` (boolean).

Error handling for each:
- **Schema**: on error, mark `schema_ok = false`. Schema loss reduces optimization accuracy significantly. Continue but note this is a major gap.
- **Joins**: on error, mark `joins_ok = false`. Without join paths, join-order rewrites are unreliable. Note this gap.
- **Stats**: on error, mark `stats_ok = false`. Without stats, cost-based decisions are estimates only. Note this gap.
- **Indexes**: on error, mark `indexes_ok = false`. Without index info, sargable predicate rewrites rely on heuristics only. Note this gap.

After all four commands complete, check the context coverage:

| Context coverage | Action |
|------------------|--------|
| All four failed (schema, stats, indexes, joins all `false`) | Trigger **All context sources fail** gate. Stop. |
| Three of four failed | Proceed only if at least schema OR indexes succeeded. If both schema AND indexes failed, trigger **All context sources fail** gate. Stop. |
| Two or fewer failed | Proceed with warnings for each missing source. |
| Zero failed | Proceed normally with full context. |

Report context coverage: "Context available: schema=<yes/no>, joins=<yes/no>, stats=<yes/no>, indexes=<yes/no>"

### Step 8: Analyze the Baseline Plan

With the normalized explain plan, schema, joins, stats, indexes, and dialect in hand, analyze why the query performs the way it does. Look for:

1. **Full table scans** — `data.execution_plan.full_scan_tables[]`. These are the highest-value targets. If a full-scanned table has no useful index and the filter is non-sargable (e.g., `WHERE YEAR(date_col) = 2020`), a predicate rewrite is the first priority.
2. **Heuristic warnings** — `data.heuristics[]`. Pay attention to `FULL_TABLE_SCAN`, `NON_SARGABLE_PREDICATE`, `MISSING_INDEX`, `IMPLICIT_TYPE_CONVERSION`, `SUBOPTIMAL_JOIN_ORDER`, `EXCESSIVE_SORTING`, `TEMPORARY_TABLE_USAGE`.
3. **Join operations** — `data.execution_plan.join_operations[]`. Nested loop joins on large tables, cartesian joins, or hash joins with high row counts suggest join-order or join-type issues.
4. **Index usage** — `data.execution_plan.index_usage[]`. If an index exists but isn't being used, check for function-wrapped columns, implicit conversions, or leading-wildcard LIKE patterns.
5. **Sort and temp operations** — `data.execution_plan.sort_operations` and `temp_operations`. Target these for index-based ORDER BY and GROUP BY rewrites.

Organize findings into a prioritized list. The items with the highest estimated impact go first.

### Step 9: Generate and Verify SQL Rewrites

Generate rewrites using three distinct strategies. Each strategy targets a different class of optimization problem. Run them in order; a successful earlier strategy may make later strategies unnecessary.

#### Strategy 1: Predicate and Projection Rewrite

Target: sargable predicates, function-unwrapping, unnecessary columns in SELECT.

- Replace `WHERE YEAR(date_col) = 2020` with `WHERE date_col >= '2020-01-01' AND date_col < '2021-01-01'`
- Replace `WHERE UPPER(name) = 'ACME'` with `WHERE name = 'ACME'` (if collation allows)
- Replace `SELECT *` with explicit column lists (especially when covering indexes exist)
- Move predicates closer to the table scan (push filters into subqueries/CTEs where possible)

#### Strategy 2: Join and Subquery Rewrite

Target: join order, join type, subquery-to-join conversion.

- Reorder joins so the most selective table drives the query (smallest row count after filters)
- Replace `NOT IN (SELECT ...)` with `NOT EXISTS` or anti-join patterns (handles NULL safely)
- Convert correlated subqueries to JOINs when the correlation is a simple equality
- Replace `WHERE col IN (SELECT ...)` with `WHERE EXISTS (SELECT 1 ...)` when appropriate, or with JOIN when distinctness isn't required
- For MySQL: consider `STRAIGHT_JOIN` or optimizer hints when the join graph shows obvious ordering issues

#### Strategy 3: Aggregation and Query-Shape Rewrite

Target: GROUP BY optimization, window functions, UNION to UNION ALL.

- Replace `UNION` with `UNION ALL` when the sub-results are known to be disjoint (UNION incurs a sort+dedup cost)
- Move aggregations into derived tables that can be materialized once, then joined
- Replace `GROUP BY` on a large table with pre-aggregation in a CTE when possible
- Add `WHERE` filters inside subqueries/CTEs to reduce rows before aggregation

**For each strategy:**

1. Produce the rewritten SQL.
2. Run `querylex validate "<rewritten SQL>"`. Track whether this strategy ever produced a validated rewrite (`strategy1_validated`, `strategy2_validated`, `strategy3_validated` — initialized to `false`).
3. If validation fails, analyze the error and fix it. Retry up to 3 times per strategy. Each retry must produce a meaningful fix — don't resubmit the same broken SQL.
4. After 3 validation failures for a strategy, abandon that strategy and log the failure with the error from the last attempt.
5. If validation succeeds, set the strategy's `_validated` flag to `true`. Run `querylex explain "<rewritten SQL>"` (with `--analyze` if the original used it).
6. If explain fails on a validated rewrite, log the failure but do not count it against the strategy — the rewrite itself was valid even if the plan couldn't be obtained.
7. Compare the rewrite's plan against the baseline plan.

After all three strategies complete, check the validation flags:

- If none of `strategy1_validated`, `strategy2_validated`, or `strategy3_validated` is `true` → trigger **All rewrite strategies fail validation** gate. Stop with the failure log.
- If at least one strategy produced a validated rewrite, proceed to the comparison phase.

**What counts as better:**

| Metric | What to compare |
|--------|----------------|
| Full table scans | Fewer full-scanned tables |
| Estimated cost | Lower `estimated_total_cost` |
| Rows examined | Fewer `estimated_rows_examined` |
| Index usage | New index usage where baseline had none, or covering index replacing index+table read |
| Sort operations | Fewer `sort_operations` |
| Temp operations | Fewer `temp_operations` |
| Join type | Merge/hash join replacing nested loop on large tables |

A rewrite is considered better if it improves at least one metric and degrades none significantly. Full-scan elimination is weighted highest; cost reduction is next. Minor cost fluctuations (<10%) on a non-bottleneck step are acceptable.

### Step 10: Save and Return if Better

If any rewrite from step 9 is better than the baseline:

1. Run `querylex save "<original SQL>" "<optimized SQL>"` to persist the optimization to memory.
2. On save success: the optimization is now cached for future `querylex memory` lookups.
3. On save warning: note that the optimization was generated but couldn't be saved. Show the SQL anyway.

Return the result to the user with:

1. **The optimized SQL** (in a code block with the appropriate dialect language tag, e.g. `sql`, `mysql`)
2. **What changed** — a bullet list of specific changes (e.g., "Replaced `DATE(created_at)` with range check: `created_at >= '2026-01-01' AND created_at < '2026-02-01'` — this makes the filter sargable and uses `idx_orders_date`")
3. **Plan comparison** — before/after key metrics. Focus on the difference the user will feel: fewer rows examined, index usage enabled, sort/temp eliminated.
4. **Strategy used** — which of the three strategies produced the winning rewrite.
5. **Any warnings** — stale context, partial stats, missing join paths, etc.

### Step 11: Index Recommendation (if no rewrite helped and --no-index is not set)

If no rewrite was better and the user did not specify `--no-index`:

Look at the baseline plan evidence:
- Full table scan on a table with no covering index → recommend an index on the filter columns
- Sort operation without `ORDER BY` index → recommend an index on the ORDER BY columns
- Nested loop join on a large table with foreign key → recommend an index on the join column (if missing)
- Missing index heuristic (`data.heuristics[]` contains `MISSING_INDEX`)

For each recommendation, return:
1. A `CREATE INDEX` statement (dialect-appropriate)
2. Which predicate/join/sort it targets
3. Expected impact on the plan
4. A warning to test in non-production first

### Step 12: Unable-to-Optimize Fallback

This step is reached only when strategies produced valid rewrites but none was better than the baseline. (If all strategies failed validation, the **All rewrite strategies fail validation** gate already stopped the workflow.)

Return:
1. The best validated rewrite attempt (the one whose plan was closest to the baseline, even if not strictly better)
2. An attempt log showing which strategies were tried, which produced valid rewrites, and why each was not better than baseline
3. Context coverage report — which context sources were available and which were missing
4. Dialect-aware next steps:
   - **MySQL/MariaDB**: suggest checking `innodb_buffer_pool_size`, table fragmentation, running `ANALYZE TABLE`, or enabling the performance_schema
   - **PostgreSQL**: suggest running `VACUUM ANALYZE`, checking `work_mem`, or enabling `auto_explain`
   - **SQLite**: suggest running `ANALYZE`, checking `cache_size` pragma
   - **MSSQL**: suggest updating statistics, checking `max degree of parallelism`, or reviewing missing index DMVs

## Final Output Guidelines

Keep the output focused on what the user needs to act on:

- **The SQL change** (if any) — they need to deploy it
- **The plan impact** — they need to justify the change
- **The index recommendation** (if any) — they need to evaluate it
- **Warnings** — only include if they materially affect correctness

Do not include diagnostic noise. Skip low-value warnings like `EMBEDDINGS_UNAVAILABLE` in the final output. If the workflow hit a hard gate and stopped, the gate's stop message (with fix instructions) is the complete output — no additional text needed. If the workflow completed but found no improvement, the unable-to-optimize breakdown explains the situation.

## Command Reference

All commands return JSON envelopes. Parse `success` first, then `data` or `error`.

```bash
# Check workspace status (active database, indexing state)
querylex workspace-stats

# Check memory for cached optimization
querylex memory "SELECT * FROM orders WHERE ..."

# Validate SQL against active database
querylex validate "SELECT ..."

# Explain execution plan (estimated)
querylex explain "SELECT ..."

# Explain with actual execution (executes the query)
querylex explain "SELECT ..." --analyze

# Fetch schema for tables
querylex schema --tables-json '["orders","lineitem","customer"]'

# Fetch join paths between tables
querylex joins --tables-json '["orders","lineitem","customer"]'

# Fetch table statistics
querylex stats --tables-json '["orders","lineitem","customer"]'

# Fetch index information
querylex indexes --tables-json '["orders","lineitem","customer"]'

# Save optimized query to memory
querylex save "SELECT * FROM orders WHERE ..." "SELECT o_orderkey, o_totalprice FROM orders WHERE o_orderdate >= '2026-01-01'"
```

### Parsing JSON Responses

Every querylex response follows this envelope:

```json
{
  "success": true,
  "data": { /* command-specific payload */ },
  "error": null,
  "warnings": [{ "code": "WARN_CODE", "message": "..." }],
  "meta": { "trace_id": "...", "protocol_version": "1.0.0", "duration_ms": 42 }
}
```

Key data fields per command for optimization:
- **workspace-stats**: `data.active_database_id`, `data.connected_databases[].type`, `data.connected_databases[].status`
- **memory**: `data.match_found`, `data.similarity`, `data.entry.sql` (the cached optimized SQL)
- **validate**: `data.valid`, `data.normalized_sql`, `data.tables[]`
- **explain**: `data.execution_plan` (with `.full_scan_tables[]`, `.index_usage[]`, `.sort_operations`, `.temp_operations`, `.join_operations[]`, `.estimated_total_cost`, `.estimated_rows_examined`), `data.heuristics[]`
- **schema**: `data.tables[].columns[]` (with `.name`, `.type`, `.nullable`, `.primary_key`), `data.tables[].constraints[]`
- **joins**: `data.joins[]` (with `.source`, `.target`, `.columns`, `.confidence`, `.source_type`)
- **stats**: `data.tables[].row_count`, `data.tables[].cardinality`, `data.tables[].last_analyzed_at`
- **indexes**: `data.tables[].indexes[]` (with `.name`, `.type`, `.unique`, `.columns[].name`, `.columns[].order`)
- **save**: `data.saved`, `data.updated_existing`
