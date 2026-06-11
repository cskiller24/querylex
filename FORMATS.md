# List of formats in querylex

## database.json sample format

```json
{
  "id": "prod-mysql-main",
  "name": "Production MySQL",
  "type": "mysql",
  "version": "8.0.36",
  "host": "db.example.internal",
  "port": 3306,
  "database": "app_db",
  "username": "app_user",
  "ssl": {
    "enabled": true,
    "mode": "REQUIRED"
  },
  "timezone": "UTC",
  "encoding": "utf8mb4",
  "connection": {
    "max_pool_size": 10,
    "connect_timeout_ms": 10000,
    "read_timeout_ms": 30000,
    "write_timeout_ms": 30000
  },
  "metadata": {
    "environment": "production",
    "created_at": "2026-05-29T00:00:00Z",
    "updated_at": "2026-05-29T00:00:00Z",
    "last_connected_at": "2026-05-29T00:00:00Z",
    "schema_count": 12,
    "table_count": 184
  },
  "features": {
    "supports_transactions": true,
    "supports_json": true,
    "supports_cte": true,
    "supports_window_functions": true
  },
  "credential_reference": {
    "provider": "os-keychain",
    "service": "querylex",
    "account": "prod-mysql-main",
    "secret_kind": "database-password",
    "created_at": "2026-05-29T00:00:00Z"
  }
}
```

## schema.json sample format

```json
{
  "name": "mydb",
  "desc": "Example database",
  "labels": [
    { "name": "production", "virtual": false }
  ],
  "driver": {
    "name": "postgres",
    "database_version": "14.0",
    "meta": {
      "current_schema": "public",
      "search_paths": ["public"],
      "dict": { "Tables": "Tables" }
    }
  },
  "tables": [
    {
      "name": "users",
      "type": "BASE TABLE",
      "comment": "Users table",
      "labels": [{ "name": "core" }],
      "columns": [
        { "name": "id",         "type": "integer",      "nullable": false, "extra_def": "auto_increment", "comment": "Primary key" },
        { "name": "name",       "type": "varchar(255)",  "nullable": false, "comment": "Full name" },
        { "name": "email",      "type": "varchar(355)",  "nullable": false, "comment": "Email address" },
        { "name": "created_at", "type": "timestamp",     "nullable": false, "default": "now()" },
        { "name": "deleted_at", "type": "timestamp",     "nullable": true,  "default": null }
      ],
      "indexes": [
        { "name": "users_pkey",       "def": "PRIMARY KEY (id)",    "table": "users", "columns": ["id"] },
        { "name": "users_email_idx",  "def": "UNIQUE (email)",      "table": "users", "columns": ["email"], "comment": "Unique email index" }
      ],
      "constraints": [
        { "name": "users_pkey",        "type": "PRIMARY KEY", "def": "PRIMARY KEY (id)",   "table": "users", "columns": ["id"] },
        { "name": "users_email_uniq",  "type": "UNIQUE",      "def": "UNIQUE (email)",     "table": "users", "columns": ["email"] }
      ],
      "triggers": [
        { "name": "update_users_updated_at", "def": "BEFORE UPDATE ON users ...", "comment": "Keep updated_at fresh" }
      ],
      "def": "CREATE TABLE users (...)"
    },
    {
      "name": "posts",
      "type": "BASE TABLE",
      "comment": "Posts table",
      "columns": [
        { "name": "id",      "type": "integer",     "nullable": false },
        { "name": "user_id", "type": "integer",     "nullable": false, "comment": "FK to users" },
        { "name": "title",   "type": "varchar(255)", "nullable": false }
      ],
      "indexes": [
        { "name": "posts_pkey",        "def": "PRIMARY KEY (id)",      "table": "posts", "columns": ["id"] },
        { "name": "posts_user_id_idx", "def": "INDEX (user_id)",       "table": "posts", "columns": ["user_id"] }
      ],
      "constraints": [
        { "name": "posts_pkey",    "type": "PRIMARY KEY", "def": "PRIMARY KEY (id)",                    "table": "posts", "columns": ["id"] },
        { "name": "posts_user_fk", "type": "FOREIGN KEY", "def": "FOREIGN KEY (user_id) REFERENCES users(id)", "table": "posts", "referenced_table": "users", "columns": ["user_id"], "referenced_columns": ["id"] }
      ],
      "referenced_tables": ["users"],
      "def": "CREATE TABLE posts (...)"
    }
  ],
  "relations": [
    {
      "table": "posts",
      "columns": ["user_id"],
      "cardinality": "zero_or_more",
      "parent_table": "users",
      "parent_columns": ["id"],
      "parent_cardinality": "exactly_one",
      "def": "FOREIGN KEY (user_id) REFERENCES users(id)",
      "virtual": false
    }
  ],
  "functions": [
    {
      "name": "now",
      "return_type": "timestamp",
      "arguments": "",
      "type": "FUNCTION"
    }
  ],
  "enums": [
    {
      "name": "user_role",
      "values": ["admin", "editor", "viewer"]
    }
  ],
  "viewpoints": [
    {
      "name": "User content",
      "desc": "Tables related to users and their posts",
      "labels": ["core"],
      "tables": ["users", "posts"],
      "distance": 1,
      "groups": [
        {
          "name": "Identity",
          "desc": "User identity tables",
          "tables": ["users"],
          "color": "#4A90D9"
        }
      ]
    }
  ]
}
```

