# Querylex Product Specification

## 1. Product Summary

Querylex is a CLI-based, AI-augmented SQL query generation and optimization system.

It helps users:

- Generate SQL from natural language using the active database schema, terminology, join graph, table statistics, indexes, and query memory.
- Optimize existing SQL using explain plans, schema context, statistics, index metadata, and dialect-aware rewrite heuristics.
- Manage multiple database connections across MySQL, MariaDB, PostgreSQL, SQLite, and Microsoft SQL Server.

The product has two layers:

- Slash skills such as `/querylex-sql`, `/querylex-optimize`, and `/querylex-stats`. These are conversational workflows that may use AI reasoning and call deterministic commands.
- CLI commands such as `querylex resolve`, `querylex schema`, and `querylex explain`. These return deterministic JSON only and are safe for automation.

## 2. Normative Companion Documents 

These companion documents are part of the specification and are required for implementation:

- `FORMATS.md` defines command response envelopes, JSON sample formats, command samples, and stable error shapes.
- `ANALYSIS.md` defines optimization heuristics, plan comparison rules, context coverage, rewrite attempts, and the AI boundary for `/querylex-optimize`.
- `domain_atlas_process.md` defines the indexing pipeline that creates `domain_map.json`, `schema/domain_map.json`, `schema/join_graph.json`, and `schema/schema_map.json`.

The runtime file `$HOME/.querylex/<db-id>/terminologies.md` is not a separate companion specification. Its format is defined in this document and in `FORMATS.md#terminologiesmd-sample-format`.

## 3. Installation And First Run

The implementation may choose the package manager and language runtime, but every supported distribution must provide the same installed contract:

- A `querylex` CLI executable available on `PATH`.
- Compatibility shortcuts `querylex-add-db` and `querylex-stats`, which may internally alias to `querylex add-db` and `querylex workspace-stats`.
- Support for Windows, macOS, and Linux.
- Access to the platform credential store for secrets.
- Optional AI provider configuration for slash-skill workflows.

On first run, Querylex creates `$HOME/.querylex/`, initializes logs, and creates `$HOME/.querylex/querylex.json` only when a database is added or an explicit initialization flow runs.

The first useful command for a new user is:

```bash
querylex-add-db
```

If a user invokes a skill before adding a database, the skill must explain that no active database exists and direct the user to run `querylex-add-db`.

## 4. Invocation Model

### Slash Skills

Slash skills are user-facing assistant workflows:

- `/querylex-sql "question"` generates SQL from natural language.
- `/querylex-optimize [--analyze] [--no-index] "SQL"` optimizes SQL.
- `/querylex-stats` summarizes Querylex workspace status.

Slash skills may call AI models. They may also present human-readable explanations and ask the user whether to proceed when cached results are found.

The old example form `/querylex resolve "question"` is a deprecated alias for `/querylex-sql "question"`. New documentation and implementations should use `/querylex-sql`.

### Deterministic Commands

Commands are shell-level or internal workflow commands. They never use AI and always return JSON using `FORMATS.md#command-response-envelope`.

Examples:

```bash
querylex resolve "orders in the last 30 days"
querylex schema --table customers --table orders
querylex explain "SELECT * FROM orders"
```

`/querylex-stats` and `querylex-stats` are the same status feature exposed through two layers:

- `/querylex-stats` is the conversational skill wrapper.
- `querylex-stats` is the deterministic JSON command.

## 5. Active Database Resolution

Every skill that needs database context starts with the same active database preflight.

1. Read `$HOME/.querylex/querylex.json`.
2. Parse JSON and validate it against `FORMATS.md#querylexjson-sample-format`.
3. Resolve `active_database_id`.
4. Load `$HOME/.querylex/<db-id>/database.json`.
5. Inspect `$HOME/.querylex/<db-id>/indexes/index_status.json` and `index_manifest.json`.

Failure handling:

