# `/querylex-optimize` — SQL Optimization Skill

## 1. Overview

`/querylex-optimize` optimizes a supplied SQL query using explain plans, schema context, statistics, indexes, joins, and dialect-aware rewrite heuristics.

**Canonical invocation:**

```text
/querylex-optimize --analyze "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'"
```

**Supported flags:**

| Flag | Description |
|---|---|
| `--analyze` | Passes `--analyze` through to `querylex explain` after warning that the database may execute the query depending on dialect. |
| `--no-index` | The skill may rewrite SQL but must not recommend new indexes. |

**Companion documents:**

- `ANALYSIS.md` — optimization heuristics, plan comparison rules, context coverage, rewrite attempts, AI boundary.
- `FORMATS.md` — command response envelopes, JSON sample formats, error shapes.
- `QUERYLEX.md` — product specification root.

---

## 2. Active Database Resolution

Every skill invocation starts with the same active database preflight.

1. Read `$HOME/.querylex/querylex.json`.
2. Parse JSON and validate it against `FORMATS.md#querylexjson-sample-format`.
3. Resolve `active_database_id`.
4. Load `$HOME/.querylex/<db-id>/database.json`.
5. Inspect `$HOME/.querylex/<db-id>/indexes/index_status.json` and `index_manifest.json`.

### Failure handling

| Condition | Required behavior |
|---|---|
| `querylex.json` does not exist | Stop. Tell the user no Querylex workspace exists yet and ask them to run `querylex-add-db`. |
| `querylex.json` is malformed | Stop. Report `WORKSPACE_STATE_INVALID`, point to the file, and suggest restoring from backup or re-running setup. |
| No `active_database_id` is set | Stop. Show connected databases if available and ask the user to select or add one. |
| Active database id is not listed | Stop. Report stale workspace state and suggest `querylex-stats` or reselecting a database. |
| Active database status is `not_indexed` | Stop for optimization. Ask the user to complete indexing. |
| Active database status is `indexing` | Show last-known progress. Proceed only if a previous successful manifest exists and the user accepts stale context; otherwise stop. |
| Active database status is `index_failed` | Stop unless a previous successful manifest exists. If proceeding with stale artifacts, include a warning in every result. |
| Active database status is `stale` | Proceed with a warning that schema, stats, joins, and indexes may be outdated. |
| Active database status is `indexed` | Proceed normally. |

---

## 3. Similarity And Memory Semantics

`querylex memory` returns a single strong match when one exists. `querylex history` returns a ranked list for broader reference.

### Scoring components

| Component | Weight |
|---|---:|
| Embedding cosine similarity of normalized question or SQL intent | 0.45 |
| Schema-entity overlap for tables, columns, metrics, and joins | 0.25 |
| Intent classification match (lookup, aggregation, trend, optimization) | 0.15 |
| Filter, grouping, ordering, and time-window overlap | 0.10 |
| Recency decay | 0.05 |

### Thresholds

- `>= 0.86`: strong match. `querylex memory` may return it as a cached answer.
- `0.65` to `0.85`: related match. `querylex memory` returns `match_found: false`; `querylex history` may include it as reference context.
- `< 0.65`: no meaningful match.

If embeddings are unavailable, Querylex may use lexical and schema-overlap scoring, but it must lower confidence and include a warning.

`querylex history` sorts by `(similarity_score * 0.8) + (recency_score * 0.2)`.

---

## 4. `/querylex-optimize` Workflow

### Step-by-step

1. **Receive the SQL and flags.**
2. **Run active database preflight** (see §2) and tell the user which active database is being used.
3. **Run `querylex memory "sql query"`** to check for a cached optimization.
   - On error: log a warning and continue to Step 4.
   - On matching optimized query: return the cached optimization, explain why it matched, and ask whether the user wants fresh analysis.
   - If the user declines fresh analysis: stop and keep the cached result.
   - If the user requests fresh analysis: continue to Step 4.
   - On no match: continue to Step 4.
4. **Validate the original SQL** with `querylex validate "SQL"`.
   - On error: return the validation error and stop.
   - On success: use normalized SQL for remaining steps.
5. **Run `querylex explain "SQL"`** on the original SQL.
   - If the user invoked `/querylex-optimize --analyze`, run `querylex explain "SQL" --analyze`.
   - On error: return the explain error and stop.
   - On success: normalize the plan using `ANALYSIS.md#plan-normalization`.