## domain_map.json sample format

```json
{
  "metadata": {
    "table_count": "<int>",
    "domain_count": "<int>",
    "subdomain_count": "<int>"
  },
  "domains": {
    "<domain_name>": {
      "tables": [
        "<table_name>"
      ],
      "sub_domains": {
        "<sub_domain_name>": [
          "<table_name>"
        ]
      }
    }
  }
}
```

## join_graph.json sample format

```json
{
  "relationships": [
    {
      "from": "<table>.<column>",
      "to": "<table>.<column>",
      "declared": "<bool>",
      "cross_domain": "<bool>"
    },
    {
      "from": "<table>.(<col1>,<col2>)",
      "to": "<table>.(<col1>,<col2>)",
      "declared": "<bool>",
      "cross_domain": "<bool>"
    }
  ]
}
```

## schema_map.json sample format

```json
{
  "tables": {
    "<table_name>": {
      "domain": "<string>",
      "sub_domain": "<string|null>",
      "pk": "<string|null>",
      "bridge": "<bool>",
      "bridge_domains": ["<string>"],
      "fk_out": [
        {
          "to": "<table>.<column>",
          "declared": "<bool>"
        }
      ],
      "fk_in": [
        "<table>.<column>"
      ],
      "indexed_columns": [
        "<column>"
      ],
      "composite_indexes": [
        ["<col1>", "<col2>"]
      ]
    }
  }
}
```

## querylex.json sample format

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
      "status": "indexing",
      "indexing_progress": 45
    }
  ],
  "active_database_id": "prod-mysql-main",
  "revision": 42,
  "updated_at": "2026-05-29T00:00:00Z"
}
```

## terminologies.md sample format

`terminologies.md` is Markdown with one fenced `querylex-terms` block. Freeform Markdown outside the block is allowed for user notes and ignored by the parser.

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

Required fields per term:

| Field | Type | Notes |
|---|---|---|
| `term` | string | Business term or phrase. |
| `type` | string | One of `metric`, `entity`, `entity_filter`, `dimension`, `date_window`, or `synonym`. |
| `maps_to` | array | Table and optional column mappings. |

Optional fields include `description`, `filters`, `values`, `expression`, and `synonyms`.

## Command response envelope

All querylex commands return JSON only. Successful commands use this envelope:

```json
{
  "success": true,
  "data": {},
  "warnings": [],
  "meta": {
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 42
  }
}
```

Failed commands use this envelope:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Machine-readable explanation.",
    "retryable": false,
    "details": null
  },
  "meta": {
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 42
  }
}
```

Warnings do not fail execution:

```json
{
  "code": "WARNING_CODE",
  "message": "Human-readable warning.",
  "details": null
}
```

Common stable error codes:

| Code | Meaning | Retryable |
|---|---|---|
| `INVALID_ARGUMENT` | Invalid command arguments. | false |
| `INVALID_SQL` | SQL validation failed. | false |
| `UNSAFE_SQL` | SQL would execute unsafe or unsupported behavior. | false |
| `TABLE_NOT_FOUND` | Referenced table does not exist. | false |
| `COLUMN_NOT_FOUND` | Referenced column does not exist. | false |
| `CONNECTION_FAILED` | Database connection failed. | true |
| `WORKSPACE_STATE_INVALID` | Querylex workspace state is missing, malformed, or internally inconsistent. | false |
| `ACTIVE_DATABASE_NOT_SET` | No active database is selected. | false |
| `DATABASE_NOT_INDEXED` | The selected database has no usable indexed artifacts. | false |
| `INDEXING_IN_PROGRESS` | The selected database is currently indexing and has no usable previous manifest. | true |
| `PERMISSION_DENIED` | Active credentials lack required permissions. | false |
| `CREDENTIAL_UNAVAILABLE` | Credential reference could not be resolved from the OS keychain. | true |
| `MEMORY_NOT_FOUND` | Memory entry was not found. | false |
| `MEMORY_STORE_UNAVAILABLE` | Memory subsystem is unavailable. | true |
| `MEMORY_WRITE_FAILED` | Memory entry could not be written. | true |
| `MEMORY_INDEX_STALE` | Memory index metadata is stale relative to `memory.sqlite`. | true |
| `QUERY_TIMEOUT` | Command exceeded timeout. | true |
| `SCHEMA_PARSE_ERROR` | Schema metadata could not be parsed. | true |
| `TERMINOLOGY_PARSE_ERROR` | `terminologies.md` could not be parsed. | false |
| `STATS_UNAVAILABLE` | Table statistics are stale or unavailable. | true |
| `INDEX_ANALYSIS_FAILED` | Index metadata could not be analyzed. | true |
| `EXPLAIN_FAILED` | Execution plan could not be generated. | true |
| `EXPLAIN_CACHE_STALE` | Cached explain output cannot be trusted for the current schema, stats, or database version. | true |
| `JOIN_PATH_NOT_FOUND` | No relationship path exists between supplied tables. | false |
| `AI_SERVICE_UNAVAILABLE` | Required AI reasoning service could not be reached. | true |
| `UNSUPPORTED_DATABASE` | Database type is not supported. | false |
| `INTERNAL_ERROR` | Unexpected failure. | true |

