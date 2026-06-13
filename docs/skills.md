# QueryLex Skills Reference

Two skills are available, installed via `npx skills add cskiller24/querylex`. Both skills use the QueryLex CLI binary (`querylex`) as their backend — it must be on `$PATH` and the working database must be indexed (one-time `querylex add-db` setup).

---

## Skill 1: querylex-sql — Natural Language to SQL

### Purpose

Translates natural language questions into dialect-correct SQL by orchestrating the QueryLex CLI toolchain. The process introspects the active database, resolves intent to candidate tables/columns, checks memory for cached results, gathers schema/join/index context, constructs SQL, validates it against the live schema, and saves accepted queries to memory.

### Trigger

Triggered whenever the user asks a natural-language question about their database data. This includes phrases like:
- "show me all employees hired last year with their salary"
- "what's the average salary by department?"
- "write a SQL query for..."
- "list the top 5 customers by total spend"
- Any question that implies database lookups

Do NOT use for SQL optimization (that is `/querylex-optimize`) or for general schema exploration.

### Prerequisites

- `querylex` binary on `$PATH` (verify with `which querylex`)
- An indexed database (one-time `querylex add-db` setup)
- `querylex workspace-stats` returns a healthy active database

### The 11-Step Workflow

Execute these steps in order, one at a time. Every `querylex` subcommand returns a JSON envelope — always check `success` first.

---

#### Step 1: Receive the User Question

Capture the user's natural language question exactly as phrased. This is the `question` string used in subsequent commands.

---

#### Step 2: Active Database Preflight

Run `querylex workspace-stats` to get workspace status, active database ID, type, and indexing state.

| Condition | Action |
|-----------|--------|
| Command fails (no workspace) | Stop. Tell the user no workspace exists. Ask them to run `querylex add-db`. |
| No `active_database_id` | Stop. Show connected databases and ask user to select or add one. |
| Status `not_indexed` | Stop. Ask user to complete indexing first. |
| Status `indexing` | Show progress. Proceed only if a previous manifest exists and user accepts stale context. |
| Status `index_failed` | Stop unless a previous manifest exists. |
| Status `stale` | Proceed with a warning that artifacts may be outdated. |
| Status `indexed` | Proceed normally. |

Extract the database type (`type` field — mysql, postgresql, sqlite, mssql, mariadb) for dialect-aware SQL construction in step 9.

---

#### Step 3: Resolve Intent

Run `querylex resolve "<question>"` to identify relevant tables and columns.

- On error: return the error and stop.
- On empty result (tables is empty array or `NO_MATCHING_TABLES` warning): tell the user no relevant tables were found and ask for more context. Stop.
- On success: extract table names from `data.tables[].table`. Store two variables:
  - `TABLES` — the list of resolved table names (e.g., `["employees", "salaries"]`)
  - `TABLES_JSON` — the same list as a JSON array string (e.g., `'["employees","salaries"]'`)

---

#### Step 4: Check Memory

Run `querylex memory "<question>"` to search for cached similar queries.