| Condition | Required behavior |
|---|---|
| `querylex.json` does not exist | Stop. Tell the user no Querylex workspace exists yet and ask them to run `querylex-add-db`. |
| `querylex.json` is malformed | Stop. Report `WORKSPACE_STATE_INVALID`, point to the file, and suggest restoring from backup or re-running setup. |
| No `active_database_id` is set | Stop. Show connected databases if available and ask the user to select or add one. |
| Active database id is not listed | Stop. Report stale workspace state and suggest `querylex-stats` or reselecting a database. |
| Active database status is `not_indexed` | Stop for generation and optimization. Ask the user to complete indexing. |
| Active database status is `indexing` | Show last-known progress. Proceed only if a previous successful manifest exists and the user accepts stale context; otherwise stop. |
| Active database status is `index_failed` | Stop unless a previous successful manifest exists. If proceeding with stale artifacts, include a warning in every result. |
| Active database status is `stale` | Proceed with a warning that schema, stats, joins, and indexes may be outdated. |
| Active database status is `indexed` | Proceed normally. |

## 6. State Durability And Concurrency

Runtime state lives under `$HOME/.querylex/`, but JSON files must be treated as durable state, not casual scratch files.

Required write behavior:

- Use per-file lock files or an OS-level advisory lock before writing shared JSON state.
- Write JSON updates to a temporary file in the same directory.
- Flush the temporary file and directory metadata where the platform supports it.
- Atomically rename the temporary file over the destination.
- Include a `revision` or `updated_at` value in mutable files so readers can detect changes between reads.

Required read behavior:

- Readers must tolerate files being temporarily locked and retry with bounded backoff.
- Readers must treat malformed JSON as state corruption and return a stable error instead of crashing.
- Background indexing must update `index_status.json` atomically as last-known progress, including phase, percent, and heartbeat time.

## 7. Credentials

`database.json` stores only non-sensitive connection metadata and a credential reference.

The credential reference identifies a secret in the OS credential store:

```json
{
  "provider": "os-keychain",
  "service": "querylex",
  "account": "prod-mysql-main",
  "secret_kind": "database-password",
  "created_at": "2026-05-29T00:00:00Z"
}
```

Provider mapping:

| Platform | Preferred provider |
|---|---|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux desktop | Secret Service / libsecret |
| Linux headless | Configured secret backend or encrypted file fallback explicitly approved by the user |

Credential references must not contain passwords, tokens, private keys, or reversible secret material. If a credential cannot be retrieved, commands return `CREDENTIAL_UNAVAILABLE` or `PERMISSION_DENIED` depending on the failure.

## 8. AI Model Integration

Deterministic commands do not call AI services.

Slash skills may call an AI model for:

- Natural-language intent interpretation.
- SQL construction from resolved schema context.
- Optimization reasoning and rewrite generation.

The implementation must provide:

- A configurable provider and model.
- A way to store AI credentials outside project files, preferably in the OS keychain or environment variables.
- Prompt templates that include only the active database context needed for the task.
- Token budgeting that prefers `schema_slim.json`, resolver output, and table-scoped command results over full schema dumps.
- A deterministic fallback path when possible.
- Clear `AI_SERVICE_UNAVAILABLE` behavior when the model cannot be reached and the workflow cannot continue safely.

The boundary for optimizer AI reasoning is defined in `ANALYSIS.md#llm-boundary`.

## 9. Similarity And Memory Semantics

`querylex memory` returns a single strong match when one exists. `querylex history` returns a ranked list for broader reference.

Similarity scoring uses the same components for both:

| Component | Weight |
|---|---:|
| Embedding cosine similarity of normalized question or SQL intent | 0.45 |
| Schema-entity overlap for tables, columns, metrics, and joins | 0.25 |
| Intent classification match, such as lookup, aggregation, trend, or optimization | 0.15 |
| Filter, grouping, ordering, and time-window overlap | 0.10 |
| Recency decay | 0.05 |

Thresholds:

- `>= 0.86`: strong match. `querylex memory` may return it as a cached answer.
- `0.65` to `0.85`: related match. `querylex memory` returns `match_found: false`, while `querylex history` may include it as reference context.
- `< 0.65`: no meaningful match.