## Command table identifier arguments

Commands that accept tables use repeated `--table` arguments as the canonical form:

```bash
querylex schema --table customers --table orders
querylex joins --table public.orders --table public.order_items
querylex indexes --tables-json '["Sales Region.Order Items", "dbo.Customers"]'
```

Rules:

- `--table <identifier>` may be repeated.
- `--tables-json <json-array>` is available for exact machine invocation and identifiers that are hard to quote in a shell.
- Positional shorthand such as `querylex schema customers orders` is allowed only for simple identifiers.
- The parser must not split identifiers on dots. `public.orders` is one identifier.
- Dialect-specific quoted identifiers must be preserved and normalized by the active dialect adapter.

## querylex-add-db command samples

### Interactive first prompt

Command:

```bash
querylex-add-db
```

Response:

```json
{
  "success": true,
  "data": {
    "state": "awaiting_input",
    "step": "database_type",
    "prompt": "Select database type.",
    "options": [
      "mysql",
      "postgres",
      "sqlite",
      "microsoft sql",
      "mariadb"
    ],
    "defaults": {
      "port_by_type": {
        "mysql": 3306,
        "postgres": 5432,
        "microsoft sql": 1433,
        "mariadb": 3306
      }
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": null,
    "duration_ms": 8
  }
}
```

### MySQL database added and indexing started

Command:

```bash
querylex-add-db --type mysql --name "Production MySQL" --host db.example.internal --port 3306 --database app_db --username app_user --ssl-mode REQUIRED
```

Response:

```json
{
  "success": true,
  "data": {
    "database": {
      "id": "prod-mysql-main",
      "name": "Production MySQL",
      "type": "mysql",
      "version": "8.0.36",
      "host": "db.example.internal",
      "port": 3306,
      "database": "app_db",
      "username": "app_user",
      "ssl": {
        "enabled": true,
        "mode": "REQUIRED"
      },
      "credential_reference": {
        "provider": "os-keychain",
        "service": "querylex",
        "account": "prod-mysql-main"
      }
    },
    "files_written": [
      "$HOME/.querylex/prod-mysql-main/database.json"
    ],
    "active_database_id": "prod-mysql-main",
    "indexing": {
      "status": "indexing",
      "progress_percent": 0,
      "job_id": "idx_20260529_000001",
      "artifacts_expected": [
        "$HOME/.querylex/prod-mysql-main/schema.json",
        "$HOME/.querylex/prod-mysql-main/schema_slim.json",
        "$HOME/.querylex/prod-mysql-main/domain_map.json",
        "$HOME/.querylex/prod-mysql-main/schema/domain_map.json",
        "$HOME/.querylex/prod-mysql-main/schema/join_graph.json"
      ]
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 921
  }
}
```

### SQLite database added

Command:

```bash
querylex-add-db --type sqlite --name "Local Analytics" --path "$HOME/data/analytics.sqlite"
```

Response:

```json
{
  "success": true,
  "data": {
    "database": {
      "id": "local-analytics",
      "name": "Local Analytics",
      "type": "sqlite",
      "path": "$HOME/data/analytics.sqlite",
      "credential_reference": null
    },
    "files_written": [
      "$HOME/.querylex/local-analytics/database.json"
    ],
    "active_database_id": "local-analytics",
    "indexing": {
      "status": "indexed",
      "progress_percent": 100,
      "job_id": "idx_20260529_000002",
      "artifacts_written": [
        "$HOME/.querylex/local-analytics/schema.json",
        "$HOME/.querylex/local-analytics/schema_slim.json",
        "$HOME/.querylex/local-analytics/domain_map.json",
        "$HOME/.querylex/local-analytics/schema/domain_map.json",
        "$HOME/.querylex/local-analytics/schema/join_graph.json"
      ]
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "local-analytics",
    "duration_ms": 412
  }
}
```

### Connection failure

Command:

```bash
querylex-add-db --type postgres --name "Staging PostgreSQL" --host staging-db.example.internal --port 5432 --database app_staging --username app_user
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "CONNECTION_FAILED",
    "message": "Unable to connect to staging-db.example.internal:5432.",
    "retryable": true,
    "details": {
      "host": "staging-db.example.internal",
      "port": 5432,
      "database": "app_staging",
      "credential_saved": false,
      "files_written": []
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": null,
    "duration_ms": 10002
  }
}
```

