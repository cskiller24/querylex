# QueryLex CLI Reference

All commands return a standardized JSON envelope, making the tool ideal for programmatic use.

---

## Response Envelope

Every command returns this JSON structure:

```json
{
  "success": true,
  "data": { },
  "error": null,
  "warnings": [
    { "code": "WARNING_CODE", "message": "description", "details": {} }
  ],
  "meta": {
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "protocol_version": "1.0.0",
    "active_database_id": "abc-123-def",
    "cache_hit": true,
    "duration_ms": 42
  }
}
```

On failure:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description",
    "retryable": false
  },
  "warnings": [],
  "meta": { "trace_id": "...", "protocol_version": "1.0.0", "duration_ms": 15 }
}
```

### meta

| Field | Type | Description |
|-------|------|-------------|
| `trace_id` | string | UUID v4 identifying the request |
| `protocol_version` | string | Always `"1.0.0"` |
| `active_database_id` | string or null | ID of the active database |
| `cache_hit` | boolean or null | Whether the result came from cache (explain only) |
| `duration_ms` | int64 | Wall-clock execution time in milliseconds |

### error

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Stable error code |
| `message` | string | Human-readable description |
| `retryable` | boolean | Whether retrying may succeed |

### warning

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Warning identifier |
| `message` | string | Human-readable warning |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General failure (error response) |
| 130 | Interrupted by SIGINT/SIGTERM |

---

## 1. add-db — Add a Database Connection

Interactively add a new database connection through guided prompts. When all flags are provided, prompts are skipped.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | `""` | Database type: `mysql`, `postgres`, `sqlite`, `mssql`, `mariadb` |
| `--name` | string | `""` | Display name for the connection |
| `--host` | string | `""` | Server hostname |
| `--port` | int | `0` | Server port (default: 3306 for mysql, 5432 for postgres) |
| `--database` | string | `""` | Database name or file path (SQLite) |
| `--username` | string | `""` | Authentication user |
| `--password` | string | `""` | Database password |
| `--ssl-mode` | string | `""` | SSL mode: `require`, `disable`, `verify-ca`, `verify-full` |

### Arguments

None.

### Interactive Prompts (when flags are omitted)

Database type → Display name → Host → Port → Database name → Username → Password (hidden, stored in OS keychain) → SSL mode.

### Response

`Response<AddDBData>`:

| Field | Type | Description |
|-------|------|-------------|
| `database_id` | string | UUID assigned to this database |
| `name` | string | Display name |
| `type` | string | Database engine type |
| `credential_reference` | object | Reference to stored credential (never the password itself) |
| `database_file` | string | Path to `database.json` metadata file |
| `workspace_file` | string | Path to workspace registry |
| `indexing_status` | string | `"not_indexed"`, `"indexing"`, `"indexed"`, `"index_failed"` |
| `indexing_progress` | int | Percentage 0–100 |

### Usage

```bash
# Interactive guided setup
querylex add-db

# Non-interactive (automation/CI)
querylex add-db --type postgres --name prod-db --host db.example.com \
  --port 5432 --database myapp --username app_user --password s3cret
```

### Error Cases

- `CONNECTION_FAILED` (retryable) — Database unreachable or wrong credentials
- `UNSUPPORTED_DATABASE` — Unknown database type
- `LOCK_ACQUISITION_TIMEOUT` (retryable) — Workspace lock contention

---

## 2. edit-db — Edit a Database Connection

Interactively edit an existing database connection's settings.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | id | yes | Database UUID from `list-dbs` |

### Response

`Response<EditDBData>`:

| Field | Type | Description |
|-------|------|-------------|
| `database_id` | string | Database UUID |
| `name` | string | Updated display name |
| `type` | string | Database engine type |
| `host` | string | Updated host |
| `port` | int | Updated port |
| `database` | string | Updated database name |
| `username` | string | Updated username |
| `ssl_mode` | string | Updated SSL mode |
| `password_updated` | bool | Whether a new password was stored |
| `indexing_required` | bool | Whether re-indexing is needed (true when host/port/database changed) |

### Usage

```bash
querylex edit-db a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

Prompts for all fields with current values shown as defaults.

