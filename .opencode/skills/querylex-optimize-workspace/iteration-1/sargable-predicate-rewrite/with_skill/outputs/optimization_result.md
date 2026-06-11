# Query Optimization Result

## Metadata

| Field | Value |
|-------|-------|
| **Active Database** | TPC-H MySQL (`tpch-mysql`, type: `mysql`, status: `indexed`) |
| **Strategy Used** | Strategy 1 — Predicate and Projection Rewrite (Sargable Predicate) |
| **Analysis Type** | Offline (DB not running; analysis based on cached index metadata + TPC-H schema knowledge) |
| **Date** | 2026-06-11 |

## Original SQL

```sql
SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate
FROM orders
WHERE YEAR(o_orderdate) = 1994
ORDER BY o_orderdate DESC
```

## Problem Analysis

### Root Cause: Non-Sargable Predicate

The function `YEAR(o_orderdate)` wraps the indexed column `o_orderdate`, which prevents MySQL from using `idx_orders_o_orderdate` (a BTREE index on `o_orderdate`). This makes the predicate **non-sargable** (Search ARGument ABLE).

### Index Context (from cached QueryLex data)

| Index | Columns | Type | Unique |
|-------|---------|------|--------|
| `PRIMARY` | `o_orderkey` ASC | BTREE | Yes |
| `idx_orders_o_orderdate` | `o_orderdate` ASC | BTREE | No |
| `idx_orders_o_custkey` | `o_custkey` ASC | BTREE | No |

### Table Context

| Table | Approx. Rows | Engine |
|-------|-------------|--------|
| `orders` | 1,500,000 (SF=1) | InnoDB |

### Baseline Plan (expected)

```
-> Sort: o_orderdate DESC  (filesort — costly, on-disk sort for 1.5M scanned rows)
   -> Filter: YEAR(o_orderdate) = 1994
      -> Table scan on orders  (full scan of ~1.5M rows)
```

**Key problems in baseline plan:**

1. **Full table scan on orders** — 1.5M rows scanned because MySQL must evaluate `YEAR()` on every row; the index on `o_orderdate` is bypassed
2. **Non-sargable predicate** — `YEAR(o_orderdate) = 1994` prevents index range scan, heuristic flag: `NON_SARGABLE_PREDICATE`
3. **Filesort for ORDER BY** — even though `idx_orders_o_orderdate` exists, the full scan forces a separate sort pass (potentially on-disk temp table)

### Heuristic Findings

| Heuristic | Severity | Detail |
|-----------|----------|--------|
| `FULL_TABLE_SCAN` | HIGH | 1.5M row full scan on `orders` — the `idx_orders_o_orderdate` index exists but is not usable due to `YEAR()` wrapping |
| `NON_SARGABLE_PREDICATE` | HIGH | `YEAR(o_orderdate)` prevents index usage on `o_orderdate` |
| `EXCESSIVE_SORTING` | MEDIUM | `ORDER BY o_orderdate DESC` requires filesort since index cannot be used for the scan |

## Optimized SQL

```sql
SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate
FROM orders
WHERE o_orderdate >= '1994-01-01'
  AND o_orderdate < '1995-01-01'
ORDER BY o_orderdate DESC
```

### What Changed

| Change | Before | After |
|--------|--------|-------|
| **Predicate** | `YEAR(o_orderdate) = 1994` | `o_orderdate >= '1994-01-01' AND o_orderdate < '1995-01-01'` |
| **Sargability** | Non-sargable (function on column) | Fully sargable (bare column with range operators) |
| **Index usage** | `idx_orders_o_orderdate` unusable | `idx_orders_o_orderdate` used for range scan |

### Why This Rewrite Works

1. **Range equivalence**: `YEAR(date_col) = 1994` is semantically equivalent to `date_col >= '1994-01-01' AND date_col < '1995-01-01'`. All dates in 1994 fall in the half-open interval [1994-01-01, 1995-01-01).

2. **Index usability**: With the bare column `o_orderdate` on both sides of the range operators, MySQL's optimizer can use `idx_orders_o_orderdate` (BTREE) for an **index range scan**. MySQL evaluates the range condition against the index B-tree structure directly — no function evaluation needed per row.

3. **Sort elimination**: Because the index is on `o_orderdate ASC` and the query sorts `DESC`, MySQL can traverse the index **backwards** during the range scan, producing rows in `o_orderdate DESC` order directly from the index. This eliminates the filesort entirely.

4. **Row reduction**: Instead of scanning 1.5M rows and filtering, the index range scan touches only the ~365/3650 ≈ 10% of rows that fall in the 1994 date range (assuming uniform date distribution over ~10 years for TPC-H). Expected row examination drops from 1.5M to ~150K.

## Plan Comparison