## querylex-stats command samples

### Workspace status

Command:

```bash
querylex-stats
```

Response:

```json
{
  "success": true,
  "data": {
    "active_database_id": "prod-mysql-main",
    "connected_databases": [
      {
        "id": "prod-mysql-main",
        "name": "Production MySQL",
        "type": "mysql",
        "status": "indexed",
        "indexing_progress": 100,
        "last_indexed_at": "2026-05-29T00:00:00Z",
        "schema_count": 12,
        "table_count": 184,
        "memory_entry_count": 931
      },
      {
        "id": "staging-postgres",
        "name": "Staging PostgreSQL",
        "type": "postgres",
        "status": "indexing",
        "indexing_progress": 45,
        "last_indexed_at": null,
        "schema_count": 3,
        "table_count": 52,
        "memory_entry_count": 0
      }
    ]
  },
  "warnings": [
    {
      "code": "INDEXING_IN_PROGRESS",
      "message": "staging-postgres is still indexing.",
      "details": {
        "database_id": "staging-postgres",
        "progress_percent": 45
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 15
  }
}
```

### No active database

Command:

```bash
querylex-stats
```

Response:

```json
{
  "success": true,
  "data": {
    "active_database_id": null,
    "connected_databases": []
  },
  "warnings": [
    {
      "code": "NO_DATABASES_CONNECTED",
      "message": "No querylex databases have been added.",
      "details": null
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": null,
    "duration_ms": 4
  }
}
```

## querylex resolve command samples

### Resolved tables and columns

Command:

```bash
querylex resolve "top customers by revenue in the last 30 days"
```

Response:

```json
{
  "success": true,
  "data": {
    "question": "top customers by revenue in the last 30 days",
    "tables": [
      {
        "table": "customers",
        "confidence": 0.98,
        "matched_terms": [
          "customers"
        ],
        "columns": [
          {
            "name": "id",
            "confidence": 0.97,
            "reason": "Customer primary key."
          },
          {
            "name": "name",
            "confidence": 0.91,
            "reason": "Display column for customer."
          }
        ]
      },
      {
        "table": "orders",
        "confidence": 0.95,
        "matched_terms": [
          "revenue",
          "last 30 days"
        ],
        "columns": [
          {
            "name": "customer_id",
            "confidence": 0.96,
            "reason": "Join key to customers."
          },
          {
            "name": "created_at",
            "confidence": 0.94,
            "reason": "Date filter for last 30 days."
          },
          {
            "name": "total_amount",
            "confidence": 0.94,
            "reason": "Revenue amount."
          }
        ]
      }
    ],
    "terminology_matches": [
      {
        "term": "revenue",
        "maps_to": "orders.total_amount",
        "source": "$HOME/.querylex/prod-mysql-main/terminologies.md"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 42
  }
}
```

### No matching tables

Command:

```bash
querylex resolve "customers with high happiness score"
```

Response:

```json
{
  "success": true,
  "data": {
    "question": "customers with high happiness score",
    "tables": [],
    "terminology_matches": []
  },
  "warnings": [
    {
      "code": "NO_MATCHING_TABLES",
      "message": "No relevant tables were found.",
      "details": {
        "lowest_returned_confidence": null
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 18
  }
}
```

### Schema parse error

Command:

```bash
querylex resolve "orders from last month"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "SCHEMA_PARSE_ERROR",
    "message": "Unable to load schema metadata for active database.",
    "retryable": true,
    "details": {
      "schema_path": "$HOME/.querylex/prod-mysql-main/schema_slim.json"
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 15
  }
}
```

## querylex memory command samples

### Question match found

Command:

```bash
querylex memory "top customers by revenue in the last 30 days"
```

Response:

```json
{
  "success": true,
  "data": {
    "match_found": true,
    "similarity": 0.94,
    "match_type": "question_to_sql",
    "entry": {
      "id": "mem_01JZ000000000000000001",
      "input": "top customers by revenue",
      "sql": "SELECT c.id, c.name, SUM(o.total_amount) AS revenue FROM customers c JOIN orders o ON o.customer_id = c.id GROUP BY c.id, c.name ORDER BY revenue DESC LIMIT 10;",
      "created_at": "2026-05-20T12:00:00Z",
      "updated_at": "2026-05-20T12:00:00Z",
      "database_id": "prod-mysql-main"
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "cache_hit": true,
    "duration_ms": 6
  }
}
```

### Optimized SQL match found

Command:

```bash
querylex memory "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'"
```

Response:

```json
{
  "success": true,
  "data": {
    "match_found": true,
    "similarity": 0.91,
    "match_type": "sql_to_optimized_sql",
    "entry": {
      "id": "mem_01JZ000000000000000002",
      "input": "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'",
      "sql": "SELECT * FROM orders WHERE created_at >= '2026-05-01 00:00:00' AND created_at < '2026-05-02 00:00:00';",
      "optimization_summary": "Rewrote DATE(created_at) predicate into a range predicate so an index on created_at can be used.",
      "database_id": "prod-mysql-main"
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "cache_hit": true,
    "duration_ms": 7
  }
}
```

### No match

Command:

```bash
querylex memory "customers by support ticket sentiment"
```

Response:

```json
{
  "success": true,
  "data": {
    "match_found": false,
    "similarity": null,
    "match_type": null,
    "entry": null
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "cache_hit": false,
    "duration_ms": 5
  }
}
```

### Memory unavailable

Command:

```bash
querylex memory "top customers by revenue"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "MEMORY_STORE_UNAVAILABLE",
    "message": "Memory subsystem is unavailable.",
    "retryable": true,
    "details": {
      "store": "$HOME/.querylex/prod-mysql-main/memory.sqlite"
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 4
  }
}
```

## querylex history command samples

### Related history found

Command:

```bash
querylex history "customer revenue"
```

Response:

```json
{
  "success": true,
  "data": {
    "topic": "customer revenue",
    "results": [
      {
        "id": "mem_01JZ000000000000000003",
        "input": "monthly revenue by customer",
        "similarity": 0.88,
        "sql": "SELECT c.id, c.name, DATE_FORMAT(o.created_at, '%Y-%m') AS month, SUM(o.total_amount) AS revenue FROM customers c JOIN orders o ON o.customer_id = c.id GROUP BY c.id, c.name, month;",
        "last_used_at": "2026-05-28T09:00:00Z"
      },
      {
        "id": "mem_01JZ000000000000000004",
        "input": "top customers by revenue",
        "similarity": 0.83,
        "sql": "SELECT customer_id, SUM(total_amount) AS revenue FROM orders GROUP BY customer_id ORDER BY revenue DESC LIMIT 10;",
        "last_used_at": "2026-05-21T10:30:00Z"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 12
  }
}
```

### Empty history

Command:

```bash
querylex history "support sentiment"
```

Response:

```json
{
  "success": true,
  "data": {
    "topic": "support sentiment",
    "results": []
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 7
  }
}
```

## querylex schema command samples

### Table definitions

Command:

```bash
querylex schema --table customers --table orders
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "customers",
        "schema": "app_db",
        "type": "BASE TABLE",
        "comment": "Customer accounts.",
        "columns": [
          {
            "name": "id",
            "ordinal_position": 1,
            "type": "bigint unsigned",
            "nullable": false,
            "default": null,
            "primary_key": true,
            "generated": false,
            "comment": "Primary key."
          },
          {
            "name": "name",
            "ordinal_position": 2,
            "type": "varchar(255)",
            "nullable": false,
            "default": null,
            "primary_key": false,
            "generated": false,
            "comment": "Customer display name."
          }
        ],
        "constraints": [
          {
            "name": "PRIMARY",
            "type": "PRIMARY KEY",
            "columns": [
              "id"
            ],
            "referenced_table": null,
            "referenced_columns": null
          }
        ],
        "definition": "CREATE TABLE customers (...)"
      },
      {
        "table": "orders",
        "schema": "app_db",
        "type": "BASE TABLE",
        "comment": "Customer orders.",
        "columns": [
          {
            "name": "id",
            "ordinal_position": 1,
            "type": "bigint unsigned",
            "nullable": false,
            "default": null,
            "primary_key": true,
            "generated": false,
            "comment": "Primary key."
          },
          {
            "name": "customer_id",
            "ordinal_position": 2,
            "type": "bigint unsigned",
            "nullable": false,
            "default": null,
            "primary_key": false,
            "generated": false,
            "comment": "FK to customers."
          },
          {
            "name": "created_at",
            "ordinal_position": 3,
            "type": "datetime",
            "nullable": false,
            "default": null,
            "primary_key": false,
            "generated": false,
            "comment": "Order creation timestamp."
          },
          {
            "name": "total_amount",
            "ordinal_position": 4,
            "type": "decimal(12,2)",
            "nullable": false,
            "default": "0.00",
            "primary_key": false,
            "generated": false,
            "comment": "Order revenue amount."
          }
        ],
        "constraints": [
          {
            "name": "PRIMARY",
            "type": "PRIMARY KEY",
            "columns": [
              "id"
            ],
            "referenced_table": null,
            "referenced_columns": null
          },
          {
            "name": "orders_customer_fk",
            "type": "FOREIGN KEY",
            "columns": [
              "customer_id"
            ],
            "referenced_table": "customers",
            "referenced_columns": [
              "id"
            ]
          }
        ],
        "definition": "CREATE TABLE orders (...)"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 14
  }
}
```

### Partial schema with missing table

Command:

```bash
querylex schema --table customers --table invoices
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "customers",
        "schema": "app_db",
        "type": "BASE TABLE",
        "comment": "Customer accounts.",
        "columns": [],
        "constraints": [],
        "definition": "CREATE TABLE customers (...)"
      }
    ],
    "missing_tables": [
      "invoices"
    ]
  },
  "warnings": [
    {
      "code": "TABLE_NOT_FOUND",
      "message": "One or more requested tables were not found.",
      "details": {
        "tables": [
          "invoices"
        ]
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 9
  }
}
```

## querylex stats tables command samples

### Table statistics

Command:

```bash
querylex stats --table customers --table orders
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "customers",
        "row_count": 90321,
        "row_count_exact": false,
        "data_length_bytes": 83886080,
        "index_length_bytes": 25165824,
        "cardinality": {
          "id": 90321,
          "email": 90321,
          "status": 4
        },
        "histograms": [
          {
            "column": "status",
            "buckets": 4,
            "freshness": "fresh"
          }
        ],
        "last_analyzed_at": "2026-05-29T00:00:00Z"
      },
      {
        "table": "orders",
        "row_count": 1523402,
        "row_count_exact": false,
        "data_length_bytes": 536870912,
        "index_length_bytes": 201326592,
        "cardinality": {
          "id": 1523402,
          "customer_id": 90321,
          "status": 5
        },
        "histograms": [],
        "last_analyzed_at": "2026-05-28T23:40:00Z"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 19
  }
}
```

### Stale or unavailable stats

Command:

```bash
querylex stats --table logs
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "logs",
        "row_count": null,
        "row_count_exact": false,
        "data_length_bytes": null,
        "index_length_bytes": null,
        "cardinality": {},
        "histograms": [],
        "last_analyzed_at": null
      }
    ]
  },
  "warnings": [
    {
      "code": "STATS_UNAVAILABLE",
      "message": "Statistics are unavailable for logs.",
      "details": {
        "table": "logs"
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 8
  }
}
```

## querylex joins command samples

### Direct and recursive joins

Command:

```bash
querylex joins --table customers --table orders --table payments
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      "customers",
      "orders",
      "payments"
    ],
    "joins": [
      {
        "left_table": "orders",
        "left_column": "customer_id",
        "right_table": "customers",
        "right_column": "id",
        "join_type": "INNER",
        "relationship": "many_to_one",
        "source": "declared_foreign_key",
        "confidence": 1.0,
        "constraint_name": "orders_customer_fk"
      },
      {
        "left_table": "payments",
        "left_column": "order_id",
        "right_table": "orders",
        "right_column": "id",
        "join_type": "INNER",
        "relationship": "many_to_one",
        "source": "declared_foreign_key",
        "confidence": 1.0,
        "constraint_name": "payments_order_fk"
      }
    ],
    "paths": [
      {
        "from": "customers",
        "to": "payments",
        "distance": 2,
        "tables": [
          "customers",
          "orders",
          "payments"
        ],
        "join_indexes_available": true
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 11
  }
}
```

### Inferred join warning

Command:

```bash
querylex joins --table orders --table shipments
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      "orders",
      "shipments"
    ],
    "joins": [
      {
        "left_table": "shipments",
        "left_column": "order_id",
        "right_table": "orders",
        "right_column": "id",
        "join_type": "INNER",
        "relationship": "many_to_one",
        "source": "inferred_name_match",
        "confidence": 0.82,
        "constraint_name": null
      }
    ],
    "paths": [
      {
        "from": "orders",
        "to": "shipments",
        "distance": 1,
        "tables": [
          "orders",
          "shipments"
        ],
        "join_indexes_available": true
      }
    ]
  },
  "warnings": [
    {
      "code": "INFERRED_JOIN",
      "message": "One join was inferred because no declared foreign key exists.",
      "details": {
        "tables": [
          "orders",
          "shipments"
        ]
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 13
  }
}
```

### No join path

Command:

```bash
querylex joins --table customers --table audit_events
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "JOIN_PATH_NOT_FOUND",
    "message": "No join path exists between supplied tables.",
    "retryable": false,
    "details": {
      "tables": [
        "audit_events",
        "customers"
      ]
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 9
  }
}
```

## querylex indexes command samples

### Index coverage

Command:

```bash
querylex indexes --table customers --table orders
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "customers",
        "indexes": [
          {
            "name": "PRIMARY",
            "type": "BTREE",
            "unique": true,
            "primary": true,
            "visible": true,
            "columns": [
              {
                "name": "id",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 90321,
                "prefix_length": null
              }
            ],
            "expression": null,
            "comment": null
          },
          {
            "name": "customers_email_uq",
            "type": "BTREE",
            "unique": true,
            "primary": false,
            "visible": true,
            "columns": [
              {
                "name": "email",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 90321,
                "prefix_length": null
              }
            ],
            "expression": null,
            "comment": "Unique customer email."
          }
        ]
      },
      {
        "table": "orders",
        "indexes": [
          {
            "name": "idx_orders_customer_created",
            "type": "BTREE",
            "unique": false,
            "primary": false,
            "visible": true,
            "columns": [
              {
                "name": "customer_id",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 90321,
                "prefix_length": null
              },
              {
                "name": "created_at",
                "order": "ASC",
                "sequence": 2,
                "cardinality": 712442,
                "prefix_length": null
              }
            ],
            "expression": null,
            "comment": null
          }
        ]
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 10
  }
}
```