If embeddings are unavailable, Querylex may use lexical and schema-overlap scoring, but it must lower confidence and include a warning.

`querylex history` sorts by `(similarity_score * 0.8) + (recency_score * 0.2)`.

## 10. Terminology Map

`terminologies.md` is a generated, user-editable Markdown file stored per database:

```text
$HOME/.querylex/<db-id>/terminologies.md
```

`querylex-add-db` must create this file during indexing. If business terms are unknown, it creates a template with examples and an empty `terms` array.

The file is parsed as Markdown containing a fenced YAML block marked `querylex-terms`:

````markdown
# Querylex Terminologies

```querylex-terms
terms:
  - term: revenue
    type: metric
    maps_to:
      - table: orders
        column: total_amount
    description: Gross order value before refunds.
    filters: []
  - term: active customer
    type: entity_filter
    maps_to:
      - table: customers
        column: status
    values:
      - active
    description: Customer account that can place orders.
```
````

Rules:

- Freeform Markdown outside the fenced block is allowed and ignored by the parser.
- `term`, `type`, and `maps_to` are required for each entry.
- Supported `type` values are `metric`, `entity`, `entity_filter`, `dimension`, `date_window`, and `synonym`.
- Missing file: recreate a blank template when possible, warn, and continue.
- Malformed fenced block: warn with `TERMINOLOGY_PARSE_ERROR` and continue without terminology context.

This step is a file read and parse. There is no separate `querylex terminologies` command in the MVP.

## 11. Skills

### `/querylex-sql`

Generates SQL from a natural-language question.

Canonical invocation:

```text
/querylex-sql "Create a query that checks orders in the last 30 days"
```

Workflow:

1. Receive the user question.
2. Run active database preflight from Section 5 and tell the user which active database is being used.
3. Run `querylex resolve "question"` to identify candidate tables and columns.
   - On error: return the error and stop.
   - On empty result: tell the user no relevant tables were found, ask for more context, and stop.
   - On success: keep the resolved tables and columns as the table scope for later commands.
4. Run `querylex memory "question"`.
   - On error: log a warning and continue to Step 5.
   - On strong match: show the cached SQL and ask whether to use it or generate a fresh query. If the user uses the cached result, stop after returning it. If the user asks for a fresh query, continue.
   - On no strong match: run `querylex history "question"` for related examples. Use relevant results only as non-authoritative reference context.
5. Read and parse `$HOME/.querylex/<db-id>/terminologies.md`.
   - On missing file: recreate a blank template when possible, warn, and continue.
   - On parse error: warn and continue without terminology context.
6. Fetch schema details with `querylex schema --table <table>` for each resolved table.
   - On error: warn and continue. Schema detail improves accuracy but does not always block generation.
   - On success: use column definitions, data types, nullability, constraints, generated columns, and comments.
7. Fetch table statistics with `querylex stats --table <table>` for each resolved table.
   - On error: warn and continue.
   - On success: use row counts and cardinality to inform join order and filter placement in Step 10.
8. Fetch join information with `querylex joins --table <table>` for each resolved table.
   - On error: stop only if the requested SQL requires joining tables that cannot be connected. Otherwise warn and continue.
   - On success: use join paths, join types, foreign keys, and relationship confidence.
9. Fetch index information with `querylex indexes --table <table>` for each resolved table.
   - On error: warn and continue.
   - On success: use index information to choose sargable filters and efficient join predicates in Step 10.
10. Construct SQL using the resolved intent, terminology context, schema, stats, joins, indexes, and database dialect.
11. Validate the generated SQL using `querylex validate "SQL"`.
   - On validation error: regenerate or repair the SQL and retry validation.
   - Retry limit: 3 validation attempts total.
   - On success: continue.
12. Save the accepted query with `querylex save "question" "sql query"`.
   - On error: warn the user that the SQL was generated but could not be saved to memory.
   - Return the memory id when saved.
   - If the user edits the SQL after it is displayed, the skill must warn that edits are not remembered until the edited SQL is saved. It should provide the exact save command or an equivalent one-click save path.