---

## 3. delete-db — Delete a Database Connection

Remove a database connection and all associated artifacts (credentials, schema cache, memory store, indexes).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` / `-y` | bool | false | Skip confirmation prompt |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | id | no | Database UUID. If omitted, interactive prompt for selection. |

### Response

`Response<DeleteDBData>`:

| Field | Type | Description |
|-------|------|-------------|
| `deleted` | bool | Whether the database was removed |
| `database_id` | string | Deleted database UUID |
| `name` | string | Deleted database name |
| `artifacts_removed` | string[] | List of removed artifact files |

### Behavior

- Removes credential from credential store
- Deletes the per-database directory and all artifacts
- If the deleted database was active, active database is cleared

### Usage

```bash
querylex delete-db a1b2c3d4-e5f6-7890-abcd-ef1234567890   # by ID
querylex delete-db                                          # interactive selection
querylex delete-db a1b2c3d4... --force                      # skip confirmation
```

---

## 4. use-db — Switch Active Database

Set the active database for subsequent commands.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | id | no | Database UUID. If omitted, interactive prompt for selection. |

### Response

`Response<UseDBData>`:

| Field | Type | Description |
|-------|------|-------------|
| `previous_active_id` | string or null | Previously active database ID |
| `new_active_id` | string | Newly activated database ID |
| `new_active_name` | string | Display name of newly activated database |
| `new_active_type` | string | Engine type of newly activated database |

```bash
querylex use-db a1b2c3d4-e5f6-7890-abcd-ef1234567890
querylex use-db  # interactive selection
```

---

## 5. list-dbs — List Connected Databases

Show all registered database connections.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output JSON instead of human-readable table |

### Arguments

None.

### Output

**Human-readable mode** (default): formatted table with columns NAME, TYPE, HOST:PORT, DATABASE, STATUS, ACTIVE. Active database marked with `*`.

**JSON mode** (`--json`): `Response<ListDBsData>` with per-database entries:

| Field | Type | Description |
|-------|------|-------------|
| `databases[].id` | string | Database UUID |
| `databases[].name` | string | Display name |
| `databases[].type` | string | Engine type |
| `databases[].host` | string | Hostname |
| `databases[].port` | int | Port number |
| `databases[].database` | string | Database name |
| `databases[].username` | string | Connection username |
| `databases[].ssl_mode` | string | SSL mode |
| `databases[].status` | string | Indexing status |
| `databases[].is_active` | bool | Whether this is the active database |
| `count` | int | Total number of connected databases |

```bash
querylex list-dbs
querylex list-dbs --json
```

---

## 6. encrypt — Manage Encryption Keys

Manage the AES-256-GCM encryption key for the encrypted credential store (fallback when OS keychain is unavailable).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--rotate` | bool | false | Generate a fresh key and re-encrypt all credentials |
| `--force` / `-y` | bool | false | Skip confirmation prompt |
| `--json` | bool | false | Output JSON instead of human-readable message |

### Arguments

None.

### Response

JSON mode: `Response<EncryptData>` with `key_generated` (bool) and `key_rotated` (bool).

### Behavior

- Without flags: generates a new AES-256-GCM key. If existing encrypted credentials exist, they are re-encrypted.
- With `--rotate`: generates a new key and re-encrypts all credentials.
- Without `--force`: confirmation prompt before making changes.
- By default (without `--json`): human-readable success message.

### Warnings

- `ENCRYPTION_CANCELLED` — Key generation cancelled by user
- `ENCRYPTION_ROTATION_CANCELLED` — Key rotation cancelled by user

```bash
querylex encrypt
querylex encrypt --rotate
querylex encrypt --rotate --force --json
```

---

## 7. workspace-stats — Workspace Status