6. **Extract all referenced tables** from the validated SQL, including tables inside CTEs, subqueries, derived tables, and views when resolvable.
7. **Fetch context for referenced tables:**
   - `querylex schema --table <table>`
   - `querylex joins --table <table>`
   - `querylex stats --table <table>`
   - `querylex indexes --table <table>`
   - These commands may run in parallel.
   - Each failure is recorded as missing context. It does not automatically block optimization unless context coverage falls below the threshold in `ANALYSIS.md#context-coverage`.
8. **Analyze the original plan** using `ANALYSIS.md`.
   - Inputs include the normalized explain plan, schema, joins, stats, indexes, dialect, context coverage, and warnings.
   - If context coverage is insufficient, return the unable-to-optimize output focused on missing context.
9. **Generate and verify SQL rewrites.**
   - **Attempt 1:** predicate and projection rewrite.
   - **Attempt 2:** join and subquery rewrite.
   - **Attempt 3:** aggregation and query-shape rewrite.
   - Each attempt must use a distinct strategy.
   - Each generated SQL rewrite must pass `querylex validate "optimized SQL"`.
   - Validate-fix retry limit: 3 attempts per strategy.
   - After validation succeeds, run `querylex explain "optimized SQL"` with the same analyze mode used for the original plan.
   - Compare plans using `ANALYSIS.md#what-counts-as-better`.
10. **If a rewrite is better,** save it with `querylex save "original sql" "optimized sql"` and return:
    - Optimized SQL.
    - Plan comparison metrics.
    - Explanation of changes and why they helped.
    - Any warnings about stale or partial context.
11. **If no rewrite is better and `--no-index` is not set,** consider an index recommendation using `ANALYSIS.md#index-recommendation-rules`.
    - Recommend an index only when plan evidence shows a clear gap that rewrites cannot address.
    - Return the `CREATE INDEX` statement, target predicate/join/sort/grouping, expected plan impact, dialect notes, and a non-production testing warning.
12. **If no safe improvement is found,** return an unable-to-optimize result:
    - Best validated attempt, if any.
    - Attempt log for all strategies.
    - Context coverage and missing context warnings.
    - Dialect-aware next steps from `ANALYSIS.md#unable-to-optimize-output`.

---

## 5. Supporting Skill: `/querylex-stats`

### Overview

Shows Querylex workspace status across connected databases.

```text
/querylex-stats
```

### Relationship to command

- The skill calls `querylex-stats`.
- The command returns deterministic JSON.
- The skill may render the same data as a human-readable summary.

### Workflow

1. Read `$HOME/.querylex/querylex.json`.
   - Missing file: return a first-run status with no connected databases and suggest `querylex-add-db`.
   - Malformed JSON: return `WORKSPACE_STATE_INVALID` and stop.
2. List connected databases and identify the active database.
   - No active database: show connected databases and mark active database as `null`.
3. For each connected database, read:
   - `database.json`
   - `indexes/index_status.json`
   - `indexes/index_manifest.json`
   - `memory.sqlite` metadata
   - `memory_index.json`
   - `explain_cache/` summary
4. Report last-known indexing status:
   - status enum
   - progress percent
   - current phase
   - last heartbeat time
   - last successful indexed time
   - failed phase and failure details when present
5. Report health warnings:
   - stale index
   - failed index
   - missing artifacts
   - malformed state file
   - missing credential reference
   - stale memory index
   - explain cache invalidation required
6. Return the full status.

Indexing progress is last-known state written by the indexer. It is not a live stream unless a future `--watch` option is added.

---

## 6. Commands

All commands return deterministic JSON only. Every response follows `FORMATS.md#command-response-envelope`.

### Command Argument Conventions

```bash
querylex schema --table customers --table orders
querylex joins --table public.orders --table public.order_items
querylex indexes --tables-json '["Sales Region.Order Items", "dbo.Customers"]'
```

Rules:

- `--table <identifier>` may be repeated.
- `--tables-json <json-array>` is available for exact machine invocation and identifiers that are hard to quote in a shell.
- Positional shorthand such as `querylex schema customers orders` is allowed only for simple identifiers that contain no spaces or shell-sensitive quoting.
- The parser must not split identifiers on dots. `public.orders` is one identifier.
- Dialect-specific quoted identifiers must be preserved and normalized by the dialect adapter.

### `querylex memory "question or sql"`

Check memory for a similar saved question, generated query, or optimized query.

- Uses similarity semantics from §3.
- Returns the closest entry only when the match is strong enough.
- Returns `match_found: false` when no close match exists.
- Memory errors are warnings in skill workflows.
- Sample: `FORMATS.md#querylex-memory-command-samples`

