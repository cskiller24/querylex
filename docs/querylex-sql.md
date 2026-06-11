# `/querylex-sql` — SQL Generation Skill

## 1. Overview

`/querylex-sql` generates SQL from a natural-language question. It is a conversational skill workflow that uses AI reasoning and calls deterministic CLI commands for context gathering.

**Canonical invocation:**

```text
/querylex-sql "Create a query that checks orders in the last 30 days"
```

**Companion documents:**

- `FORMATS.md` — command response envelopes, JSON sample formats, error shapes.
- `QUERYLEX.md` — product specification root.
- `domain_atlas_process.md` — indexing pipeline definitions.

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
| Active database status is `not_indexed` | Stop for generation. Ask the user to complete indexing. |
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

## 4. Terminology Map

`terminologies.md` is a generated, user-editable Markdown file stored per database:

```text
$HOME/.querylex/<db-id>/terminologies.md
```

`querylex-add-db` must create this file during indexing. If business terms are unknown, it creates a template with examples and an empty `terms` array.

### File format

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

### Rules

- Freeform Markdown outside the fenced block is allowed and ignored by the parser.
- `term`, `type`, and `maps_to` are required for each entry.
- Supported `type` values: `metric`, `entity`, `entity_filter`, `dimension`, `date_window`, `synonym`.
- Missing file: recreate a blank template when possible, warn, and continue.
- Malformed fenced block: warn with `TERMINOLOGY_PARSE_ERROR` and continue without terminology context.

This step is a file read and parse. There is no separate `querylex terminologies` command in the MVP.

---

## 5. `/querylex-sql` Workflow

### Step-by-step

1. **Receive the user question.**
2. **Run active database preflight** (see §2) and tell the user which active database is being used.
3. **Run `querylex resolve "question"`** to identify candidate tables and columns.
   - On error: return the error and stop.
   - On empty result: tell the user no relevant tables were found, ask for more context, and stop.
   - On success: keep the resolved tables and columns as the table scope for later commands.
4. **Run `querylex memory "question"`.**
   - On error: log a warning and continue to Step 5.
   - On strong match: show the cached SQL and ask whether to use it or generate a fresh query. If the user uses the cached result, stop after returning it. If the user asks for a fresh query, continue.
   - On no strong match: run `querylex history "question"` for related examples. Use relevant results only as non-authoritative reference context.
5. **Read and parse `$HOME/.querylex/<db-id>/terminologies.md`.**
   - On missing file: recreate a blank template when possible, warn, and continue.
   - On parse error: warn and continue without terminology context.
6. **Fetch schema details** with `querylex schema --table <table>` for each resolved table.
   - On error: warn and continue. Schema detail improves accuracy but does not always block generation.
   - On success: use column definitions, data types, nullability, constraints, generated columns, and comments.
7. **Fetch table statistics** with `querylex stats --table <table>` for each resolved table.
   - On error: warn and continue.
   - On success: use row counts and cardinality to inform join order and filter placement in Step 10.
8. **Fetch join information** with `querylex joins --table <table>` for each resolved table.
   - On error: stop only if the requested SQL requires joining tables that cannot be connected. Otherwise warn and continue.
   - On success: use join paths, join types, foreign keys, and relationship confidence.
9. **Fetch index information** with `querylex indexes --table <table>` for each resolved table.
   - On error: warn and continue.
   - On success: use index information to choose sargable filters and efficient join predicates in Step 10.
10. **Construct SQL** using the resolved intent, terminology context, schema, stats, joins, indexes, and database dialect.
11. **Validate the generated SQL** using `querylex validate "SQL"`.
    - On validation error: regenerate or repair the SQL and retry validation.
    - Retry limit: 3 validation attempts total.
    - On success: continue.
12. **Save the accepted query** with `querylex save "question" "sql query"`.
    - On error: warn the user that the SQL was generated but could not be saved to memory.
    - Return the memory id when saved.
    - If the user edits the SQL after it is displayed, the skill must warn that edits are not remembered until the edited SQL is saved. It should provide the exact save command or an equivalent one-click save path.

### Parallelism

- Steps 4 through 9 may run in parallel after Step 3 succeeds.
- Step numbers define dependencies and output semantics, not a required sequential execution schedule.
- Step 10 must wait for all non-blocking context fetches to finish or time out.

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

### `querylex resolve "question"`

Identify candidate tables and columns relevant to a natural-language question.

- Returns ranked tables, candidate columns, confidence scores, matched terminology, and weak-match warnings.
- Empty results are valid and should stop query generation until the user gives more context.
- Sample: `FORMATS.md#querylex-resolve-command-samples`

### `querylex memory "question or sql"`

Check memory for a similar saved question, generated query, or optimized query.

- Uses similarity semantics from §3.
- Returns the closest entry only when the match is strong enough.
- Returns `match_found: false` when no close match exists.
- Memory errors are warnings in skill workflows.
- Sample: `FORMATS.md#querylex-memory-command-samples`

### `querylex history "topic"`

Search broader query history when `querylex memory` has no strong match.

- Returns related saved entries sorted by similarity and recency.
- Used as reference context, not as an authoritative answer.
- Sample: `FORMATS.md#querylex-history-command-samples`

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

### `querylex validate "SQL"`

Validate SQL against the active database schema without executing data-returning or data-changing statements.

- Parses SQL, resolves table and column references, checks dialect compatibility, rejects unsafe statements, and returns normalized SQL when valid.
- Sample: `FORMATS.md#querylex-validate-command-samples`

### `querylex save "input" "sql query"`

Save accepted generated SQL or optimized SQL to memory.

- For `/querylex-sql`, `input` is the user's natural-language question.
- For `/querylex-optimize`, `input` is the original SQL.
- Re-running save with the same normalized input overwrites the previous entry.
- Sample: `FORMATS.md#querylex-save-command-samples`

### `querylex delete "input"`

Delete a saved memory entry.

- Deleting a missing entry is a successful no-op with `deleted: false`.
- Sample: `FORMATS.md#querylex-delete-command-samples`

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