Show workspace overview, active database, connected database health, artifact status, and indexing state.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--human` | bool | false | Render as human-readable summary instead of JSON |

### Arguments

None.

### Response

`Response<StatsData>`:

| Field | Type | Description |
|-------|------|-------------|
| `active_database_id` | string or null | Currently active database ID |
| `connected_databases[]` | array | List of registered databases |
| `connected_databases[].id` | string | Database UUID |
| `connected_databases[].name` | string | Display name |
| `connected_databases[].type` | string | Engine type |
| `connected_databases[].status` | string | `"not_indexed"`, `"indexing"`, `"indexed"`, `"index_failed"`, `"stale"` |
| `connected_databases[].indexing_progress` | int | 0–100 |
| `health` | object | Detailed health report |
| `health.databases[].status` | string | `"healthy"`, `"degraded"`, `"unavailable"` |
| `health.databases[].artifacts` | object | Map of artifact name → `"present"`, `"stale"`, `"missing"` |
| `health.databases[].credential_status` | string | `"available"`, `"unavailable"` |
| `health.databases[].memory_index_state` | string | `"ready"`, `"stale"`, `"unavailable"` |
| `health.databases[].explain_cache_summary` | string | Summary of explain cache state |

### Status Meanings

| Status | Meaning |
|--------|---------|
| `indexed` | Fully indexed and ready |
| `stale` | Schema may have changed since indexing |
| `not_indexed` | No indexing has been performed |
| `indexing` | Indexing in progress (check `indexing_progress`) |
| `index_failed` | Previous indexing attempt failed |

### Warnings

- `NOT_INDEXED` — Database not indexed
- `STALE_ARTIFACTS` — Cached artifacts may be outdated
- `CREDENTIAL_UNAVAILABLE` — Credential cannot be retrieved

```bash
querylex workspace-stats
querylex workspace-stats --human
```

---

## 8. schema — Table Schema

Show schema information (columns, types, constraints) for one or more tables.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable: `--table t1 --table t2`) |
| `--tables-json` | string | `""` | Tables as JSON array: `--tables-json '["t1","t2"]'` |

Both flags can be combined; tables are de-duplicated.

### Arguments

None.

### Response

`Response<SchemaData>`:

| Field | Type | Description |
|-------|------|-------------|
| `tables[].table` | string | Table name |
| `tables[].schema` | string | Schema/namespace name |
| `tables[].type` | string | `"TABLE"`, `"VIEW"` |
| `tables[].comment` | string | Table comment |
| `tables[].columns[].name` | string | Column name |
| `tables[].columns[].ordinal_position` | int | Column order |
| `tables[].columns[].type` | string | Data type with length/precision |
| `tables[].columns[].nullable` | bool | Whether NULL is allowed |
| `tables[].columns[].default` | any | Default value expression |
| `tables[].columns[].primary_key` | bool | Is part of primary key |
| `tables[].columns[].generated` | bool | Is a generated/virtual column |
| `tables[].columns[].generated_expression` | string | Generation expression |
| `tables[].columns[].comment` | string | Column comment |
| `tables[].constraints[].name` | string | Constraint name |
| `tables[].constraints[].type` | string | `"PRIMARY_KEY"`, `"FOREIGN_KEY"`, `"UNIQUE"`, `"CHECK"` |
| `tables[].constraints[].columns` | string[] | Column names |
| `tables[].constraints[].referenced_table` | string | Referenced table (FK only) |
| `tables[].constraints[].referenced_columns` | string[] | Referenced columns (FK only) |
| `tables[].definition` | string | DDL definition (populated for views) |

### When to Use

Before generating SQL to understand available columns and their types. Before optimizing to check if the schema supports different query shapes.

### Usage

```bash
querylex schema --table users --table orders
querylex schema --tables-json '["users","orders"]'
```

### Error Cases

- `TABLE_NOT_FOUND` — Referenced table doesn't exist
- `CONNECTION_FAILED` (retryable) — Database unreachable

---

## 9. stats — Table Statistics

Show row counts, data/index sizes, and index cardinality for tables.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

None.

### Response

`Response<StatsTablesData>`:

| Field | Type | Description |
|-------|------|-------------|
| `tables[].table` | string | Table name |
| `tables[].row_count` | int64 | Estimated row count |
| `tables[].data_length_bytes` | int64 | Data storage size in bytes |
| `tables[].index_length_bytes` | int64 | Index storage size in bytes |
| `tables[].cardinality` | object | Map of index name → cardinality estimate |
| `tables[].last_analyzed_at` | string | Timestamp of last analysis |

### When to Use

During optimization to understand table sizes (drives join order decisions) and index selectivity.

```bash
querylex stats --table users --table orders
```

### Error Cases

- `STATS_UNAVAILABLE` (retryable) — Statistics not available

---

## 10. indexes — Index Information

Show index metadata for tables.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |
| `--live` | bool | false | Query database directly instead of cached schema_map.json |

### Arguments

None.

### Response

`Response<IndexData>`:

| Field | Type | Description |
|-------|------|-------------|
| `tables[].table` | string | Table name |
| `tables[].indexes[].name` | string | Index name |
| `tables[].indexes[].type` | string | Index type: `"btree"`, `"hash"`, `"gin"`, `"gist"` |
| `tables[].indexes[].unique` | bool | Is unique index |
| `tables[].indexes[].primary` | bool | Is primary key index |
| `tables[].indexes[].visible` | bool | Is visible to the optimizer |
| `tables[].indexes[].columns[].name` | string | Column name |
| `tables[].indexes[].columns[].order` | string | `"ASC"` or `"DESC"` |
| `tables[].indexes[].columns[].sequence` | int | Position in composite index |
| `tables[].indexes[].columns[].cardinality` | int64 | Distinct value estimate |
| `tables[].indexes[].expression` | string | Expression index definition |
| `tables[].indexes[].comment` | string | Index comment |

### Behavior

- **Fast path** (`--live=false`, default): reads from `schema_map.json` artifact. No DB connection needed.
- **Live path** (`--live=true`): queries the database directly for real-time metadata.

### When to Use

During SQL generation to write sargable predicates. During optimization to check index coverage and recommend new indexes.

```bash
querylex indexes --table users --table orders
querylex indexes --table users --live
```

---

## 11. explain — Execution Plan

Show the query execution plan with normalized metrics and heuristic-based issue detection.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--analyze` | bool | false | Execute the query for actual runtime timing and row counts (with confirmation prompt) |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | sql | yes | SQL query string to explain |