Parallelism:

- Steps 4 through 9 may run in parallel after Step 3 succeeds.
- Step numbers define dependencies and output semantics, not a required sequential execution schedule.
- Step 10 must wait for all non-blocking context fetches to finish or time out.

### `/querylex-optimize`

Optimizes a supplied SQL query using explain plans, schema context, statistics, indexes, joins, and the analysis contract in `ANALYSIS.md`.

Canonical invocation:

```text
/querylex-optimize --analyze "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'"
```

Supported flags:

- `--analyze`: the skill passes `--analyze` through to `querylex explain` after warning that the database may execute the query depending on dialect.
- `--no-index`: the skill may rewrite SQL but must not recommend new indexes.

Workflow:

1. Receive the SQL and flags.
2. Run active database preflight from Section 5 and tell the user which active database is being used.
3. Run `querylex memory "sql query"` to check for a cached optimization.
   - On error: log a warning and continue to Step 4.
   - On matching optimized query: return the cached optimization, explain why it matched, and ask whether the user wants fresh analysis.
   - If the user declines fresh analysis: stop and keep the cached result.
   - If the user requests fresh analysis: continue to Step 4.
   - On no match: continue to Step 4.
4. Validate the original SQL with `querylex validate "SQL"`.
   - On error: return the validation error and stop.
   - On success: use normalized SQL for remaining steps.
5. Run `querylex explain "SQL"` on the original SQL.
   - If the user invoked `/querylex-optimize --analyze`, run `querylex explain "SQL" --analyze`.
   - On error: return the explain error and stop.
   - On success: normalize the plan using `ANALYSIS.md#plan-normalization`.
6. Extract all referenced tables from the validated SQL, including tables inside CTEs, subqueries, derived tables, and views when resolvable.
7. Fetch context for referenced tables:
   - `querylex schema --table <table>`
   - `querylex joins --table <table>`
   - `querylex stats --table <table>`
   - `querylex indexes --table <table>`
   - These commands may run in parallel.
   - Each failure is recorded as missing context. It does not automatically block optimization unless context coverage falls below the threshold in `ANALYSIS.md#context-coverage`.
8. Analyze the original plan using `ANALYSIS.md`.
   - Inputs include the normalized explain plan, schema, joins, stats, indexes, dialect, context coverage, and warnings.
   - If context coverage is insufficient, return the unable-to-optimize output focused on missing context.
9. Generate and verify SQL rewrites.
   - Attempt 1: predicate and projection rewrite.
   - Attempt 2: join and subquery rewrite.
   - Attempt 3: aggregation and query-shape rewrite.
   - Each attempt must use a distinct strategy.
   - Each generated SQL rewrite must pass `querylex validate "optimized SQL"`.
   - Validate-fix retry limit: 3 attempts per strategy.
   - After validation succeeds, run `querylex explain "optimized SQL"` with the same analyze mode used for the original plan.
   - Compare plans using `ANALYSIS.md#what-counts-as-better`.
10. If a rewrite is better, save it with `querylex save "original sql" "optimized sql"` and return:
    - Optimized SQL.
    - Plan comparison metrics.
    - Explanation of changes and why they helped.
    - Any warnings about stale or partial context.
11. If no rewrite is better and `--no-index` is not set, consider an index recommendation using `ANALYSIS.md#index-recommendation-rules`.
    - Recommend an index only when plan evidence shows a clear gap that rewrites cannot address.
    - Return the `CREATE INDEX` statement, target predicate/join/sort/grouping, expected plan impact, dialect notes, and a non-production testing warning.
12. If no safe improvement is found, return an unable-to-optimize result:
    - Best validated attempt, if any.
    - Attempt log for all strategies.
    - Context coverage and missing context warnings.
    - Dialect-aware next steps from `ANALYSIS.md#unable-to-optimize-output`.

### `/querylex-stats`

Shows Querylex workspace status across connected databases.

Canonical invocation:

```text
/querylex-stats
```

Relationship to command:

- The skill calls `querylex-stats`.
- The command returns deterministic JSON.
- The skill may render the same data as a human-readable summary.