### Table has no secondary indexes

Command:

```bash
querylex indexes --table audit_events
```

Response:

```json
{
  "success": true,
  "data": {
    "tables": [
      {
        "table": "audit_events",
        "indexes": [
          {
            "name": "PRIMARY",
            "type": "BTREE",
            "unique": true,
            "primary": true,
            "visible": true,
            "columns": [
              {
                "name": "id",
                "order": "ASC",
                "sequence": 1,
                "cardinality": 5048122,
                "prefix_length": null
              }
            ],
            "expression": null,
            "comment": null
          }
        ]
      }
    ]
  },
  "warnings": [
    {
      "code": "NO_SECONDARY_INDEXES",
      "message": "audit_events has no secondary indexes.",
      "details": {
        "table": "audit_events"
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 8
  }
}
```

## querylex validate command samples

### Valid SQL

Command:

```bash
querylex validate "SELECT c.id, c.name, SUM(o.total_amount) AS revenue FROM customers c JOIN orders o ON o.customer_id = c.id WHERE o.created_at >= CURRENT_DATE - INTERVAL 30 DAY GROUP BY c.id, c.name ORDER BY revenue DESC LIMIT 10"
```

Response:

```json
{
  "success": true,
  "data": {
    "valid": true,
    "statement_type": "SELECT",
    "dialect": "mysql",
    "normalized_sql": "SELECT c.id, c.name, SUM(o.total_amount) AS revenue FROM customers AS c JOIN orders AS o ON o.customer_id = c.id WHERE o.created_at >= CURRENT_DATE - INTERVAL 30 DAY GROUP BY c.id, c.name ORDER BY revenue DESC LIMIT 10",
    "tables": [
      "customers",
      "orders"
    ],
    "columns": [
      "customers.id",
      "customers.name",
      "orders.customer_id",
      "orders.created_at",
      "orders.total_amount"
    ],
    "read_only": true
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 7
  }
}
```

### Invalid column

Command:

```bash
querylex validate "SELECT fullname FROM customers"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "COLUMN_NOT_FOUND",
    "message": "Unknown column customers.fullname.",
    "retryable": false,
    "details": {
      "line": 1,
      "column": 8,
      "table": "customers",
      "column_name": "fullname",
      "suggestions": [
        "name"
      ]
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 5
  }
}
```

### Unsafe SQL rejected

Command:

```bash
querylex validate "DELETE FROM orders WHERE created_at < '2020-01-01'"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "UNSAFE_SQL",
    "message": "Data-changing statements are not allowed by querylex validate.",
    "retryable": false,
    "details": {
      "statement_type": "DELETE",
      "read_only": false
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 4
  }
}
```

## querylex save command samples

### New memory entry saved

Command:

```bash
querylex save "top customers by revenue in the last 30 days" "SELECT c.id, c.name, SUM(o.total_amount) AS revenue FROM customers c JOIN orders o ON o.customer_id = c.id WHERE o.created_at >= CURRENT_DATE - INTERVAL 30 DAY GROUP BY c.id, c.name ORDER BY revenue DESC LIMIT 10;"
```

Response:

```json
{
  "success": true,
  "data": {
    "saved": true,
    "updated_existing": false,
    "entry": {
      "id": "mem_01JZ000000000000000005",
      "input": "top customers by revenue in the last 30 days",
      "sql_hash": "sha256:2db3b61f7b4c",
      "database_id": "prod-mysql-main",
      "created_at": "2026-05-29T00:00:00Z",
      "updated_at": "2026-05-29T00:00:00Z"
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 6
  }
}
```

### Existing memory entry updated

Command:

```bash
querylex save "top customers by revenue in the last 30 days" "SELECT c.id, c.name, SUM(o.total_amount) AS revenue FROM customers c JOIN orders o ON o.customer_id = c.id WHERE o.created_at >= NOW() - INTERVAL 30 DAY GROUP BY c.id, c.name ORDER BY revenue DESC LIMIT 20;"
```

Response:

```json
{
  "success": true,
  "data": {
    "saved": true,
    "updated_existing": true,
    "entry": {
      "id": "mem_01JZ000000000000000005",
      "input": "top customers by revenue in the last 30 days",
      "sql_hash": "sha256:77842bc224a1",
      "database_id": "prod-mysql-main",
      "created_at": "2026-05-29T00:00:00Z",
      "updated_at": "2026-05-29T01:00:00Z"
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 6
  }
}
```