### Response

`Response<ExplainData>`:

#### execution_plan

| Field | Type | Description |
|-------|------|-------------|
| `estimated_total_cost` | float or null | Optimizer's cost estimate |
| `actual_total_time_ms` | float or null | Actual execution time (only with `--analyze`) |
| `estimated_rows_examined` | int or null | Optimizer's row estimate |
| `actual_rows_examined` | int or null | Actual rows examined (only with `--analyze`) |
| `full_scan_tables` | string[] | Tables that were full-scanned |
| `index_usage[].table` | string | Table name |
| `index_usage[].index` | string | Index name |
| `index_usage[].covering` | bool | Whether it's a covering index scan |
| `index_usage[].access_type` | string | Access method: `"index_seek"`, `"index_only"`, `"index_scan"`, `"ALL"` |
| `sort_operations` | int | Number of sort operations |
| `temp_operations` | int | Number of temp table operations |
| `join_operations[].type` | string | Join algorithm: `"Hash Join"`, `"Nested Loop"`, `"Merge Join"` |
| `join_operations[].tables` | string[] | Tables involved |
| `dialect_raw` | any | Raw plan output (DB-specific format) |

#### heuristics

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Signal code |
| `severity` | string | `"high"`, `"medium"`, `"low"` |
| `detail` | string | Human-readable explanation |

### Heuristic Signals

| Code | Severity | Description |
|------|----------|-------------|
| `FULL_TABLE_SCAN` | high | Table is full-scanned; consider adding an index |
| `CARTESIAN_JOIN` | high | Cartesian/cross join detected; add join predicates |
| `NON_SARGABLE_PREDICATE` | medium | Index touched but cannot seek (function-wrapped column) |
| `MISSING_INDEX` | medium | Full scan with no index usage |
| `EXCESSIVE_SORTING` | medium | More than 2 sort operations |
| `TEMPORARY_TABLE_USAGE` | medium | Temporary tables created |
| `INDEX_NOT_USED` | medium | Index exists but optimizer chose not to use it |
| `HIGH_COST_ESTIMATE` | medium | Estimated cost > 1000 |
| `SUBOPTIMAL_JOIN_ORDER` | medium | Nested loop join with high cost |
| `IMPLICIT_TYPE_CONVERSION` | low | Type conversion detected |
| `MULTI_TABLE_JOIN` | low | More than 4 tables joined |
| `STALE_STATISTICS` | low | Table statistics may be outdated |