Workflow:

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

## 12. Commands

All commands return deterministic JSON only. Every response follows `FORMATS.md#command-response-envelope`.

### Command Argument Conventions

Commands that accept table identifiers use this canonical form:

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
- Dialect-specific quoted identifiers, such as `"Sales Region"."Order Items"` or `[dbo].[Order Items]`, must be preserved and normalized by the dialect adapter.

### Database Setup And Status Commands

- `querylex-add-db` - Add a new database connection through a guided setup flow.
  - Asks for database type: `mysql`, `postgres`, `sqlite`, `microsoft sql`, or `mariadb`.
  - Asks for connection details required by the selected database type.
  - Stores non-sensitive connection metadata in `$HOME/.querylex/<db-id>/database.json`.
  - Stores passwords, tokens, private keys, and other secrets in the OS keychain only.
  - Verifies the connection before indexing.
  - Indexes the database and creates:
    - `$HOME/.querylex/<db-id>/schema.json`
    - `$HOME/.querylex/<db-id>/schema_slim.json`
    - `$HOME/.querylex/<db-id>/domain_map.json`
    - `$HOME/.querylex/<db-id>/terminologies.md`
    - `$HOME/.querylex/<db-id>/memory.sqlite`
    - `$HOME/.querylex/<db-id>/memory_index.json`
    - `$HOME/.querylex/<db-id>/schema/domain_map.json`
    - `$HOME/.querylex/<db-id>/schema/join_graph.json`
    - `$HOME/.querylex/<db-id>/schema/schema_map.json`
    - `$HOME/.querylex/<db-id>/indexes/index_status.json`
    - `$HOME/.querylex/<db-id>/indexes/index_manifest.json`
  - Tables are inferred into domains and join graphs using the process in ` as_process.md`.
  - Sample command and responses: `FORMATS.md#querylex-add-db-command-samples`

- `querylex-stats` - Show Querylex workspace status across connected databases.
  - Returns connected databases, active database, indexing status, indexing progress, last indexed time, schema counts, memory counts, explain cache summary, and warnings.
  - Sample command and responses: `FORMATS.md#querylex-stats-command-samples`

### Query Generation Commands

- `querylex resolve "question"` - Identify candidate tables and columns relevant to a natural-language question.
  - Returns ranked tables, candidate columns, confidence scores, matched terminology, and weak-match warnings.
  - Empty results are valid and should stop query generation until the user gives more context.
  - Sample command and responses: `FORMATS.md#querylex-resolve-command-samples`

- `querylex memory "question or sql"` - Check memory for a similar saved question, generated query, or optimized query.
  - Uses the similarity semantics in Section 9.
  - Returns the closest entry only when the match is strong enough.
  - Returns `match_found: false` when no close match exists.
  - Memory errors are warnings in skill workflows.
  - Sample command and responses: `FORMATS.md#querylex-memory-command-samples`

- `querylex history "topic"` - Search broader query history when `querylex memory` has no strong match.
  - Returns related saved entries sorted by similarity and recency.
  - Used as reference context, not as an authoritative answer.
  - Sample command and responses: `FORMATS.md#querylex-history-command-samples`

- `querylex schema --table <table>` - Fetch full column definitions for supplied tables.
  - Includes data types, nullability, defaults, generated columns, primary keys, constraints, comments, and table definitions where available.
  - Sample command and responses: `FORMATS.md#querylex-schema-command-samples`

- `querylex stats --table <table>` - Fetch row counts, cardinality, freshness, and basic distribution estimates for supplied tables.
  - Used for join order, filter placement, and optimization strategy selection.
  - Statistics may be approximate and must include freshness metadata.
  - Sample command and responses: `FORMATS.md#querylex-stats-tables-command-samples`

- `querylex joins --table <table>` - Get join paths between supplied tables using declared and inferred relationships.
  - Returns direct joins, recursive join paths, confidence, relationship source, and warnings for inferred or ambiguous joins.
  - Join path errors are blocking when the requested SQL requires joining tables that cannot be connected.
  - Sample command and responses: `FORMATS.md#querylex-joins-command-samples`

