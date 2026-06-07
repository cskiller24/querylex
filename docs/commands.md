# QueryLex Command Reference

All 14 CLI commands documented with flags, arguments, and JSON response schemas.

## Response Envelope

Every command returns a standardized JSON envelope:

```json
{
  "success": true,
  "data": { /* command-specific payload */ },
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
    "message": "Human-readable error description",
    "retryable": false
  },
  "warnings": [],
  "meta": {
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "protocol_version": "1.0.0",
    "duration_ms": 15
  }
}
```

## Response Types

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
| `code` | string | Stable error code (see [Error Codes](#error-codes)) |
| `message` | string | Human-readable description |
| `retryable` | boolean | Whether retrying may succeed |
| `details` | any | Additional error context (optional) |

### warning

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Warning identifier |
| `message` | string | Human-readable warning |
| `details` | any | Additional context (optional) |

---

## 1. `querylex add-db`

Interactively add a new database connection through guided prompts.

### Usage

```bash
querylex add-db
```

### Flags

None.

### Arguments

None.

### Interactive Prompts

| Prompt | Description |
|--------|-------------|
| Database type | One of: MySQL/MariaDB, PostgreSQL, SQLite, Microsoft SQL Server |
| Display name | Human-readable label for the database |
| Host | Server hostname or IP |
| Port | Server port |
| Database name | Database or file path (SQLite) |
| Username | Authentication user |
| Password | Hidden input, stored in OS keychain |
| SSL mode | `disable`, `prefer`, `require`, `verify-ca`, `verify-full` |

### Response Type

`Response<AddDBData>`

```json
{
  "data": {
    "database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "name": "my-database",
    "type": "mysql",
    "credential_reference": {
      "provider": "keychain",
      "service": "querylex",
      "account": "my-database:app_user",
      "secret_kind": "database_password",
      "created_at": "2026-06-08T12:00:00Z"
    },
    "database_file": "/home/user/.querylex/a1b2c3d4/database.json",
    "workspace_file": "/home/user/.querylex/querylex.json",
    "indexing_status": "indexed",
    "indexing_progress": 100
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `database_id` | string | UUID assigned to this database |
| `name` | string | Display name |
| `type` | string | Database engine type |
| `credential_reference` | object | Reference to stored credential (never the password itself) |
| `database_file` | string | Path to database.json metadata file |
| `workspace_file` | string | Path to workspace registry |
| `indexing_status` | string | `"not_indexed"`, `"indexing"`, `"indexed"`, `"index_failed"` |
| `indexing_progress` | int | Percentage 0–100 |

---

## 2. `querylex workspace-stats`

Show workspace status across connected databases.

### Usage

```bash
querylex workspace-stats [--human]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--human` | bool | `false` | Render as human-readable summary instead of JSON |

### Arguments

None.

### Response Type (JSON mode)

`Response<StatsData>`

```json
{
  "data": {
    "active_database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "connected_databases": [
      {
        "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
        "name": "my-database",
        "type": "mysql",
        "status": "indexed",
        "indexing_progress": 100
      }
    ],
    "health": {
      "databases": [
        {
          "database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
          "database_name": "my-database",
          "status": "healthy",
          "indexing_progress": 100,
          "artifacts": {
            "schema_map": "present",
            "join_graph": "present",
            "schema_slim": "present",
            "index_status": "present"
          },
          "credential_status": "available",
          "memory_index_state": "ready",
          "explain_cache_summary": "12 entries"
        }
      ]
    }
  }
}
```

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
| `health.databases[]` | array | Per-database health |
| `health.databases[].status` | string | `"healthy"`, `"degraded"`, `"unavailable"` |
| `health.databases[].artifacts` | object | Map of artifact name → status (`"present"`, `"stale"`, `"missing"`) |
| `health.databases[].credential_status` | string | `"available"`, `"unavailable"` |
| `health.databases[].memory_index_state` | string | `"ready"`, `"stale"`, `"unavailable"` |
| `health.databases[].explain_cache_summary` | string | Summary of explain cache state |

### Warnings

- Non-indexed databases (`NOT_INDEXED`)
- Stale artifacts (`STALE_ARTIFACTS`)
- Unavailable credentials (`CREDENTIAL_UNAVAILABLE`)

---

## 3. `querylex schema`

Show schema information for tables.

### Usage

```bash
querylex schema [--table <name>]... [--tables-json <json>]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable: `--table t1 --table t2`) |
| `--tables-json` | string | `""` | Tables as JSON array: `--tables-json '["t1","t2"]'` |