### `querylex validate "SQL"`

Validate SQL against the active database schema without executing data-returning or data-changing statements.

- Parses SQL, resolves table and column references, checks dialect compatibility, rejects unsafe statements, and returns normalized SQL when valid.
- Sample: `FORMATS.md#querylex-validate-command-samples`

### `querylex explain "SQL"`

Run dialect-appropriate `EXPLAIN` for the supplied SQL and return the execution plan.

- Returns JSON-formatted plans by default where supported.
- Does not execute the SQL body beyond the database engine's normal explain behavior.
- Used to compare original and rewritten SQL.
- Sample: `FORMATS.md#querylex-explain-command-samples`

### `querylex explain "SQL" --analyze`

Run runtime plan analysis when safe and explicitly requested.

- May execute the query depending on the database engine.
- Returns actual timing and row metrics in addition to estimated plan data.
- Sample: `FORMATS.md#querylex-explain-command-samples`

### `querylex schema --table <table>`

Fetch full column definitions for supplied tables.

- Includes data types, nullability, defaults, generated columns, primary keys, constraints, comments, and table definitions where available.
- Sample: `FORMATS.md#querylex-schema-command-samples`

### `querylex stats --table <table>`

Fetch row counts, cardinality, freshness, and basic distribution estimates for supplied tables.

- Used for join order, filter placement, and optimization strategy selection.
- Statistics may be approximate and must include freshness metadata.
- Sample: `FORMATS.md#querylex-stats-tables-command-samples`

### `querylex joins --table <table>`

Get join paths between supplied tables using declared and inferred relationships.

- Returns direct joins, recursive join paths, confidence, relationship source, and warnings for inferred or ambiguous joins.
- Join path errors are blocking when the requested SQL requires joining tables that cannot be connected.
- Sample: `FORMATS.md#querylex-joins-command-samples`

### `querylex indexes --table <table>`

Get index information for supplied tables.

- Includes index type, uniqueness, indexed columns, column order, cardinality, visibility, and whether an index is primary, unique, composite, partial, or functional when supported.
- Sample: `FORMATS.md#querylex-indexes-command-samples`

### `querylex save "input" "sql query"`

Save accepted generated SQL or optimized SQL to memory.

- For `/querylex-sql`, `input` is the user's natural-language question.
- For `/querylex-optimize`, `input` is the original SQL.
- Re-running save with the same normalized input overwrites the previous entry.
- Sample: `FORMATS.md#querylex-save-command-samples`

### `querylex-stats`

Show Querylex workspace status across connected databases.

- Returns connected databases, active database, indexing status, indexing progress, last indexed time, schema counts, memory counts, explain cache summary, and warnings.
- Sample: `FORMATS.md#querylex-stats-command-samples`

---

## 7. Memory And Cache Consistency

### Memory Store

`memory.sqlite` is the source of truth for saved generated SQL and saved optimizations.

`memory_index.json` is a derived search index. It may include embedding metadata, keyword index state, entry counts, and compaction metadata, but it must not be the only place where saved SQL exists.

**Required consistency behavior:**

- `querylex save` writes to `memory.sqlite` first in a transaction.
- After the SQLite transaction commits, Querylex updates `memory_index.json` atomically with the new memory revision.
- If JSON index update fails, the command returns success with a `MEMORY_INDEX_STALE` warning because the source-of-truth write succeeded.
- `querylex memory` and `querylex history` must detect stale index revisions and either rebuild the index or fall back to SQLite search with a warning.
- Startup and `querylex-stats` should report stale or missing memory indexes and recommend repair.

### Explain Cache

Explain cache entries are optional and stored under:

```text
$HOME/.querylex/<db-id>/explain_cache/<sql-hash>.json
```

**Cache key inputs:**

- normalized SQL hash
- active database id
- database type and version
- explain mode (`estimated` or `analyze`)
- schema/index manifest fingerprint
- statistics freshness fingerprint when available
- relevant session settings

**Invalidation triggers:**

- TTL expiration.
- Index manifest change.
- Schema manifest change.
- Statistics freshness change beyond configured tolerance.
- Database version change.
- Connection role or relevant session setting change.
- Any DDL detected during re-indexing.

**Default TTL guidance:**

- Estimated explain: 24 hours, unless schema or stats fingerprints change sooner.
- Analyze explain: disabled by default or capped at 15 minutes because runtime load affects the result.