- `querylex indexes --table <table>` - Get index information for supplied tables.
  - Includes index type, uniqueness, indexed columns, column order, cardinality, visibility, and whether an index is primary, unique, composite, partial, or functional when supported.
  - Sample command and responses: `FORMATS.md#querylex-indexes-command-samples`

- `querylex validate "SQL"` - Validate SQL against the active database schema without executing data-returning or data-changing statements.
  - Parses SQL, resolves table and column references, checks dialect compatibility, rejects unsafe statements, and returns normalized SQL when valid.
  - Sample command and responses: `FORMATS.md#querylex-validate-command-samples`

- `querylex save "input" "sql query"` - Save accepted generated SQL or optimized SQL to memory.
  - For `/querylex-sql`, `input` is the user's natural-language question.
  - For `/querylex-optimize`, `input` is the original SQL.
  - Re-running save with the same normalized input overwrites the previous entry.
  - Sample command and responses: `FORMATS.md#querylex-save-command-samples`

- `querylex delete "input"` - Delete a saved memory entry.
  - Deleting a missing entry is a successful no-op with `deleted: false`.
  - Sample command and responses: `FORMATS.md#querylex-delete-command-samples`

### Query Optimization Commands

- `querylex explain "SQL"` - Run dialect-appropriate `EXPLAIN` for the supplied SQL and return the execution plan.
  - Returns JSON-formatted plans by default where supported.
  - Does not execute the SQL body beyond the database engine's normal explain behavior.
  - Used to compare original and rewritten SQL.
  - Sample command and responses: `FORMATS.md#querylex-explain-command-samples`

- `querylex explain "SQL" --analyze` - Run runtime plan analysis when safe and explicitly requested.
  - May execute the query depending on the database engine.
  - Returns actual timing and row metrics in addition to estimated plan data.
  - Sample command and responses: `FORMATS.md#querylex-explain-command-samples`

### Supported Flags

- `--analyze` - Applies to `/querylex-optimize` and direct `querylex explain`. In the skill, it is passed through to `querylex explain` after the user has explicitly requested analyze behavior.
- `--no-index` - Applies to `/querylex-optimize`. Restricts output to SQL rewrites and prevents index recommendations.
- `--verify` - Optional future flag for `querylex joins`. Verifies inferred relationships with limited read-only sampling when permitted.

## 13. Memory And Cache Consistency

### Memory Store

`memory.sqlite` is the source of truth for saved generated SQL and saved optimizations.

`memory_index.json` is a derived search index. It may include embedding metadata, keyword index state, entry counts, and compaction metadata, but it must not be the only place where saved SQL exists.

Required consistency behavior:

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

Cache key inputs:

- normalized SQL hash
- active database id
- database type and version
- explain mode (`estimated` or `analyze`)
- schema/index manifest fingerprint
- statistics freshness fingerprint when available
- relevant session settings

Invalidation triggers:

- TTL expiration.
- Index manifest change.
- Schema manifest change.
- Statistics freshness change beyond configured tolerance.
- Database version change.
- Connection role or relevant session setting change.
- Any DDL detected during re-indexing.

Default TTL guidance:

- Estimated explain: 24 hours, unless schema or stats fingerprints change sooner.
- Analyze explain: disabled by default or capped at 15 minutes because runtime load affects the result.

## 14. Runtime File Structure

This is the installed Querylex runtime structure after databases have been added and indexed. It is not the project repository structure.

```text
$HOME/.querylex/
    querylex.json
    logs/
        querylex.log
        indexing.log
    <db-id>/
        database.json
        schema.json
        schema_slim.json
        domain_map.json
        terminologies.md
        memory.sqlite
        memory_index.json
        schema/
            domain_map.json
            join_graph.json
            schema_map.json
        indexes/
            index_status.json
            index_manifest.json
        explain_cache/
            <sql-hash>.json
```

### Global Files

