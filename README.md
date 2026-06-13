# Querylex

**Database introspection and query analysis CLI** — QueryLex introspects database schemas, validates SQL, analyzes execution plans, resolves natural language to table/column candidates, and maintains workspace state across multiple database connections. Supports MySQL, and PostgreSQL.

## Quick Start

1. Install querylex 
```bash
npm install -g cskiller24/querylex && querylex
```

2. Generate encryption keys
```bash
querylex encrypt
```

2. Add a database connection:
```bash
querylex add-db
```

3. Install skills
```bash
npx skills add cskiller24/querylex
```

4. Generate SQL using the skill:
```txt
/querylex-sql "Find the top 5 customers by total spend in the last month"
```

5. You can check your database stats with:
```bash
querylex workspace-stats
```

## Other Installations

### npm (all platforms — recommended)

```bash
npm install -g cskiller24/querylex
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
   ```

## Other useful commands

- Check all commands
```bash
querylex --help
```

- View list of database connections:

```bash
querylex list-dbs
```

- View workspace stats:
```bash
querylex workspace-stats --human
```

- Select active database connection:
```bash
querylex use-db
```

- Remove a database connection:
```bash
querylex remove-db
```

If installed via package manager, completions may be installed automatically — check your package manager's documentation.

## Documentation

For full documentation including command reference, response format specification, and advanced usage, see [docs folder](./docs/).


