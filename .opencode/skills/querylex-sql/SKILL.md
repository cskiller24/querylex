---
name: querylex-sql
description: Generate SQL queries from natural language using the QueryLex CLI toolchain. Use this skill whenever the user asks a natural-language question about their database, wants to query data in human terms (e.g. "show me all employees hired last year with their salary"), asks to "write a SQL query for...", or asks questions that imply database lookups like "what's the average salary by department?". This skill introspects the active database schema, checks memory for cached queries, fetches table statistics, join paths, and indexes, then constructs and validates dialect-appropriate SQL.
---

# querylex-sql: Natural Language to SQL

Generate dialect-correct SQL queries from natural language questions by orchestrating the QueryLex CLI toolchain. The process introspects the active database, resolves the user's intent to candidate tables and columns, checks for cached results, gathers schema/statistics/join/index context, constructs SQL, validates it, and saves accepted queries to memory.

## Prerequisites

The `querylex` binary must be on `$PATH`. Verify with `which querylex`. The working database must be indexed (one-time `querylex add-db` setup). If the binary is not found, build it from the project root: `go build -o /usr/local/bin/querylex ./cmd/querylex`.

## The 11-Step Workflow

Execute these steps in order, one at a time. Use `bash` to run `querylex` subcommands. Always parse JSON responses from querylex — every command returns a JSON envelope with `success`, `data`, `error`, `warnings`, and `meta` fields. Check `success` first; if false, read `error.code` and `error.message`.

### Step 1: Receive the User Question

Capture the user's natural language question exactly as they phrased it. This is the `question` string used in subsequent commands.

### Step 2: Active Database Preflight

Run `querylex workspace-stats` to get the status of all connected databases, the active database ID, and each database's indexing state.

Parse the JSON response:
- On error: stop and report the error.
- On success: identify the active database from the response. The response includes `active_database_id` and per-database entries with `status` (indexed, stale, indexing, not_indexed, index_failed).

Handle each status per the table below:

| Status | Action |
|--------|--------|
| `indexed` | Proceed normally. |
| `stale` | Proceed but include a warning that artifacts may be outdated. |
| `indexing` | Show last-known progress. Proceed only if a previous successful manifest exists and inform the user context may be incomplete. |
| `not_indexed` | Stop. Tell the user to complete indexing first. |
| `index_failed` | Stop unless a previous manifest exists. If proceeding with stale artifacts, include a warning. |
| `querylex.json` missing or malformed | Stop. Report workspace state invalid and suggest re-running setup. |

Extract the database type (mysql, postgresql, etc.) from the workspace-stats response — this determines the SQL dialect for step 9. |

### Step 3: Resolve Intent

Run `querylex resolve "<question>"` to identify which tables and columns are relevant to the question.

Parse the JSON response:
- On error: return the error and stop.
- On empty result (tables is empty array or `NO_MATCHING_TABLES` warning): tell the user no relevant tables were found and ask for more context. Stop.
- On success: extract the resolved table names from `data.tables[].table`. These are the `table scope` for all subsequent commands.

Store two variables:
- `TABLES` — the list of resolved table names (e.g., `["employees", "salaries", "dept_emp", "departments"]`)
- `TABLES_JSON` — the same list as a JSON array string (e.g., `'["employees","salaries","dept_emp","departments"]'`) for use with `--tables-json`

### Step 4: Check Memory

Run `querylex memory "<question>"` to search for previously cached similar queries.

Parse the JSON response:
- On error: log a warning and continue to step 5.
- On strong match (`data.match_found: true`, similarity >= 0.86): show the cached SQL and ask the user whether to use it or generate fresh. If they accept the cached result, skip to the end (don't save — it's already in memory). If they want a fresh query, continue.
- On no strong match (`data.match_found: false`): run `querylex history "<question>"` to get related examples. Note any useful reference patterns but do not treat history results as authoritative.

### Step 5: Read Terminology Map

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

- Missing file: warn and continue. If possible, create a blank template.
- Parse error: warn with `TERMINOLOGY_PARSE_ERROR` and continue.

Map any business terms found to their corresponding tables and columns. Use these mappings to inform SQL construction in step 9.

### Steps 6-8: Gather Context

Run these three commands one at a time, in any order. Each command targets the resolved tables from step 3.

```
# Step 6: Schema for each resolved table
querylex schema --table employees --table salaries --table dept_emp --table departments

# Step 7: Join paths between resolved tables
querylex joins --table employees --table salaries --table dept_emp --table departments

# Step 8: Index information for each resolved table
querylex indexes --table employees --table salaries --table dept_emp --table departments
```