- `$HOME/.querylex/querylex.json`
  - Registry of connected databases.
  - Stores `active_database_id`.
  - Stores database status: `not_indexed`, `indexing`, `indexed`, `index_failed`, or `stale`.
  - Stores indexing progress, database type, display name, timestamps, and mutable revision metadata.
  - Does not store passwords or secrets.
  - Sample format: `FORMATS.md#querylexjson-sample-format`

- `$HOME/.querylex/logs/querylex.log`
  - General runtime logs for command execution, warnings, and non-sensitive errors.

- `$HOME/.querylex/logs/indexing.log`
  - Indexing job logs for schema extraction, domain inference, join graph generation, and artifact updates.

### Per-Database Files

- `$HOME/.querylex/<db-id>/database.json`
  - Non-sensitive connection metadata for one database.
  - Includes database type, host, port, database name, username, SSL mode, database version, feature support, and credential reference.
  - Sample format: `FORMATS.md#databasejson-sample-format`

- `$HOME/.querylex/<db-id>/schema.json`
  - Full indexed database schema.
  - Includes schemas, tables, columns, indexes, constraints, relationships, views, triggers, functions, enums, comments, and table definitions where available.
  - Sample format: `FORMATS.md#schemajson-sample-format`

- `$HOME/.querylex/<db-id>/schema_slim.json`
  - Compact schema optimized for fast command lookups and lower token usage.
  - Includes table names, column names, primary keys, foreign keys, indexed columns, composite indexes, and relation summaries.

- `$HOME/.querylex/<db-id>/domain_map.json`
  - Top-level domain grouping generated during indexing.
  - Groups tables into inferred business domains and subdomains.
  - Sample format: `FORMATS.md#domain_mapjson-sample-format`

- `$HOME/.querylex/<db-id>/terminologies.md`
  - User-editable business terminology map.
  - Created by `querylex-add-db`.
  - Format defined in Section 10 and `FORMATS.md#terminologiesmd-sample-format`.

- `$HOME/.querylex/<db-id>/memory.sqlite`
  - Source-of-truth memory store for saved queries and optimizations.

- `$HOME/.querylex/<db-id>/memory_index.json`
  - Derived search metadata for the memory store.
  - May be rebuilt from `memory.sqlite`.

### Per-Database Schema Directory

- `$HOME/.querylex/<db-id>/schema/domain_map.json`
  - Schema-scoped version of the domain map used by the resolver and domain atlas process.
  - Sample format: `FORMATS.md#domain_mapjson-sample-format`

- `$HOME/.querylex/<db-id>/schema/join_graph.json`
  - Relationship graph for declared and inferred joins.
  - Includes foreign keys, composite relationships, inferred naming matches, cross-domain joins, and relationship confidence.
  - Sample format: `FORMATS.md#join_graphjson-sample-format`

- `$HOME/.querylex/<db-id>/schema/schema_map.json`
  - Fast lookup map from table to domain, subdomain, primary key, inbound/outbound foreign keys, indexed columns, and bridge-table metadata.
  - Sample format: `FORMATS.md#schema_mapjson-sample-format`

### Per-Database Indexing Directory

- `$HOME/.querylex/<db-id>/indexes/index_status.json`
  - Current indexing state for this database.
  - Tracks status, phase, progress, heartbeat, timestamps, failure details, and artifact freshness.

- `$HOME/.querylex/<db-id>/indexes/index_manifest.json`
  - Manifest of generated artifacts and source database metadata.
  - Tracks schema version, database version, indexed table count, artifact checksums, and last successful indexing run.

### Example `querylex.json`

```json
{
  "connected_databases": [
    {
      "id": "prod-mysql-main",
      "name": "Production MySQL",
      "type": "mysql",
      "status": "indexed",
      "indexing_progress": 100
    },
    {
      "id": "staging-postgres",
      "name": "Staging PostgreSQL",
      "type": "postgres",
      "status": "stale",
      "indexing_progress": 100
    }
  ],
  "active_database_id": "prod-mysql-main",
  "revision": 42,
  "updated_at": "2026-05-29T00:00:00Z"
}
```