### Caching

Plans are cached by fingerprint (normalized SQL). Cache entries have a TTL and are invalidated on staleness. `meta.cache_hit` indicates cache status.

### When to Use

The primary tool for query optimization. Analyze the plan to identify bottlenecks before attempting rewrites.

```bash
querylex explain "SELECT * FROM users WHERE id = 1"
querylex explain --analyze "SELECT * FROM users WHERE id = 1"
```

### Error Cases

- `EXPLAIN_FAILED` (retryable) — Plan extraction failed
- `INVALID_SQL` — SQL syntax error
- `UNSAFE_SQL` — DML/DCL statements are blocked

---

## 12. validate — Validate SQL

Validate SQL against the active database schema.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | sql | yes | SQL string to validate |

### Response

`Response<ValidateData>`:

| Field | Type | Description |
|-------|------|-------------|
| `valid` | bool | Whether the SQL is valid against the schema |
| `normalized_sql` | string | Normalized form of the SQL |
| `statement_type` | string | Type: `"SELECT"`, `"INSERT"`, etc. |
| `read_only` | bool | Whether the statement only reads data |
| `tables` | string[] | Tables referenced in the query |
| `columns` | string[] | Columns referenced in the query |

### Validation Layers

**Layer 1 — Client-side Safety Check**: Rejects DML (INSERT, UPDATE, DELETE) and DCL (GRANT, REVOKE, DROP, ALTER, TRUNCATE, CREATE) statements. Returns `UNSAFE_SQL` error.

**Layer 2 — Schema Validation**: Connects to the database and validates that all tables, columns, and joins referenced in the query exist in the schema.

### When to Use

Always validate SQL before presenting it to the user. Used by both skills after every SQL generation or rewrite.

```bash
querylex validate "SELECT * FROM users WHERE id = 1"
```

### Error Cases

- `UNSAFE_SQL` — Write operations blocked
- `TABLE_NOT_FOUND` — Referenced table doesn't exist
- `COLUMN_NOT_FOUND` — Referenced column doesn't exist
- `INVALID_SQL` — Syntax error

---

## 13. joins — Join Relationships

Show join relationships between tables from the pre-built join graph.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

None.

### Response

`Response<JoinsData>`:

| Field | Type | Description |
|-------|------|-------------|
| `joins[].source` | string | Source table |
| `joins[].target` | string | Target table |
| `joins[].columns` | array of [2]string | Column pairs: `[["source_col", "target_col"]]` |
| `joins[].confidence` | float | 0.0–1.0: `1.0` for declared FK, <1.0 for inferred |
| `joins[].source_type` | string | `"declared_foreign_key"`, `"inferred_naming_match"`, `"cross_domain"` |
| `joins[].composite` | bool | Whether the join uses multiple column pairs |
| `joins[].cross_domain` | bool | Whether the join spans logical domains |
| `path` | string[] | Join path (when exactly 2 tables specified) |
| `tables` | string[] | All tables referenced |
| `graph_loaded` | bool | Whether pre-computed join graph was loaded from disk |

### Behavior

- **Fast path** (default): loads pre-computed `join_graph.json` from disk. No DB connection needed.
- **Fallback**: queries the database live for foreign key metadata.
- When exactly 2 `--table` flags: computes the shortest join path between them.

### Warnings

- `AMBIGUOUS_JOIN` — Inferred edges with confidence < 1.0
- `NO_MATCHING_TABLES` — Tables not found in join graph

### When to Use

Before generating multi-table SQL to determine valid join paths and their confidence.

```bash
querylex joins --table users --table orders
querylex joins --tables-json '["users","orders","products"]'
```

### Error Cases

- `JOIN_PATH_NOT_FOUND` — No path between specified tables

---

## 14. save — Save to Memory

Save a natural-language-to-SQL pair to the persistent memory store for future retrieval.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language description of the query |
| 2 | sql | yes | SQL query string to save |

