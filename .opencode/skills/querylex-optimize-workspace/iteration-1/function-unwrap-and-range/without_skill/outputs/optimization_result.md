# MySQL Aggregation Query Optimization: DATE() Unwrap + Range Scan

## Original Query

```sql
SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31'
GROUP BY DATE(l_shipdate)
ORDER BY ship_day
```

## Schema Context (from QueryLex indexed artifacts)

**Table:** `lineitem` (TPC-H, ~6,000,000 rows)

| Column | Type | Nullable |
|---|---|---|
| l_orderkey | int | NO |
| l_partkey | int | NO |
| l_suppkey | int | NO |
| l_linenumber | int | NO |
| l_quantity | decimal(15,2) | NO |
| l_extendedprice | decimal(15,2) | NO |
| l_discount | decimal(15,2) | NO |
| l_tax | decimal(15,2) | NO |
| l_returnflag | char(1) | NO |
| l_linestatus | char(1) | NO |
| **l_shipdate** | **date** | **NO** |
| l_commitdate | date | NO |
| l_receiptdate | date | NO |
| l_shipinstruct | char(25) | NO |
| l_shipmode | char(10) | NO |
| l_comment | varchar(44) | NO |

**Indexes on lineitem:**

| Index Name | Columns | Unique |
|---|---|---|
| PRIMARY | l_orderkey, l_linenumber | No |
| idx_lineitem_orderkey | l_orderkey | No |
| idx_lineitem_partkey_suppkey | l_partkey, l_suppkey | No |
| **idx_lineitem_shipdate** | **l_shipdate** | **No** |

---

## Problem Analysis

### 1. Redundant `DATE()` wrapper (CRITICAL)

`l_shipdate` is already a `date` type column. The `DATE()` function:

- Does nothing useful (date column → date value is identity)
- **Prevents index usage** — MySQL cannot use `idx_lineitem_shipdate` when the column is wrapped in a function
- Forces a **full table scan** of all ~6,000,000 lineitem rows
- Also forces a **temporary table + filesort** for `GROUP BY DATE(l_shipdate)` since the function result has no index ordering

### 2. `BETWEEN` with date boundaries (MINOR)

While `BETWEEN '1996-01-01' AND '1996-12-31'` is semantically clear after removing `DATE()`, it still:
- Relies on MySQL interpreting the string literals as dates (implicit conversion)
- Using explicit range operators (`>= AND <`) is more explicit and avoids any edge cases with `BETWEEN`'s inclusive upper bound on datetime types

---

## Optimized Query

```sql
SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01'
GROUP BY l_shipdate
ORDER BY l_shipdate
```

---

## What Changed

| Aspect | Original | Optimized | Impact |
|---|---|---|---|
| **SELECT** | `DATE(l_shipdate)` | `l_shipdate` | Removes redundant function call; `l_shipdate` is already `date` type |
| **WHERE** | `DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31'` | `l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01'` | Bare column enables index range scan on `idx_lineitem_shipdate` |
| **GROUP BY** | `DATE(l_shipdate)` | `l_shipdate` | Allows MySQL to use index ordering for grouping, avoiding filesort |
| **ORDER BY** | `ship_day` (alias) | `l_shipdate` (explicit) | Direct reference to indexed column; identical semantics |

---

## Execution Plan Comparison

### Original (estimated)

```
Table scan: lineitem (~6,000,000 rows examined)
Type: ALL (full table scan)
Key: NULL — no index usable due to DATE() wrapper
Extra: Using where; Using temporary; Using filesort
Estimated cost: Very high — linear scan of all 6M rows
```

### Optimized (estimated)

```
Table scan: lineitem (limited range, ~365 days)
Type: range
Key: idx_lineitem_shipdate
Key length: 4 (date type = 3 bytes + nullable)
Extra: Using index condition; Using where (no temp, no filesort)
Rows examined: ~240,000 (approximately 365/3650 days = ~10% of total if uniformly distributed)
Estimated cost: Low — B-tree range scan + index-only grouping
```

---

## Performance Estimates

| Metric | Original | Optimized | Improvement |
|---|---|---|---|
| **Rows examined** | ~6,000,000 | ~240,000 (est.) | ~25x fewer |
| **Scan type** | Full table scan (ALL) | Index range scan (range) | Index seeks instead of sequential scan |
| **Sorting** | Filesort (temporary table) | Index-ordered (no sort) | Eliminates temp disk I/O |
| **Index used** | None | idx_lineitem_shipdate | Leverages existing index |
| **Disk I/O** | Full table read (~1.5GB) | Index scan + row reads (~60MB) | ~25x less I/O |

**Expected speedup:** 20-50x on 6M row table, depending on buffer pool state and storage engine.

---

## Validation Note

The MySQL database (`tpch-mysql` on `localhost:3307`) was not running during this analysis, so actual EXPLAIN output could not be generated via `querylex explain`. The analysis above is based on:

1. **QueryLex indexed schema artifacts** (`schema.json`, `schema_map.json`) — confirming `l_shipdate` is `date` type and `idx_lineitem_shipdate` exists
2. **MySQL optimizer behavior** — the rule that function-wrapped columns (`DATE(col)`, `YEAR(col)`, `LOWER(col)`, etc.) disable index usage is well-established MySQL behavior
3. **TPC-H dataset profile** — lineitem has ~6M rows for scale factor 1

To verify with a live database, run:

```bash
QUERYLEX_KEYCHAIN_PASSPHRASE="<passphrase>" querylex explain "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01' GROUP BY l_shipdate ORDER BY l_shipdate"
```

---

## Additional Optimization (if l_shipdate coverage is needed)

If this query is run frequently and the table grows, consider a covering index that includes the aggregated column:

```sql
CREATE INDEX idx_lineitem_shipdate_covering ON lineitem (l_shipdate, l_quantity);
```

This would allow **index-only scans** (no table access), reducing I/O further since MySQL can read `l_quantity` directly from the index.

---

Generated: 2026-06-11 | Source: QueryLex schema artifacts + MySQL optimization heuristics