Error handling for each:
- Schema (step 6): on error, warn and continue (schema improves accuracy but doesn't always block generation)
- Joins (step 7): on error, stop only if the question requires joining tables that cannot be connected. Otherwise warn and continue.
- Indexes (step 8): on error, warn and continue (index info helps choose sargable filters)

Once all commands have completed, proceed to step 9.

### Step 9: Construct SQL

Now that you have all context, construct a dialect-appropriate SQL query.

**Dialect awareness:** The database type from step 2 determines the SQL dialect. Use MySQL syntax for mysql, PostgreSQL for postgresql, T-SQL for mssql, SQLite for sqlite. Key differences include:
- Identifier quoting: `backticks` (MySQL) vs `"double quotes"` (PostgreSQL) vs `[brackets]` (MSSQL) vs none (SQLite)
- LIMIT syntax: `LIMIT N OFFSET M` (MySQL/PostgreSQL/SQLite) vs `OFFSET M ROWS FETCH NEXT N ROWS ONLY` (MSSQL)
- Date functions: `DATE_SUB(NOW(), INTERVAL 5 YEAR)` (MySQL) vs `NOW() - INTERVAL '5 years'` (PostgreSQL)
- String concatenation: `CONCAT()` (MySQL) vs `||` (PostgreSQL/MSSQL/SQLite)

**How to use each context type:**

- **Schema (step 6):** Use column names, data types, nullability, and constraints. This is your primary source of truth for column selection. Pay attention to `generated` columns (don't filter on them directly) and which columns are nullable (for LEFT JOIN vs INNER JOIN decisions).
- **Joins (step 7):** Use the join graph to connect tables. Prefer high-confidence edges (confidence=1.0, foreign keys) over inferred edges. If no join path exists between required tables, flag this to the user.
- **Indexes (step 8):** Write filters that can use available indexes (sargable predicates). For composite indexes, match the leading column order. Avoid wrapping indexed columns in functions (e.g., use `hire_date >= '2020-01-01'` not `YEAR(hire_date) >= 2020`).
- **Terminology (step 5):** Map business terms from the user's question to actual table/column names. If the user says "current salary", use the terminology map and the `from_date`/`to_date` range columns to pick the latest entry.

**Example:**
```
Input: "show me all employees hired in the last 5 years with their current salary and department"
Tables resolved: employees, salaries, dept_emp, departments
Dialect: MySQL
```

For date-range tables like `salaries` and `dept_emp` that use `from_date`/`to_date`, always include `to_date = '9999-01-01'` or equivalent to get the current row for temporal tables.

### Step 10: Validate

Run `querylex validate "<SQL>"` to validate the generated SQL against the database schema.

Parse the JSON response:
- On success (`data.valid: true`): proceed to step 11.
- On failure: analyze the error. Common failures and fixes:
  - `TABLE_NOT_FOUND`: the table name is wrong or schema-qualified incorrectly
  - `COLUMN_NOT_FOUND`: a column name doesn't exist — recheck the schema from step 6
  - `INVALID_SQL`: syntax error — fix the SQL syntax
  - `UNSAFE_SQL`: the SQL contains DML/DCL keywords — rewrite as SELECT-only
- Retry up to 3 times total (initial attempt + 2 retries). Each retry should produce a different fix — don't just re-submit the same broken SQL.
- After 3 failures, tell the user which errors occurred and show the best attempt so far.

### Step 11: Save and Update Terminology

On successful validation, save the query to memory:

```bash
querylex save "<question>" "<SQL>"
```

Parse the response:
- On success: the query is now in memory for future retrieval via `querylex memory`.
- On error: warn that the SQL was generated but could not be saved. Show the SQL and suggest the user save manually later.

After saving, update the terminology map with any business term mappings discovered during this workflow. Read `$HOME/.querylex/<db-id>/terminologies.md`, add or update entries for terms found in the user's question that mapped to specific tables and columns (e.g., "current salary" → salaries table with to_date filter), and write the updated file. This keeps the terminology map current so future queries benefit from the mappings you just used.

### Final Output

Present the result to the user as:

1. The generated SQL (in a code block with the appropriate dialect language tag)
2. A brief explanation of key join paths and filters used (this helps the user verify correctness)

Keep the output concise — the user wants the query, not diagnostic noise. Do not include low-value system warnings (like `EMBEDDINGS_UNAVAILABLE`) in the final output. Only mention something if it materially affects correctness, such as a stale index or missing statistics that forced a fallback decision.

## Command Reference

All commands return JSON envelopes. Use `jq` for parsing when helpful, or parse JSON directly in logic.

```bash
# Check workspace status (active database, indexing state)
querylex workspace-stats

# Resolve natural language to tables/columns
querylex resolve "the user's question"

# Check memory for cached similar queries
querylex memory "the user's question"

# Search broader history (when memory has no strong match)
querylex history "the user's question"

# Fetch schema (repeat --table for each table)
querylex schema --table table1 --table table2

# Fetch join paths
querylex joins --table table1 --table table2

# Fetch index information
querylex indexes --table table1 --table table2

# Validate SQL against active database
querylex validate "SELECT ..."

# Save accepted query to memory
querylex save "the question" "the SQL"
```

### Using --tables-json for Efficiency

When you have many tables or table names that contain spaces or special characters, use `--tables-json` instead of repeated `--table`:

```bash
querylex schema --tables-json '["employees","salaries","dept_emp","departments"]'
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
- **resolve**: `data.tables[]` (with `.table`, `.schema`, `.confidence`), `data.columns[]`, `data.confidence`
- **memory**: `data.match_found`, `data.entry` (with `.id`, `.input`, `.sql`)
- **history**: `data.results[]` (with `.id`, `.input`, `.similarity`, `.sql`)
- **schema**: `data.tables[]` (with `.columns[]`, `.constraints[]`, `.type`)
- **joins**: `data.joins[]` (with `.source`, `.target`, `.confidence`, `.source_type`, `.join_type`)
- **indexes**: `data.tables[]` (with `.indexes[]`, each having `.name`, `.type`, `.unique`, `.primary`, `.columns[]`)
- **validate**: `data.valid`, `data.normalized_sql`, `data.tables[]`, `data.columns[]`
- **save**: `data.saved`, `data.entry` (with `.id`, `.input`, `.sql_hash`)