### Response

`Response<SaveData>`:

| Field | Type | Description |
|-------|------|-------------|
| `saved` | bool | Whether the entry was persisted |
| `updated_existing` | bool | Whether an existing entry was updated |
| `entry.id` | string | ULID of the memory entry |
| `entry.input` | string | Normalized input text |
| `entry.sql_hash` | string | SHA256 hash of the SQL |
| `entry.database_id` | string | Associated database ID |
| `entry.created_at` | string | Creation timestamp |
| `entry.updated_at` | string | Last update timestamp |

### Warnings

- `EMBEDDINGS_UNAVAILABLE` — Embedding computation failed (non-fatal)
- `MEMORY_INDEX_STALE` — Keyword index could not be rebuilt

### When to Use

After accepting a generated or optimized query to make it retrievable via `memory` later.

```bash
querylex save "find users by email" "SELECT * FROM users WHERE email = $1"
```

---

## 15. memory — Search Memory

Search for previously saved queries by natural language input. Returns the best match with similarity score.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language description to search for |

### Response

`Response<MemoryData>`:

| Field | Type | Description |
|-------|------|-------------|
| `match_found` | bool | Whether a match with similarity >= 0.86 was found |
| `similarity` | float or null | Best similarity score (0.0–1.0) |
| `match_type` | string or null | Match type (keyword, semantic) |
| `entry.id` | string | ULID |
| `entry.input` | string | Original natural language input |
| `entry.sql` | string | Saved SQL query |
| `entry.optimization_summary` | string | Optimization notes |
| `entry.database_id` | string | Associated database ID |

When no match: `match_found: false`, `similarity: null`, `entry: null`.

### When to Use

Before generating or optimizing, check if a suitable query already exists in memory (avoids redundant work).

```bash
querylex memory "find users by email"
```

---

## 16. history — Browse History

Search broader query history by topic. Returns ranked results with composite scoring.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | topic | yes | Topic keyword to search |

### Response

`Response<HistoryData>`:

| Field | Type | Description |
|-------|------|-------------|
| `topic` | string | Searched topic |
| `results[].id` | string | ULID |
| `results[].input` | string | Natural language description |
| `results[].similarity` | float | Composite score (semantic 0.8 + recency 0.2) |
| `results[].sql` | string | Saved SQL query |
| `results[].last_used_at` | string | Last access timestamp |

### Scoring

- `semantic_similarity * 0.8 + recency_score * 0.2`
- Recency decays exponentially: `exp(-days_since_last_use / 43.3)`
- Results below 0.01 are filtered out

### When to Use

When `memory` has no strong match (similarity < 0.86) but you want reference patterns.

```bash
querylex history "user queries"
```

---

## 17. delete — Delete Memory Entry

Remove a saved query from memory by its natural language input.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language input of the entry to delete |

### Response

`Response<DeleteData>`:

| Field | Type | Description |
|-------|------|-------------|
| `deleted` | bool | Whether an entry was removed |
| `entry.id` | string or null | ULID of deleted entry |
| `entry.input` | string or null | Original input |
| `entry.database_id` | string or null | Associated database ID |

### Behavior

- Deleting a non-existent entry is idempotent: returns `deleted: false` with no error
- Keyword index is automatically rebuilt after deletion

```bash
querylex delete "find users by email"
```

---

## 18. resolve — Natural Language Resolution