Both flags can be combined; tables are de-duplicated.

### Arguments

None.

### Response Type

`Response<SchemaData>`

```json
{
  "data": {
    "tables": [
      {
        "table": "users",
        "schema": "public",
        "type": "TABLE",
        "comment": "User accounts",
        "columns": [
          {
            "name": "id",
            "ordinal_position": 1,
            "type": "integer",
            "nullable": false,
            "default": null,
            "primary_key": true,
            "generated": false,
            "generated_expression": "",
            "comment": "Primary key"
          },
          {
            "name": "email",
            "ordinal_position": 2,
            "type": "character varying(255)",
            "nullable": false,
            "default": null,
            "primary_key": false,
            "generated": false,
            "generated_expression": "",
            "comment": ""
          },
          {
            "name": "full_name",
            "ordinal_position": 3,
            "type": "character varying(100)",
            "nullable": true,
            "default": null,
            "primary_key": false,
            "generated": false,
            "generated_expression": "",
            "comment": ""
          },
          {
            "name": "created_at",
            "ordinal_position": 4,
            "type": "timestamp without time zone",
            "nullable": false,
            "default": "now()",
            "primary_key": false,
            "generated": false,
            "generated_expression": "",
            "comment": ""
          }
        ],
        "constraints": [
          {
            "name": "users_pkey",
            "type": "PRIMARY_KEY",
            "columns": ["id"],
            "referenced_table": "",
            "referenced_columns": null
          }
        ],
        "definition": ""
      },
      {
        "table": "orders",
        "schema": "public",
        "type": "TABLE",
        "comment": "Customer orders",
        "columns": [...],
        "constraints": [...],
        "definition": ""
      }
    ],
    "schema": null
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `tables[]` | array | Requested or all tables |
| `tables[].table` | string | Table name |
| `tables[].schema` | string | Schema/namespace name |
| `tables[].type` | string | `"TABLE"`, `"VIEW"` |
| `tables[].comment` | string | Table comment |
| `tables[].columns[]` | array | Column definitions |
| `tables[].columns[].name` | string | Column name |
| `tables[].columns[].ordinal_position` | int | Column order |
| `tables[].columns[].type` | string | Data type with length/precision |
| `tables[].columns[].nullable` | bool | Whether NULL is allowed |
| `tables[].columns[].default` | any | Default value expression |
| `tables[].columns[].primary_key` | bool | Is part of primary key |
| `tables[].columns[].generated` | bool | Is a generated/virtual column |
| `tables[].columns[].generated_expression` | string | Generation expression |
| `tables[].columns[].comment` | string | Column comment |
| `tables[].constraints[]` | array | Table constraints |
| `tables[].constraints[].name` | string | Constraint name |
| `tables[].constraints[].type` | string | `"PRIMARY_KEY"`, `"FOREIGN_KEY"`, `"UNIQUE"`, `"CHECK"` |
| `tables[].constraints[].columns` | array | Column names |
| `tables[].constraints[].referenced_table` | string | Referenced table (FK only) |
| `tables[].constraints[].referenced_columns` | array | Referenced columns (FK only) |
| `tables[].definition` | string | DDL definition (populated for views) |
| `schema` | object or null | Raw schema result from adapter (DB-specific details) |

---

## 4. `querylex stats`

Show table statistics.

### Usage

```bash
querylex stats [--table <name>]... [--tables-json <json>]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

None.

### Response Type

`Response<StatsTablesData>`

```json
{
  "data": {
    "tables": [
      {
        "table": "users",
        "row_count": 15234,
        "data_length_bytes": 4194304,
        "index_length_bytes": 2097152,
        "cardinality": {
          "users_pkey": 15234,
          "idx_users_email": 15234
        },
        "last_analyzed_at": "2026-06-07T10:00:00Z"
      }
    ]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `tables[]` | array | Stat entries |
| `tables[].table` | string | Table name |
| `tables[].row_count` | int64 | Estimated row count |
| `tables[].data_length_bytes` | int64 | Data storage size in bytes |
| `tables[].index_length_bytes` | int64 | Index storage size in bytes |
| `tables[].cardinality` | object | Map of index name → cardinality estimate |
| `tables[].last_analyzed_at` | string | Timestamp of last analysis |

---

## 5. `querylex indexes`

Show index information for tables.

### Usage

```bash
querylex indexes [--table <name>]... [--tables-json <json>] [--live]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |
| `--live` | bool | `false` | Query database live instead of reading from `schema_map.json` |

