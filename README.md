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