Resolve a natural language question to candidate tables and columns.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--tables-json` | string | `""` | Restrict resolution to specified tables |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | question | yes | Natural language question |

### Response

`Response<ResolveData>`:

| Field | Type | Description |
|-------|------|-------------|
| `tables[].name` | string | Table name |
| `tables[].score` | float | Match score 0.0–1.0 |
| `tables[].match_type` | string | `"exact"`, `"terminology"`, `"substring"`, `"fuzzy"`, `"semantic"` |
| `tables[].matched_term` | string | The term that matched |
| `columns[].name` | string | Column name |
| `columns[].table` | string | Parent table |
| `columns[].confidence` | float | Confidence 0.0–1.0 |
| `columns[].reason` | string | Explanation of why it matched |
| `confidence` | float | Overall confidence across all matches |

### Behavior

- Pure computation from local `schema_slim.json` — no database connection needed
- Multi-pass deterministic matching with 5 strategies: exact → terminology → substring → fuzzy → semantic

### When to Use

The entry point for natural-language-to-SQL generation. Identifies which tables and columns are relevant before fetching schema and joins.

```bash
querylex resolve "show me all customers who ordered last month"
querylex resolve --tables-json '["orders","customers"]' "total revenue by customer"
```

### Error Cases

- `NO_MATCHING_TABLES` — No relevant tables found

---

## 19. completion — Shell Completions

Generate shell completion script.

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | shell | yes | One of: `bash`, `zsh`, `fish`, `powershell` |

Outputs the completion script to stdout. No JSON envelope.

```bash
eval "$(querylex completion bash)"
querylex completion zsh > /usr/local/share/zsh/site-functions/_querylex
```

---

## 20. version — Version Info

Display version, commit hash, and build date.

No flags, no arguments. No JSON envelope.

```bash
querylex version
# querylex version 1.0.0 (commit abc1234, built 2026-06-08T12:00:00Z)
# Without ldflags: querylex version dev (commit unknown, built unknown)
```

---

## Error Codes

| Code | Description | Retryable |
|------|-------------|-----------|
| `INVALID_ARGUMENT` | Invalid command arguments | no |
| `CONNECTION_FAILED` | Database connection failed | yes |
| `CREDENTIAL_UNAVAILABLE` | Credential could not be retrieved | yes |
| `WORKSPACE_STATE_INVALID` | Missing, malformed, or inconsistent workspace | no |
| `LOCK_ACQUISITION_TIMEOUT` | File lock could not be acquired within timeout | yes |
| `UNSUPPORTED_DATABASE` | Database type is not supported | no |
| `PERMISSION_DENIED` | Active credentials lack required permissions | no |
| `INTERNAL_ERROR` | Unexpected internal error | no |
| `INVALID_SQL` | SQL could not be validated against schema | no |
| `UNSAFE_SQL` | DML/DCL statements not permitted | no |
| `TABLE_NOT_FOUND` | Referenced table does not exist | no |
| `COLUMN_NOT_FOUND` | Referenced column does not exist | no |
| `NO_MATCHING_TABLES` | No matching tables found for input | no |
| `EXPLAIN_FAILED` | Execution plan extraction failed | yes |
| `JOIN_PATH_NOT_FOUND` | No join path exists between specified tables | no |
| `STATS_UNAVAILABLE` | Table statistics are unavailable | yes |
| `INDEX_ANALYSIS_FAILED` | Index metadata extraction failed | yes |
| `SCHEMA_PARSE_ERROR` | Schema data could not be parsed | no |
| `TERMINOLOGY_PARSE_ERROR` | terminologies.md contains malformed YAML | no |
| `MEMORY_INDEX_STALE` | Memory index metadata is stale | yes |
| `MEMORY_STORE_UNAVAILABLE` | Memory subsystem is unavailable | yes |
| `MEMORY_WRITE_FAILED` | Unable to write memory entry | yes |
| `EXPLAIN_CACHE_STALE` | Explain cache entry stale | yes |

---

## Preflight Requirements by Command

| Command | DB Connection Required? | Indexed Schema Required? |
|---------|------------------------|--------------------------|
| `add-db` | No | No |
| `edit-db` | No | No |
| `delete-db` | No | No |
| `use-db` | No | No |
| `list-dbs` | No | No |
| `encrypt` | No | No |
| `workspace-stats` | No | No |
| `schema` | Yes | Yes |
| `stats` | Yes | Yes |
| `indexes` (default) | No | Yes |
| `indexes --live` | Yes | Yes |
| `explain` | Yes | Yes |
| `validate` | Yes | Yes |
| `joins` (default) | No | Yes |
| `joins` (fallback) | Yes | Yes |
| `resolve` | No | Yes |
| `save` | No | Yes |
| `memory` | No | Yes |
| `history` | No | Yes |
| `delete` | No | Yes |
| `completion` | No | No |