- On error: log a warning and continue to step 5.
- On strong match (similarity >= 0.86): show the cached SQL and ask the user whether to use it or generate fresh. If they accept, skip to step 11 (don't re-save). If they want fresh, continue.
- On no match: run `querylex history "<question>"` for reference patterns (optional, not authoritative).

---

#### Step 5: Read Terminology Map

Read `$HOME/.querylex/<db-id>/terminologies.md`. Parse the fenced YAML block marked `querylex-terms`:

```yaml
terms:
  - term: revenue
    type: metric
    maps_to:
      - table: orders
        column: total_amount
    description: Gross order value before refunds.
```

- Missing file: warn and continue (create a blank template if possible).
- Parse error: warn with `TERMINOLOGY_PARSE_ERROR` and continue.

Map any business terms found to their corresponding tables and columns. These mappings inform SQL construction in step 9.

---

#### Steps 6-8: Gather Context

Run these three commands one at a time, in any order. Each targets the resolved tables from step 3.

**Step 6 — Schema:**
```bash
querylex schema --table employees --table salaries --table dept_emp --table departments
```
On error: warn and continue (schema improves accuracy but doesn't always block generation).

**Step 7 — Joins:**
```bash
querylex joins --table employees --table salaries --table dept_emp --table departments
```
On error: stop only if the question requires joining tables that cannot be connected. Otherwise warn and continue.

**Step 8 — Indexes:**
```bash
querylex indexes --table employees --table salaries --table dept_emp --table departments
```
On error: warn and continue (index info helps choose sargable filters).

---

#### Step 9: Construct SQL

Use all gathered context to construct a dialect-appropriate SQL query.

**Dialect awareness:** The database type from step 2 determines SQL dialect:

| Feature | MySQL | PostgreSQL | MSSQL | SQLite |
|---------|-------|------------|-------|--------|
| Identifier quoting | backticks | double quotes | brackets | none |
| LIMIT syntax | `LIMIT N OFFSET M` | `LIMIT N OFFSET M` | `OFFSET M ROWS FETCH NEXT N ROWS ONLY` | `LIMIT N OFFSET M` |
| Date functions | `DATE_SUB(NOW(), INTERVAL 5 YEAR)` | `NOW() - INTERVAL '5 years'` | `DATEADD(YEAR, -5, GETDATE())` | `date('now', '-5 years')` |
| String concat | `CONCAT()` | `\|\|` | `+` | `\|\|` |

**How to use each context type:**

- **Schema (step 6):** Use column names, data types, nullability, and constraints. Pay attention to generated columns (don't filter on them directly) and nullable columns (for LEFT JOIN vs INNER JOIN decisions).
- **Joins (step 7):** Prefer high-confidence edges (confidence=1.0, declared foreign keys) over inferred edges. If no join path exists between required tables, flag this to the user.
- **Indexes (step 8):** Write sargable predicates that can use available indexes. Match composite index leading column order. Avoid wrapping indexed columns in functions.
- **Terminology (step 5):** Map business terms from the user's question to actual table/column names (e.g., "current salary" → `salaries` with `to_date = '9999-01-01'`).

---

#### Step 10: Validate

Run `querylex validate "<SQL>"` to validate against the database schema.

- On success (`data.valid: true`): proceed to step 11.
- On failure: analyze the error. Common failures:
  - `TABLE_NOT_FOUND` — wrong table name or schema qualification
  - `COLUMN_NOT_FOUND` — column doesn't exist; recheck schema from step 6
  - `INVALID_SQL` — syntax error
  - `UNSAFE_SQL` — DML/DCL detected; rewrite as SELECT-only
- Retry up to 3 times total (initial + 2 retries). Each retry must produce a different fix.
- After 3 failures: show the user which errors occurred and the best attempt so far.

---

#### Step 11: Save and Update Terminology

On successful validation, save the query to memory:

```bash
querylex save "<question>" "<SQL>"
```

- On success: the query is now in memory for future retrieval.
- On error: warn that the SQL was generated but could not be saved. Show the SQL and suggest manual save later.

After saving, update the terminology map with any business term mappings discovered during this workflow. Read `$HOME/.querylex/<db-id>/terminologies.md`, add or update entries for terms found in the user's question that mapped to specific tables and columns, and write the updated file.

### Output

Present to the user:
1. The generated SQL in a code block with the appropriate dialect language tag
2. A brief explanation of key join paths and filters used

Keep output concise. Do not include low-value system warnings like `EMBEDDINGS_UNAVAILABLE` or `MEMORY_INDEX_STALE` in the final output. Only mention things that materially affect correctness (stale index, missing statistics that forced a fallback decision).

### Usage Example

```
User: /querylex-sql "show me all employees hired in the last 5 years with
                     their current salary and department"

1. querylex workspace-stats → active DB = prod, type = mysql, status = indexed
2. querylex resolve "employees hired last 5 years salary department"
   → tables: [employees, salaries, dept_emp, departments]
3. querylex memory "..." → no strong match
4. querylex history "..." → reference patterns noted
5. Read terminologies.md → "current salary" → salaries.to_date filter
6. querylex schema --table employees salaries dept_emp departments
7. querylex joins --table employees salaries dept_emp departments
8. querylex indexes --table employees salaries dept_emp departments
9. Construct MySQL SQL:
   SELECT e.emp_no, e.first_name, e.last_name, s.salary, d.dept_name
   FROM employees e
   JOIN salaries s ON e.emp_no = s.emp_no AND s.to_date = '9999-01-01'
   JOIN dept_emp de ON e.emp_no = de.emp_no AND de.to_date = '9999-01-01'
   JOIN departments d ON de.dept_no = d.dept_no
   WHERE e.hire_date >= DATE_SUB(CURRENT_DATE, INTERVAL 5 YEAR);
10. querylex validate "..." → valid
11. querylex save "show me all employees hired..." "..."

Output: SQL block + explanation of joins through employees→salaries (emp_no),
employees→dept_emp (emp_no), dept_emp→departments (dept_no), and temporal
filters on salaries/dept_emp.
```

---

## Skill 2: querylex-optimize — SQL Optimization

### Purpose

Optimize SQL queries using explain plans, schema context, statistics, indexes, joins, and dialect-aware rewrite heuristics. The process validates the input SQL, fetches its explain plan, gathers context for referenced tables, applies rewrite strategies, compares plans, and recommends indexes when rewrites are insufficient.

### Trigger

Triggered whenever the user asks to improve SQL performance. This includes:
- "optimize this query: SELECT ..."
- "my query is slow", "this query takes forever"
- "how can I make this faster", "find the bottleneck"
- "analyze this explain plan", "can you rewrite this to run better"
- User pastes a SQL query and asks for performance analysis
- User shares an explain plan output and asks what's wrong

Do NOT use for generating SQL from natural language (that is `/querylex-sql`).

### Prerequisites

- `querylex` binary on `$PATH`
- An indexed database

### Flags

| Flag | Behavior |
|------|----------|
| `--analyze` | Pass `--analyze` through to `querylex explain`. Warn the user the database may execute the query. |
| `--no-index` | The skill may rewrite SQL but must not recommend new indexes. |

### The 12-Step Workflow

Execute these steps in order, one at a time. Every `querylex` subcommand returns a JSON envelope — always check `success` first.

---

#### Step 1: Receive the SQL

Capture the user's SQL query exactly as provided, along with any flags (`--analyze`, `--no-index`). This is the `original_sql` used in subsequent steps.

---

#### Step 2: Active Database Preflight

Run `querylex workspace-stats` to get workspace status, active database ID, type, and indexing state.

| Status | Action |
|--------|--------|
| Command fails | Stop. No workspace exists. Ask user to run `querylex add-db`. |
| Workspace corrupt | Stop. Report `WORKSPACE_STATE_INVALID`. |
| No `active_database_id` | Stop. Show connected databases and ask user to select or add one. |
| `not_indexed` | Stop. Ask user to complete indexing first. |
| `indexing` | Show progress. Proceed only if a previous manifest exists and user accepts stale context. |
| `index_failed` | Stop unless a previous manifest exists. |
| `stale` | Proceed with warning. |
| `indexed` | Proceed normally. |

Extract the database type for dialect-aware rewriting.

---

#### Step 3: Check Memory

Run `querylex memory "<original_sql>"` to check for a cached optimization.

- On error: log a warning and continue.
- On match (similarity >= 0.86): show the cached optimization and ask the user whether to use it or run fresh analysis. If they accept, stop.
- On no match: continue.

---

#### Step 4: Validate Original SQL

Run `querylex validate "<original_sql>"`.

- On failure: report the validation error and stop. Show the error code and message — fix the SQL before retrying optimization.
- On success: capture `normalized_sql` from the response. This is the canonical form for all remaining steps.

---

#### Step 5: Get Explain Plan

Run `querylex explain "<normalized_sql>"` (with `--analyze` if the user passed that flag).

- On failure: report the error and stop.
- On success: save the execution plan and heuristics. The `execution_plan` includes `estimated_total_cost`, `estimated_rows_examined`, `full_scan_tables`, `index_usage`, `sort_operations`, `temp_operations`, `join_operations`. The `heuristics` array contains detected issues like `FULL_TABLE_SCAN`, `NON_SARGABLE_PREDICATE`, `MISSING_INDEX`, `HIGH_COST_ESTIMATE`.

If `--analyze` was used, the plan also includes `actual_total_time_ms` and `actual_rows_examined` — the gold-standard comparison metrics.

---

#### Step 6: Extract Referenced Tables

Parse the explain plan and normalized SQL to identify all tables referenced, including those inside CTEs, subqueries, derived tables, and views when resolvable.

---

#### Step 7: Fetch Context for Referenced Tables

Run these four commands sequentially. Each failure is recorded as missing context but does not automatically block optimization unless coverage is critically low.

```bash
querylex schema --table table1 --table table2 ...
querylex joins --table table1 --table table2 ...
querylex stats --table table1 --table table2 ...
querylex indexes --table table1 --table table2 ...
```

- **Schema:** On error, warn and continue. Missing schema makes column-level rewrites risky.
- **Joins:** On error, stop only if the SQL requires joining tables that cannot be connected. Otherwise warn and continue.
- **Stats:** On error, warn and continue. Missing stats reduce confidence in join-order and filter-placement decisions.
- **Indexes:** On error, warn and continue. Missing index metadata blocks index recommendations.

---

#### Step 8: Compute Context Coverage

Score your available context:

| Signal | Weight | Notes |
|--------|--------|-------|
| Explain plan available | 35 | Always present if step 5 succeeded |
| Schema for referenced tables | 25 | Count only if ALL referenced tables had schema |
| Join graph available | 15 | Count if joins command succeeded for the table set |
| Statistics available | 15 | Count if stats succeeded for at least half of tables |
| Index metadata available | 10 | Count if indexes succeeded for at least half of tables |

**Coverage = sum of weights (max 100).**

| Score | Capability |
|-------|------------|
| 80–100 | Full context. All rewrite strategies and index recommendations. |
| 50–79 | Partial context. May rewrite SQL but avoid high-risk structural rewrites (join reordering, subquery-to-join) unless plan evidence strongly supports them. Include context warnings. |
| 35–49 | Plan-only context. Only low-risk rewrites (non-sargable fixes, narrowing projections). Do NOT recommend indexes or restructure joins. |
| < 35 | Insufficient context. Return unable-to-optimize listing which sources failed. Stop. |

---

#### Step 9: Analyze the Plan

Examine the explain plan and heuristics for these issue classes:

| Code | Description | What to do |
|------|-------------|------------|
| `FULL_TABLE_SCAN` | Large table scanned without selective predicate or usable index | Add selective predicate, make predicate sargable, or recommend an index |
| `NON_SARGABLE_PREDICATE` | Indexed column wrapped in a function (`DATE(col)`, `YEAR(col)`, `LOWER(col)`) | Move transform to constant side: `col >= 'start' AND col < 'end+1'` |
| `UNBOUNDED_SELECT_STAR` | `SELECT *` against large or joined tables | Project only required columns |
| `JOIN_ORDER_RISK` | Large table joined before selective filters or smaller driving tables | Reorder joins or push filters into subqueries/CTEs |
| `CARTESIAN_JOIN` | Join has no predicate or low-confidence inferred relationship | Add or correct join predicate |
| `LOW_SELECTIVITY_INDEX` | Chosen index has poor cardinality for predicate | Prefer a more selective existing index or recommend composite |
| `MISSING_COMPOSITE_INDEX` | Query filters/orders/groups by columns no existing index covers | Recommend a composite index (if rewrites insufficient) |
| `SORT_OR_TEMP_PRESSURE` | Plan uses expensive sort, temp table, or filesort | Align ORDER BY/GROUP BY with an index, or reduce input rows earlier |
| `CORRELATED_SUBQUERY_RISK` | Subquery executes per outer row with high estimated loops | Rewrite as join, derived table, or CTE |
| `OFFSET_PAGINATION_RISK` | Large OFFSET requires scanning skipped rows | Recommend keyset pagination |

---

#### Step 10: Generate and Validate Rewrites

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

For complex multi-table queries, try at least 2 distinct variants of each applicable strategy before declaring it exhausted. Single-table queries with one clear fix can skip this — apply the obvious fix directly.

For each strategy:
1. Write the rewritten SQL
2. Validate it: `querylex validate "<rewritten SQL>"`
3. If validation fails, fix and retry up to 2 more times (3 total per strategy)
4. If validation succeeds, run `querylex explain "<rewritten SQL>"` (with same `--analyze`)
5. Compare plans against the original
6. Always record the validation result — note whether it passed and capture the normalized SQL

---

#### Step 11: Determine if a Rewrite is Better

A rewrite is better when ANY condition is true:

| Condition | Threshold |
|-----------|-----------|
| `--analyze` actual runtime improved | >= 10% faster WITHOUT increasing rows examined >20% |
| Estimated total cost improved | >= 15% lower |
| Estimated rows examined improved | >= 25% lower WITHOUT introducing sorts/temp tables/cartesian joins |
| Severe qualitative issue removed | Full table scan eliminated on large table, AND no metric regresses >10% |

If `--analyze` metrics conflict with estimated metrics, actual runtime wins.

---

#### Step 12: Handle Each Outcome

**If a rewrite is better:**
1. Save it: `querylex save "<original_sql>" "<optimized_sql>"`
2. Present the optimized SQL in a code block with dialect language tag
3. Show plan comparison: cost/rows before → after, which metrics improved
4. Explain each change and why it helped
5. Mention the rewrite was validated and passed
6. Note any context warnings (stale artifacts, missing stats)

**If no rewrite is better and `--no-index` is NOT set:**
1. Analyze the bottlenecks that rewrites couldn't address
2. If a bottleneck maps to a missing or inadequate index on a large enough table, recommend a dialect-appropriate `CREATE INDEX`
3. Name the affected query predicate, join, sort, or grouping
4. Always include a warning to test outside production first

**If no rewrite is better and `--no-index` IS set, or no index would help:**
Return unable-to-optimize:
- Best validated attempt (if any) with why it wasn't better
- Attempt log for all strategies tried
- Context coverage score and missing context warnings
- Dialect-aware next steps (e.g., MySQL: review partitioning, `innodb_buffer_pool_size`; PostgreSQL: `ANALYZE` freshness, `work_mem`)

### Output

1. **If optimized:** optimized SQL in code block, before/after comparison, what changed and why
2. **If index recommended:** `CREATE INDEX` statement, target predicate/join, expected plan impact
3. **If unable-to-optimize:** what was tried, why it didn't help, next steps

Do not include low-value system warnings (`MEMORY_INDEX_STALE`, `EMBEDDINGS_UNAVAILABLE`) in the final output. Only mention things that materially affect correctness.

### Usage Example

```
User: /querylex-optimize "SELECT * FROM orders WHERE YEAR(order_date) = 2024"
       (with --analyze flag)

1. querylex workspace-stats → active DB = prod, type = mysql, status = indexed
2. querylex memory "SELECT * FROM orders WHERE YEAR(order_date) = 2024" → no match
3. querylex validate "..." → valid
4. querylex explain --analyze "..." → full_scan, actual_total_time_ms = 1200ms
5. Referenced tables: [orders]
6-7. querylex schema --table orders, joins, stats, indexes → idx_order_date exists
8. Coverage = 100 (plan + schema + joins + stats + indexes)
9. Issue: NON_SARGABLE_PREDICATE — YEAR(order_date) wraps indexed column
10. Strategy 1 rewrite: WHERE order_date >= '2024-01-01' AND order_date < '2025-01-01'
11. Validate → valid. Explain → index_seek, cost -95%, rows 365 vs 5M
12. Better by all thresholds. Save.

Output:

```sql
SELECT * FROM orders
WHERE order_date >= '2024-01-01' AND order_date < '2025-01-01';
```

**Before:** Full table scan (5M rows), 1200ms actual
**After:** Index seek on idx_order_date (~365 rows), ~6ms actual

**What changed:** Replaced `YEAR(order_date) = 2024` with a range predicate.
The original wrapped the indexed column in a function, making the index
non-sargable. The rewrite allows the MySQL optimizer to seek directly to
matching rows rather than scanning the entire table.
```

---

## Skill Interaction

Both skills use the same QueryLex CLI commands as building blocks, which enables data sharing and avoids redundant work:

```
querylex workspace-stats   ← both: entry point for every session
querylex memory            ← both: cache check before work
querylex validate          ← both: SQL correctness gate
querylex schema            ← both: column/type context
querylex joins             ← both: table relationships
querylex indexes           ← both: sargability guidance
querylex save              ← both: persist accepted results

querylex resolve           ← sql only: NL → table resolution
querylex history           ← sql only: reference patterns (fallback)
querylex explain           ← optimize only: plan analysis
querylex stats             ← optimize only: table size metrics
```

### Shared State

Both skills read from and write to the same SQLite-backed memory store (`querylex memory` / `querylex save`). A query cached by the SQL skill is immediately retrievable by the Optimize skill, and vice versa. Both skills also share the same per-database terminologies map (`$HOME/.querylex/<db-id>/terminologies.md`), meaning terminology learned by one skill benefits the other.

### Ordering

Both skills require a completed `querylex add-db` + indexing before any subcommand works. `workspace-stats` is always the first step to verify readiness. The SQL skill must finish its 11-step workflow before its output can be optimized — the two skills compose naturally: generate first, then optimize.

---

## Command Reference for Skills

All commands return JSON envelopes.

```bash
# Workspace status
querylex workspace-stats

# NL resolution (sql skill)
querylex resolve "the user's question"

# Memory (both skills)
querylex memory "natural language input"

# History (sql skill fallback)
querylex history "topic keyword"

# Schema (both skills)
querylex schema --table table1 --table table2

# Joins (both skills)
querylex joins --table table1 --table table2

# Stats (optimize skill)
querylex stats --table table1 --table table2

# Indexes (both skills)
querylex indexes --table table1 --table table2

# Explain (optimize skill)
querylex explain "SELECT ..."
querylex explain --analyze "SELECT ..."

# Validate (both skills)
querylex validate "SELECT ..."

# Save (both skills)
querylex save "natural language" "SQL"

# Efficient table specification for many tables
querylex schema --tables-json '["t1","t2","t3"]'
querylex joins --tables-json '["t1","t2","t3"]'
querylex indexes --tables-json '["t1","t2","t3"]'
```

### Response Fields Per Command

| Command | Key data fields |
|---------|-----------------|
| `workspace-stats` | `active_database_id`, `connected_databases[].id`, `.name`, `.type`, `.status`, `.indexing_progress` |
| `resolve` | `tables[].name`, `.score`, `.match_type`, `columns[].name`, `.table`, `.confidence`, `confidence` |
| `memory` | `match_found`, `similarity`, `entry.input`, `.sql`, `.optimization_summary` |
| `history` | `results[].input`, `.similarity`, `.sql` |
| `validate` | `valid`, `normalized_sql`, `statement_type`, `read_only`, `tables[]`, `columns[]` |
| `explain` | `execution_plan.*` (cost, rows, scans, index_usage, sorts, temps, joins), `heuristics[].code`, `.severity`, `.detail` |
| `schema` | `tables[].table`, `.columns[].name`, `.type`, `.nullable`, `.primary_key`, `.generated`, `.constraints` |
| `joins` | `joins[].source`, `.target`, `.columns[][]`, `.confidence`, `.source_type` |
| `stats` | `tables[].table`, `.row_count`, `.data_length_bytes`, `.index_length_bytes`, `.last_analyzed_at` |
| `indexes` | `tables[].indexes[].name`, `.type`, `.unique`, `.primary`, `.columns[].name`, `.order`, `.sequence` |
| `save` | `saved`, `updated_existing`, `entry.id`, `.input`, `.sql_hash` |