### Arguments

None.

### Response Type

`Response<IndexData>`

```json
{
  "data": {
    "tables": [
      {
        "table": "users",
        "indexes": [
          {
            "name": "users_pkey",
            "type": "btree",
            "unique": true,
            "primary": true,
            "visible": true,
            "columns": [
              {
                "name": "id",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 15234
              }
            ],
            "expression": "",
            "comment": ""
          },
          {
            "name": "idx_users_email",
            "type": "btree",
            "unique": true,
            "primary": false,
            "visible": true,
            "columns": [
              {
                "name": "email",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 15234
              }
            ],
            "expression": "",
            "comment": ""
          }
        ]
      }
    ]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `tables[]` | array | Index entries |
| `tables[].table` | string | Table name |
| `tables[].indexes[]` | array | Index definitions |
| `tables[].indexes[].name` | string | Index name |
| `tables[].indexes[].type` | string | Index type: `"btree"`, `"hash"`, `"gin"`, `"gist"`, etc. |
| `tables[].indexes[].unique` | bool | Is unique index |
| `tables[].indexes[].primary` | bool | Is primary key index |
| `tables[].indexes[].visible` | bool | Is visible to the optimizer |
| `tables[].indexes[].columns[]` | array | Indexed columns |
| `tables[].indexes[].columns[].name` | string | Column name |
| `tables[].indexes[].columns[].order` | string | `"ASC"` or `"DESC"` |
| `tables[].indexes[].columns[].sequence` | int | Position in composite index |
| `tables[].indexes[].columns[].cardinality` | int64 | Distinct value estimate |
| `tables[].indexes[].expression` | string | Expression index definition |
| `tables[].indexes[].comment` | string | Index comment |

### Behavior

- **Fast path** (`--live=false`, default): Reads from `schema_map.json` artifact. No database connection needed.
- **Live path** (`--live=true`): Queries the database directly for real-time index metadata.

---

## 6. `querylex explain`

Show execution plan for a SQL query.

### Usage

```bash
querylex explain [--analyze] <sql>
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--analyze` | bool | `false` | Execute the query for actual runtime timing and row counts (with confirmation prompt) |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | sql | yes | SQL query string to explain |

### Response Type

`Response<ExplainData>`

```json
{
  "data": {
    "execution_plan": {
      "estimated_total_cost": 124.53,
      "actual_total_time_ms": 45.2,
      "estimated_rows_examined": 15234,
      "actual_rows_examined": 15234,
      "full_scan_tables": [],
      "index_usage": [
        {
          "table": "orders",
          "index": "idx_orders_user_id",
          "covering": false,
          "access_type": "index_seek"
        }
      ],
      "sort_operations": 1,
      "temp_operations": 0,
      "join_operations": [
        {
          "type": "Hash Join",
          "tables": ["users", "orders"]
        }
      ],
      "warnings": [],
      "dialect_raw": {}
    },
    "heuristics": [
      {
        "code": "HIGH_COST_ESTIMATE",
        "severity": "medium",
        "detail": "Query has high estimated cost (124.53). Consider optimization."
      }
    ],
    "analyze": false
  }
}
```

#### execution_plan

| Field | Type | Description |
|-------|------|-------------|
| `estimated_total_cost` | float or null | Optimizer's cost estimate |
| `actual_total_time_ms` | float or null | Actual execution time (only with `--analyze`) |
| `estimated_rows_examined` | int or null | Optimizer's row estimate |
| `actual_rows_examined` | int or null | Actual rows examined (only with `--analyze`) |
| `full_scan_tables` | string[] | Tables that were full-scanned |
| `index_usage` | object[] | Index usage statistics |
| `index_usage[].table` | string | Table name |
| `index_usage[].index` | string | Index name |
| `index_usage[].covering` | bool | Whether it's a covering index scan |
| `index_usage[].access_type` | string | Access method: `"index_seek"`, `"index_only"`, `"index_scan"`, `"ALL"` |
| `sort_operations` | int | Number of sort operations |
| `temp_operations` | int | Number of temp table operations |
| `join_operations` | object[] | Join operations |
| `join_operations[].type` | string | Join algorithm: `"Hash Join"`, `"Nested Loop"`, `"Merge Join"`, etc. |
| `join_operations[].tables` | string[] | Tables involved |
| `warnings` | string[] | Database-specific warnings |
| `dialect_raw` | any | Raw plan output (DB-specific format) |

#### heuristics

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Signal code (see [Heuristic Signals](#heuristic-signals)) |
| `severity` | string | `"high"`, `"medium"`, `"low"` |
| `detail` | string | Human-readable explanation |

### Heuristic Signals

| Code | Severity | Description |
|------|----------|-------------|
| `FULL_TABLE_SCAN` | high | Table is full-scanned; consider adding an index |
| `CARTESIAN_JOIN` | high | Cartesian/cross join detected; add join predicates |
| `NON_SARGABLE_PREDICATE` | medium | Index touched but cannot seek (function-wrapped column, etc.) |
| `MISSING_INDEX` | medium | Full scan with no index usage; consider an index |
| `EXCESSIVE_SORTING` | medium | More than 2 sort operations; consider indexing ORDER BY columns |
| `TEMPORARY_TABLE_USAGE` | medium | Temporary tables created; check GROUP BY / DISTINCT on indexed columns |
| `INDEX_NOT_USED` | medium | Index exists but optimizer chose not to use it |
| `HIGH_COST_ESTIMATE` | medium | Estimated cost > 1000 |
| `SUBOPTIMAL_JOIN_ORDER` | medium | Nested loop join with high cost; consider rewriting join order |
| `IMPLICIT_TYPE_CONVERSION` | low | Type conversion detected; may cause performance degradation |
| `MULTI_TABLE_JOIN` | low | More than 4 tables joined; consider limiting join count |
| `STALE_STATISTICS` | low | Table statistics may be outdated |

### Caching

Explain plans are cached by fingerprint (normalized SQL). Cache entries have a TTL and are invalidated on staleness. The `meta.cache_hit` field indicates cache status.

---

## 7. `querylex validate`

Validate SQL against the active database schema.

### Usage

```bash
querylex validate <sql>
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | sql | yes | SQL string to validate |

