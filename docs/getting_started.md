# Getting Started with QueryLex

QueryLex is a context-aware SQL query companion CLI tool for MySQL, MariaDB, PostgreSQL, SQLite, and Microsoft SQL Server. It helps you explore database schemas, validate SQL, analyze query plans, discover join paths, and persist query memory.

## Installation

### From Source (Go 1.26+)

```bash
git clone https://github.com/cskiller24/querylex.git
cd querylex
make build
```

The binary is placed in `bin/querylex`.

### Install to GOPATH

```bash
make install
```

### Verify Installation

```bash
querylex version
# querylex version dev (commit unknown, built unknown)
```

When built via release or with ldflags, the version, commit, and build date are injected:

```bash
querylex version
# querylex version 1.0.0 (commit abc1234, built 2026-06-08T12:00:00Z)
```

## Configuration

### Directory Structure

QueryLex stores all state under `~/.querylex/`:

```
~/.querylex/
├── querylex.json          # Workspace registry (connected databases)
├── credentials.json.enc   # Encrypted credentials file (headless Linux fallback)
├── logs/                  # Log files
└── <database-id>/         # Per-database directory
    ├── database.json      # Connection metadata and credential reference
    ├── schema_map.json    # Indexed schema artifacts
    ├── join_graph.json    # Join relationship graph
    ├── memory.sqlite      # Saved query memory (WAL mode)
    └── ...
```

### Credential Storage

QueryLex uses a credential store chain with fallback:

1. **OS Keychain** (primary) — macOS Keychain, Windows Credential Manager, Linux Secret Service via D-Bus
2. **Encrypted File** (fallback) — AES-256-GCM encrypted `~/.querylex/credentials.json.enc` with scrypt key derivation. On headless Linux, you'll be prompted for a passphrase
3. **Environment Variables** (last resort) — `QUERYLEX_DB_PASSWORD` for database passwords

For CI/non-interactive environments, set:

```bash
export QUERYLEX_KEYCHAIN_PASSPHRASE="your-passphrase"
```

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `QUERYLEX_DB_PASSWORD` | Database password fallback | — |
| `QUERYLEX_KEYCHAIN_PASSPHRASE` | Passphrase for encrypted file store | — |

## Adding Your First Database

```bash
querylex add-db
```

You'll be guided through an interactive setup:

```
? Database type:  [Use arrows to move, type to filter]
  ▸ MySQL / MariaDB
    PostgreSQL
    SQLite
    Microsoft SQL Server
? Display name: my-database
? Host: localhost
? Port: 3306
? Database name: myapp
? Username: app_user
? Password: [hidden input]
? SSL mode: prefer
```

After setup, QueryLex automatically:
1. Stores credentials in your OS keychain
2. Tests the database connection
3. Runs the full indexing pipeline (schema extraction, join graph, domain clustering)
4. Sets the database as active

## Verifying Workspace Status

```bash
querylex workspace-stats --human
```

Shows all connected databases, their indexing status, and health information.

## Core Workflows

### 1. Explore Schema

```bash
# All tables
querylex schema

# Specific tables
querylex schema --table users --table orders

# Tables from JSON array
querylex schema --tables-json '["users","orders"]'
```

Returns column definitions, types, constraints, and defaults for each table.

### 2. Explain a Query

```bash
querylex explain "SELECT u.name, COUNT(o.id) FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.name"

# With actual execution timing (prompts for confirmation)
querylex explain --analyze "SELECT * FROM users WHERE email = 'test@example.com'"
```

Returns a normalized execution plan with heuristic analysis (full scans, missing indexes, cartesian joins, etc.).

### 3. Validate SQL

```bash
querylex validate "SELECT * FROM users WHERE id = 1"
```

Two-layer validation:
- **Layer 1**: Rejects DML/DCL (INSERT, UPDATE, DELETE, DROP, etc.)
- **Layer 2**: Resolves table and column references against the live schema

### 4. Save and Search Queries

```bash
# Save a query to memory
querylex save "find users by email" "SELECT * FROM users WHERE email = ?"

# Search memory
querylex memory "find users by email"

# Browse history by topic
querylex history "user queries"

# Delete a memory entry
querylex delete "find users by email"
```

### 5. Resolve Natural Language

```bash
querylex resolve "find customer orders"
```

Uses multi-pass deterministic matching against schema metadata to suggest relevant tables and columns. No database connection needed.

### 6. Analyze Joins

```bash
# All joins from a table
querylex joins --table orders

# Join path between two tables
querylex joins --table orders --table users
```

### 7. Check Indexes

```bash
# From indexed schema map (fast)
querylex indexes --table orders

# Live from database (real-time)
querylex indexes --table orders --live
```

### 8. Table Statistics

```bash
querylex stats --table orders
```

### Shell Completions

```bash
# Bash
querylex completion bash > /etc/bash_completion.d/querylex

# Zsh
querylex completion zsh > /usr/local/share/zsh/site-functions/_querylex

# Fish
querylex completion fish > ~/.config/fish/completions/querylex.fish

# PowerShell
querylex completion powershell > profile.ps1
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build binary to `bin/querylex` |
| `make test` | Run tests (`go test ./... -short -count=1`) |
| `make clean` | Remove `bin/` directory |
| `make install` | Install to `$GOPATH/bin` |
| `make lint` | Run `go vet ./...` |
| `make completions` | Generate shell completion scripts |
| `make release` | Run goreleaser (cross-platform release) |

## Supported Databases

| Database | Driver | Package |
|----------|--------|---------|
| MySQL / MariaDB | `go-sql-driver/mysql` | `internal/db/mysql/`, `internal/db/mariadb/` |
| PostgreSQL | `pgx/v5` | `internal/db/postgresql/` |
| SQLite | `modernc.org/sqlite` | `internal/db/sqlite/` |
| Microsoft SQL Server | `go-mssqldb` | `internal/db/mssql/` |

## Output Format

All deterministic commands return a structured JSON envelope:

```json
{
  "success": true,
  "data": { ... },
  "warnings": [],
  "meta": {
    "trace_id": "uuid-v4",
    "protocol_version": "1.0.0",
    "active_database_id": "db-uuid",
    "duration_ms": 123
  }
}
```

On failure:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description",
    "retryable": false
  },
  "meta": {
    "trace_id": "uuid-v4",
    "protocol_version": "1.0.0",
    "duration_ms": 45
  }
}
```

## Next Steps

- See [commands.md](commands.md) for the complete CLI reference
- Run `querylex --help` to see all available commands
- Run `querylex <command> --help` for command-specific usage

## Troubleshooting

### "No active database"

Run `querylex add-db` to add and activate a database connection.

### "Database is not indexed"

After adding a database, indexing runs automatically. If it failed, re-add the database or check `~/.querylex/logs/` for error details.

### Connection failures

- Verify the database is running and accessible from your machine
- Check credentials with `querylex workspace-stats --human`
- For SQLite, ensure the file path is correct and readable

### Keychain unavailable (headless Linux)

QueryLex falls back to an encrypted file store. You'll be prompted for a passphrase during `add-db`. Set `QUERYLEX_KEYCHAIN_PASSPHRASE` for non-interactive use.