| Metric | Original (Expected) | Optimized (Expected) | Improvement |
|--------|--------------------|--------------------|-------------|
| **Access type** | `ALL` (full table scan) | `range` (index range scan) | Full scan eliminated |
| **Key used** | `NULL` | `idx_orders_o_orderdate` | Index enabled |
| **Rows examined** | ~1,500,000 | ~150,000 (est. 10%) | **~10x reduction** |
| **Extra (sort)** | `Using filesort` | `Backward index scan` | Filesort eliminated |
| **Using where** | Yes (evaluates YEAR per row) | Yes (range condition on index) | Predicate pushed to index |
| **Temp table** | Possible (on-disk filesort) | None | Eliminated |

### Expected EXPLAIN Output Comparison

**Original:**
```
+----+-------------+--------+------+---------------+------+---------+------+---------+-----------------------------+
| id | select_type | table  | type | possible_keys | key  | key_len | ref  | rows    | Extra                       |
+----+-------------+--------+------+---------------+------+---------+------+---------+-----------------------------+
|  1 | SIMPLE      | orders | ALL  | NULL          | NULL | NULL    | NULL | 1500000 | Using where; Using filesort |
+----+-------------+--------+------+---------------+------+---------+------+---------+-----------------------------+
```

**Optimized:**
```
+----+-------------+--------+-------+-------------------------+-------------------------+---------+------+--------+----------------------------------+
| id | select_type | table  | type  | possible_keys           | key                     | key_len | ref  | rows   | Extra                            |
+----+-------------+--------+-------+-------------------------+-------------------------+---------+------+--------+----------------------------------+
|  1 | SIMPLE      | orders | range | idx_orders_o_orderdate  | idx_orders_o_orderdate  | 4       | NULL | 150000 | Using index condition; Backward  |
+----+-------------+--------+-------+-------------------------+-------------------------+---------+------+--------+----------------------------------+
```

## Rewrite Strategies Tried

### Strategy 1: Predicate and Projection Rewrite (WINNER)

**Action**: Unwrap `YEAR()` by replacing with a sargable date range.

**Result**: Eliminates full table scan, enables `idx_orders_o_orderdate` range scan, eliminates filesort.

### Strategy 2: Join and Subquery Rewrite

**Not applicable** — the query is a single-table scan with no joins or subqueries.

### Strategy 3: Aggregation and Query-Shape Rewrite

**Not applicable** — the query has no aggregation, GROUP BY, HAVING, UNION, or window functions.

## Context Coverage

| Context Source | Status | Impact |
|----------------|--------|--------|
| **Schema** | Unavailable (DB offline) | Low — TPC-H schema is standard and well-known |
| **Indexes** | Available (cached) | Confirmed `idx_orders_o_orderdate` on `o_orderdate` |
| **Stats** | Unavailable (DB offline) | Medium — row counts inferred from TPC-H SF=1 spec |
| **Joins** | Unavailable (DB offline) | None — single-table query, no join context needed |

## Limitations

- **Database not running** at `127.0.0.1:3307` — live `EXPLAIN` and `EXPLAIN ANALYZE` could not be executed. The plan comparison above shows the **expected** behavior based on MySQL optimizer rules and the known index structure.
- **To verify**: start the TPC-H Docker containers (`make up` from `benchmarks/tpch/`), then run:
  ```bash
  querylex explain "SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate FROM orders WHERE o_orderdate >= '1994-01-01' AND o_orderdate < '1995-01-01' ORDER BY o_orderdate DESC"
  ```
- **Statistics** could not be verified — actual row estimates depend on `innodb_stats_persistent` and histogram availability.

## Appendix: Complete QueryLex Workflow Executed

```
# Step 2 — Preflight
querylex workspace-stats
→ Active: tpch-mysql (mysql, indexed)

# Step 3 — Memory check
querylex memory "<original SQL>"
→ No cached match (similarity scoring used lexical fallback)

# Step 4 — Validate
querylex validate "<original SQL>"
→ FAIL: DB not running — proceeded with offline analysis

# Step 5 — Explain
querylex explain "<original SQL>"
→ SKIPPED: DB not running

# Step 7 — Context
querylex indexes --table orders
→ SUCCESS (cached): idx_orders_o_orderdate confirmed on o_orderdate
querylex schema --table orders     → FAIL: DB not running
querylex stats --table orders      → FAIL: DB not running
querylex joins --table orders      → FAIL: DB not running

# Step 9 — Rewrite
Strategy 1: Predicate Rewrite (sargable)
→ Rewrite: YEAR() unwrapped to date range
→ Validate: SKIPPED (DB offline)
→ Explain: SKIPPED (DB offline)
→ Result: EXPECTED IMPROVEMENT — full scan → index range scan
```