### Response Type

`Response<ValidateData>`

```json
{
  "data": {
    "valid": true,
    "normalized_sql": "SELECT * FROM users WHERE id = $1",
    "statement_type": "SELECT",
    "read_only": true,
    "tables": ["users"],
    "columns": ["id", "email", "full_name", "created_at"]
  }
}
```

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

---

## 8. `querylex joins`

Show join relationships for tables.

### Usage

```bash
querylex joins [--table <name>]... [--tables-json <json>]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--table` | stringArray | `[]` | Table names (repeatable) |
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

None.

### Response Type

`Response<JoinsData>`

```json
{
  "data": {
    "joins": [
      {
        "source": "orders",
        "target": "users",
        "columns": [["user_id", "id"]],
        "confidence": 1.0,
        "source_type": "declared_foreign_key",
        "composite": false,
        "cross_domain": false
      },
      {
        "source": "orders",
        "target": "products",
        "columns": [["product_id", "id"]],
        "confidence": 0.85,
        "source_type": "inferred_naming_match",
        "composite": false,
        "cross_domain": false
      }
    ],
    "path": [],
    "tables": ["orders", "users", "products"],
    "graph_loaded": true
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `joins[]` | array | Join edges |
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

- **Fast path** (default): Loads pre-computed `join_graph.json` from disk
- **Fallback**: Queries the database live for foreign key metadata
- When exactly 2 tables are specified via `--table`, computes the shortest join path between them

### Warnings

- `AMBIGUOUS_JOIN` — Inferred edges with confidence < 1.0
- `NO_MATCHING_TABLES` — Tables not found in join graph

---

## 9. `querylex save`

Save a query to memory.

### Usage

```bash
querylex save <input> <sql>
```

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language description of the query |
| 2 | sql | yes | SQL query string to save |

### Response Type

`Response<SaveData>`

```json
{
  "data": {
    "saved": true,
    "updated_existing": false,
    "entry": {
      "id": "01JQZRYV4K0000000000000000",
      "input": "find users by email",
      "sql_hash": "a1b2c3d4e5f6...",
      "database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "created_at": "2026-06-08T12:00:00Z",
      "updated_at": "2026-06-08T12:00:00Z"
    }
  }
}
```

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

---

## 10. `querylex memory`

Search memory for matching queries.

### Usage

```bash
querylex memory <input>
```

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language description to search for |

### Response Type

`Response<MemoryData>`

```json
{
  "data": {
    "match_found": true,
    "similarity": 0.92,
    "match_type": "semantic",
    "entry": {
      "id": "01JQZRYV4K0000000000000000",
      "input": "find users by email",
      "sql": "SELECT * FROM users WHERE email = $1",
      "optimization_summary": "",
      "created_at": "2026-06-08T12:00:00Z",
      "updated_at": "2026-06-08T12:00:00Z",
      "database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    }
  }
}
```

When no match is found:

```json
{
  "data": {
    "match_found": false,
    "similarity": null,
    "match_type": null,
    "entry": null
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `match_found` | bool | Whether a match with similarity >= 0.86 was found |
| `similarity` | float or null | Best similarity score (0.0–1.0) |
| `match_type` | string or null | Match type (keyword, semantic) |
| `entry` | object or null | Matched entry details |
| `entry.id` | string | ULID |
| `entry.input` | string | Original natural language input |
| `entry.sql` | string | Saved SQL query |
| `entry.optimization_summary` | string | Optimization notes |
| `entry.created_at` | string | Creation timestamp |
| `entry.updated_at` | string | Last update timestamp |
| `entry.database_id` | string | Associated database ID |

---

## 11. `querylex history`

Browse query history by topic.

### Usage

```bash
querylex history <topic>
```

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | topic | yes | Topic keyword to search for |

### Response Type

`Response<HistoryData>`

```json
{
  "data": {
    "topic": "user queries",
    "results": [
      {
        "id": "01JQZRYV4K0000000000000000",
        "input": "find users by email",
        "similarity": 0.92,
        "sql": "SELECT * FROM users WHERE email = $1",
        "last_used_at": "2026-06-08T12:00:00Z"
      },
      {
        "id": "01JQZRYV4K0000000000000001",
        "input": "list active users",
        "similarity": 0.45,
        "sql": "SELECT * FROM users WHERE active = true",
        "last_used_at": "2026-06-07T08:30:00Z"
      }
    ]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `topic` | string | Searched topic |
| `results[]` | array | Ranked results |
| `results[].id` | string | ULID |
| `results[].input` | string | Natural language description |
| `results[].similarity` | float | Composite score (semantic 0.8 + recency 0.2) |
| `results[].sql` | string | Saved SQL query |
| `results[].last_used_at` | string | Last access timestamp |

### Scoring

Results are sorted by a composite score:
- `semantic_similarity * 0.8 + recency_score * 0.2`
- Recency decays exponentially: `exp(-days_since_last_use / 43.3)`
- Results below 0.01 are filtered out

---

## 12. `querylex delete`

Delete a memory entry.

### Usage

```bash
querylex delete <input>
```

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | input | yes | Natural language input of the entry to delete |

### Response Type

`Response<DeleteData>`

```json
{
  "data": {
    "deleted": true,
    "entry": {
      "id": "01JQZRYV4K0000000000000000",
      "input": "find users by email",
      "database_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    }
  }
}
```

If the entry doesn't exist:

```json
{
  "data": {
    "deleted": false,
    "entry": null
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `deleted` | bool | Whether an entry was removed |
| `entry.id` | string or null | ULID of deleted entry |
| `entry.input` | string or null | Original input |
| `entry.database_id` | string or null | Associated database ID |

### Behavior

- Deleting a non-existent entry is idempotent: returns `deleted: false` with no error
- Keyword index is automatically rebuilt after deletion

---

## 13. `querylex resolve`

Resolve natural language to table/column candidates.

### Usage

```bash
querylex resolve <question>
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--tables-json` | string | `""` | Tables as JSON array |

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | question | yes | Natural language question (multiple words joined as single argument) |

### Response Type

`Response<ResolveData>`

```json
{
  "data": {
    "tables": [
      {
        "name": "customers",
        "score": 0.95,
        "match_type": "terminology",
        "matched_term": "customer"
      },
      {
        "name": "orders",
        "score": 0.72,
        "match_type": "fuzzy",
        "matched_term": "order"
      }
    ],
    "columns": [
      {
        "name": "email",
        "table": "customers",
        "confidence": 0.88,
        "reason": "Column name 'email' matches common term for contact"
      }
    ],
    "confidence": 0.83
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `tables[]` | array | Matched table candidates |
| `tables[].name` | string | Table name |
| `tables[].score` | float | Match score 0.0–1.0 |
| `tables[].match_type` | string | `"exact"`, `"terminology"`, `"substring"`, `"fuzzy"`, `"semantic"` |
| `tables[].matched_term` | string | The term that matched |
| `columns[]` | array | Matched column candidates |
| `columns[].name` | string | Column name |
| `columns[].table` | string | Parent table |
| `columns[].confidence` | float | Confidence 0.0–1.0 |
| `columns[].reason` | string | Explanation of why it matched |
| `confidence` | float | Overall confidence across all matches |

### Behavior

- Pure computation from local `schema_slim.json` — no database connection needed
- Multi-pass deterministic matching with 5 strategies (exact, terminology, substring, fuzzy, semantic)

### Warnings

- `NO_MATCHING_TABLES` — No matches found for the input

---

## 14. `querylex completion`

Generate shell completion script.

### Usage

```bash
querylex completion [bash|zsh|fish|powershell]
```

### Flags

None.

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | shell | yes | One of: `bash`, `zsh`, `fish`, `powershell` |

### Response

Outputs the completion script to stdout. No JSON envelope.

---

## 15. `querylex version`

Display version information.

### Usage

```bash
querylex version
```

### Flags

None.

### Arguments

None.

### Response

```
querylex version 1.0.0 (commit abc1234, built 2026-06-08T12:00:00Z)
```

When built without ldflags:

```
querylex version dev (commit unknown, built unknown)
```

No JSON envelope.

---

## Error Codes

| Code | Description | Retryable |
|------|-------------|-----------|
| `INVALID_ARGUMENT` | Invalid command arguments | false |
| `CONNECTION_FAILED` | Database connection failed | true |
| `CREDENTIAL_UNAVAILABLE` | Credential could not be retrieved from the keychain | true |
| `WORKSPACE_STATE_INVALID` | Workspace state is missing, malformed, or internally inconsistent | false |
| `LOCK_ACQUISITION_TIMEOUT` | File lock could not be acquired within timeout | true |
| `UNSUPPORTED_DATABASE` | Database type is not supported | false |
| `PERMISSION_DENIED` | Active credentials lack required permissions | false |
| `INTERNAL_ERROR` | An unexpected internal error occurred | false |
| `INVALID_SQL` | SQL could not be validated against the database schema | false |
| `UNSAFE_SQL` | DML/DCL statements (INSERT, UPDATE, DELETE, DROP, ALTER, etc.) are not permitted | false |
| `TABLE_NOT_FOUND` | Referenced table does not exist in the database schema | false |
| `COLUMN_NOT_FOUND` | Referenced column does not exist in the table | false |
| `NO_MATCHING_TABLES` | No matching tables found for the given input | false |
| `EXPLAIN_FAILED` | Execution plan extraction failed | true |
| `JOIN_PATH_NOT_FOUND` | No join path exists between the specified tables | false |
| `STATS_UNAVAILABLE` | Table statistics are unavailable | true |
| `INDEX_ANALYSIS_FAILED` | Index metadata extraction failed | true |
| `SCHEMA_PARSE_ERROR` | Schema data could not be parsed or is in an unexpected format | false |
| `TERMINOLOGY_PARSE_ERROR` | terminologies.md contains a malformed YAML block | false |
| `MEMORY_INDEX_STALE` | Memory index metadata is stale relative to memory.sqlite | true |
| `MEMORY_STORE_UNAVAILABLE` | Memory subsystem is unavailable | true |
| `MEMORY_WRITE_FAILED` | Unable to write memory entry | true |
| `EXPLAIN_CACHE_STALE` | Explain cache entry is stale (fingerprint mismatch or TTL expired) | true |

## Preflight Requirements by Command

| Command | Preflight Type | DB Connection Required? | Schema Indexed Required? |
|---------|----------------|------------------------|--------------------------|
| `add-db` | None | No (creates connection) | No |
| `workspace-stats` | Workspace only | No | No |
| `schema` | Full | Yes | Yes |
| `stats` | Full | Yes | Yes |
| `indexes` (default) | Workspace + artifacts | No | Yes |
| `indexes --live` | Full | Yes | Yes |
| `explain` | Full | Yes | Yes |
| `validate` | Full | Yes | Yes |
| `joins` (default) | Workspace + artifacts | No | Yes |
| `joins` (fallback) | Full | Yes | Yes |
| `resolve` | Workspace + artifacts | No | Yes |
| `save` | Memory | No | Yes |
| `memory` | Memory | No | Yes |
| `history` | Memory | No | Yes |
| `delete` | Memory | No | Yes |
| `completion` | None | No | No |

## Common Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General failure (error response) |
| 130 | Interrupted by SIGINT/SIGTERM |