### Memory write failed

Command:

```bash
querylex save "top customers" "SELECT customer_id FROM orders"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "MEMORY_WRITE_FAILED",
    "message": "Unable to write memory entry.",
    "retryable": true,
    "details": {
      "store": "$HOME/.querylex/prod-mysql-main/memory.sqlite"
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 6
  }
}
```

## querylex delete command samples

### Entry deleted

Command:

```bash
querylex delete "top customers by revenue in the last 30 days"
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": true,
    "entry": {
      "id": "mem_01JZ000000000000000005",
      "input": "top customers by revenue in the last 30 days",
      "database_id": "prod-mysql-main"
    }
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 5
  }
}
```

### Entry not found

Command:

```bash
querylex delete "outdated dashboard query"
```

Response:

```json
{
  "success": true,
  "data": {
    "deleted": false,
    "entry": null
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 5
  }
}
```

## querylex explain command samples

### Estimated execution plan

Command:

```bash
querylex explain "SELECT * FROM orders WHERE created_at >= '2026-05-01'"
```

Response:

```json
{
  "success": true,
  "data": {
    "sql": "SELECT * FROM orders WHERE created_at >= '2026-05-01'",
    "analyze": false,
    "format": "json",
    "execution_plan": [
      {
        "id": 1,
        "select_type": "SIMPLE",
        "table": "orders",
        "access_type": "range",
        "possible_keys": [
          "idx_orders_created_at",
          "idx_orders_customer_created"
        ],
        "key": "idx_orders_created_at",
        "key_length": "5",
        "ref": null,
        "rows_examined": 181223,
        "filtered_percent": 100.0,
        "extra": [
          "Using index condition"
        ],
        "cost_info": {
          "query_cost": 24511.38
        }
      }
    ],
    "heuristics": [
      {
        "code": "RANGE_SCAN",
        "severity": "info",
        "message": "Query uses an indexed range scan."
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 15
  }
}
```

### Analyze execution plan

Command:

```bash
querylex explain "SELECT status, COUNT(*) FROM orders GROUP BY status" --analyze
```

Response:

```json
{
  "success": true,
  "data": {
    "sql": "SELECT status, COUNT(*) FROM orders GROUP BY status",
    "analyze": true,
    "format": "json",
    "execution_plan": [
      {
        "id": 1,
        "select_type": "SIMPLE",
        "table": "orders",
        "access_type": "index",
        "possible_keys": [
          "idx_orders_status"
        ],
        "key": "idx_orders_status",
        "rows_examined": 1523402,
        "filtered_percent": 100.0,
        "actual_rows": 5,
        "actual_time_ms": {
          "first_row": 0.61,
          "last_row": 318.44
        },
        "loops": 1,
        "extra": [
          "Using index"
        ],
        "cost_info": {
          "query_cost": 153210.12
        }
      }
    ],
    "heuristics": [
      {
        "code": "COVERING_INDEX_USED",
        "severity": "info",
        "message": "The selected index covers the grouped column."
      }
    ]
  },
  "warnings": [
    {
      "code": "EXPLAIN_ANALYZE_EXECUTES_QUERY",
      "message": "EXPLAIN ANALYZE may execute the query depending on the database engine.",
      "details": {
        "database_type": "mysql"
      }
    }
  ],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 344
  }
}
```

### Full table scan detected

Command:

```bash
querylex explain "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'"
```

Response:

```json
{
  "success": true,
  "data": {
    "sql": "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'",
    "analyze": false,
    "format": "json",
    "execution_plan": [
      {
        "id": 1,
        "select_type": "SIMPLE",
        "table": "orders",
        "access_type": "ALL",
        "possible_keys": [
          "idx_orders_created_at"
        ],
        "key": null,
        "key_length": null,
        "ref": null,
        "rows_examined": 1523402,
        "filtered_percent": 10.0,
        "extra": [
          "Using where"
        ],
        "cost_info": {
          "query_cost": 310992.4
        }
      }
    ],
    "heuristics": [
      {
        "code": "FULL_TABLE_SCAN",
        "severity": "warning",
        "message": "The orders table is scanned fully."
      },
      {
        "code": "NON_SARGABLE_PREDICATE",
        "severity": "warning",
        "message": "DATE(created_at) prevents normal index range usage."
      }
    ]
  },
  "warnings": [],
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 16
  }
}
```

### Explain failed

Command:

```bash
querylex explain "SELECT * FROM missing_table"
```

Response:

```json
{
  "success": false,
  "error": {
    "code": "EXPLAIN_FAILED",
    "message": "Failed to generate execution plan.",
    "retryable": false,
    "details": {
      "database_error_code": "ER_NO_SUCH_TABLE",
      "table": "missing_table"
    }
  },
  "meta": {
    "trace_id": "uuid",
    "protocol_version": "1.0.0",
    "active_database_id": "prod-mysql-main",
    "duration_ms": 10
  }
}
```
