# TPC-H Benchmark Setup with QueryLex

TPC-H benchmark database on MySQL 8.0 and PostgreSQL 16, managed via Docker Compose and integrated with QueryLex.

## Quick Start

```bash
# 1. Generate TPC-H data (SF=1, ~1GB)
make generate

# 2. Start databases and load data
make up

# 3. Wait for MySQL to finish loading (~2-5 min for SF=1)
docker compose logs -f mysql
# Stop watching when you see: "ready for connections" and "ANALYZE TABLE" output

# 4. Verify row counts
docker compose exec mysql mysql -utpch -ptpch -e "
  SELECT 'region',   COUNT(*) FROM tpch.region   UNION ALL
  SELECT 'nation',   COUNT(*) FROM tpch.nation   UNION ALL
  SELECT 'part',     COUNT(*) FROM tpch.part     UNION ALL
  SELECT 'supplier', COUNT(*) FROM tpch.supplier UNION ALL
  SELECT 'partsupp', COUNT(*) FROM tpch.partsupp UNION ALL
  SELECT 'customer', COUNT(*) FROM tpch.customer UNION ALL
  SELECT 'orders',   COUNT(*) FROM tpch.orders   UNION ALL
  SELECT 'lineitem', COUNT(*) FROM tpch.lineitem
"
```

## Expected Row Counts (SF=1)

| Table     | Rows       |
|-----------|------------|
| region    | 5          |
| nation    | 25         |
| part      | 200,000    |
| supplier  | 10,000     |
| partsupp  | 800,000    |
| customer  | 150,000    |
| orders    | 1,500,000  |
| lineitem  | 6,001,215  |

## Connection Details

|          | MySQL          | PostgreSQL       |
|----------|----------------|------------------|
| Host     | `127.0.0.1`    | `127.0.0.1`      |
| Port     | `3307`         | `5433`           |
| Database | `tpch`         | `tpch`            |
| User     | `tpch`         | `tpch`            |
| Password | `tpch`         | `tpch`            |

## Add to QueryLex

### Non-interactive (recommended for scripting)

```bash
export QUERYLEX_DB_PASSWORD=tpch
export QUERYLEX_KEYCHAIN_PASSPHRASE=tpch-passphrase

# MySQL
./querylex add-db --type mysql --name tpch-mysql \
  --host 127.0.0.1 --port 3307 \
  --database tpch --username tpch

# PostgreSQL (optional)
./querylex add-db --type postgresql --name tpch-postgres \
  --host 127.0.0.1 --port 5433 \
  --database tpch --username tpch
```

### Interactive

```bash
./querylex add-db
```

Walk through prompts:
```
Database type: MySQL / MariaDB
Display name: tpch-mysql
Host: 127.0.0.1
Port: 3307
Database name: tpch
Username: tpch
Password: tpch
SSL mode: prefer
```

## Verify Setup

```bash
# Check connection health and indexing status
./querylex workspace-stats --human

# Validate SQL against the active database
./querylex validate "SELECT * FROM region"

# Explore schema
./querylex schema --table lineitem --table orders
./querylex joins --table orders
./querylex stats --table lineitem
```

## Schema Overview

8 tables with TPC-H standard relationships:

```
region ──< nation ──< customer ──< orders ──< lineitem
                    ──< supplier ──< partsupp ──< lineitem
                                   ──< part ──< partsupp
```

Foreign keys and secondary indexes are created by `init/mysql/03-indexes.sql`.

## Management

```bash
# Stop containers (preserves data)
make down

# Tear down everything (removes volumes)
make clean

# Regenerate at a different scale factor
make generate SCALE_FACTOR=5
```

## Troubleshooting

**Docker Desktop not running:**
Start Docker Desktop and ensure WSL 2 integration is enabled in Settings > Resources > WSL Integration.

**MySQL loading stuck:**
Check `secure_file_priv` is empty in `conf/mysql.cnf`. Verify `.tbl` files exist in `data/` with `|` delimiters and no trailing `|` (generator's `sed 's/|$//'` handles this).

**QueryLex credential errors:**
- On headless Linux, set `QUERYLEX_KEYCHAIN_PASSPHRASE` before running `add-db`
- Verify credentials with `querylex workspace-stats --human`
- Reset: `rm -rf ~/.querylex/` and re-add databases
