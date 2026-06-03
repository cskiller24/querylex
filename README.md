# Querylex

**AI-augmented SQL generation and optimization CLI** — Querylex generates dialect-correct SQL from natural language using live database context (schema, terminology, joins, statistics, indexes) and optimizes existing SQL via plan comparison and rewrite heuristics. Supports MySQL, MariaDB, PostgreSQL, SQLite, and Microsoft SQL Server.

## Installation

### macOS

```bash
brew install querylex/querylex/querylex
```

### Linux

Download the `.deb` or `.rpm` package for your architecture from the [latest GitHub Release](https://github.com/cskiller24/querylex/releases/latest):

**Debian/Ubuntu (.deb):**
```bash
sudo dpkg -i querylex_*.deb
```

**Fedora/RHEL (.rpm):**
```bash
sudo rpm -i querylex_*.rpm
```

### Windows

```bash
scoop bucket add querylex https://github.com/querylex/scoop-bucket
scoop install querylex
```

### Manual (any platform)

1. Download the archive for your platform and architecture from the [latest GitHub Release](https://github.com/cskiller24/querylex/releases/latest).
2. Extract it:
   ```bash
   tar -xzf querylex_*.tar.gz    # Linux/macOS
   unzip querylex_*.zip          # Windows
   ```
3. Move the binary to a directory on your `PATH`:
   ```bash
   sudo mv querylex*/querylex /usr/local/bin/
   sudo mv querylex*/querylex-add-db /usr/local/bin/
   sudo mv querylex*/querylex-stats /usr/local/bin/
   ```

## Shell Completions

### bash

```bash
source <(querylex completion bash)
echo "source <(querylex completion bash)" >> ~/.bashrc
```

### zsh

```bash
source <(querylex completion zsh)
echo "source <(querylex completion zsh)" >> ~/.zshrc
```

### fish

```bash
querylex completion fish | source
echo "querylex completion fish | source" >> ~/.config/fish/config.fish
```

### PowerShell

```powershell
querylex completion powershell | Out-String | Invoke-Expression
```

If installed via package manager, completions may be installed automatically — check your package manager's documentation.

## Getting Started

```bash
# 1. Add a database connection
querylex-add-db

# 2. Check workspace status
querylex-stats

# 3. Generate SQL from natural language
querylex sql "show me all users who ordered in the last month"

# 4. Optimize a SQL query
querylex optimize "SELECT * FROM orders JOIN users ON orders.user_id = users.id"
```

## Quick Example

```bash
# Add your MySQL database
querylex-add-db
# → Follow the guided prompts to connect to your database

# See your workspace overview
querylex-stats

# Ask a natural language question
querylex sql "list all products with their total sales amount"

# Optimize a slow query
querylex optimize "SELECT p.name, SUM(oi.quantity * oi.unit_price) as total
                   FROM products p
                   LEFT JOIN order_items oi ON p.id = oi.product_id
                   GROUP BY p.id, p.name"
```

## Documentation

For full documentation including command reference, response format specification, and advanced usage, see [QUERYLEX.md](./QUERYLEX.md).

## E2E Testing

QueryLex uses Docker Compose for end-to-end testing against real database instances.
Tests live under `test/{engine}/` and use the `//go:build e2e` build tag.

### Prerequisites

- Docker and Docker Compose v2
- Go 1.26.3+
- `make ci-setup` installs `gotestsum` (for JUnit XML output in CI)

### Running Tests Locally

```bash
# Single engine (MySQL example)
make test-e2e-mysql

# Available engine targets:
make test-e2e-postgresql
make test-e2e-mariadb
make test-e2e-mssql
make test-e2e-sqlite    # No Docker required (in-process SQLite)

# All engines sequentially (may be resource-intensive)
make test-e2e-all

# Cross-engine SQL validation matrix (requires all DBs running)
make test-e2e-cross-engine
```

### Golden Files

Golden files live in `test/testdata/golden/{engine}/`. They capture expected
JSON output for schema extraction and EXPLAIN plans. To regenerate:

```bash
go test -tags e2e -run TestMySQLGolden -update
```

### CI Pipeline

On push/PR to `main`, GitHub Actions runs `e2e.yml`:
- **5 parallel matrix jobs**: MySQL, PostgreSQL, MariaDB, MSSQL, SQLite
- **MSSQL**: Uses a pre-built Docker image with AdventureWorksLT (built in CI, cached via buildx)
- **SQLite**: Runs in-process, no Docker
- **JUnit XML**: Test results uploaded as CI artifacts per engine
- **`fail-fast: false`**: One engine failure does not cancel others

### Adding a New Engine

1. Add a Docker Compose service under a new profile in `compose.yaml`
2. Add `Connect{Engine}` function in `test/testhelper/connect.go`
3. Create `test/{engine}/` package with 7 test files + 1 setup file (mirror `test/mysql/`)
4. Add `compose-up-{engine}` and `test-e2e-{engine}` targets in `Makefile`
5. Add the engine to the CI matrix in `.github/workflows/e2e.yml`
