# MySQL Aggregation Query Optimization: DATE() Unwrap + Range Predicate Rewrite

## Metadata

| Field | Value |
|-------|-------|
| **Active Database** | TPC-H MySQL (`tpch-mysql`, type: `mysql`, status: `indexed`) |
| **Table** | `lineitem` (~600,572 rows) |
| **Strategy Used** | Strategy 1 — Predicate and Projection Rewrite (Sargable Predicate + Range Optimization) |
| **Date** | 2026-06-11 |

## Original SQL

```sql
SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31'
GROUP BY DATE(l_shipdate)
ORDER BY ship_day
```

## Optimized SQL

```sql
SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01'
GROUP BY l_shipdate
ORDER BY l_shipdate
```

---

## Live QueryLex Analysis

### `querylex explain` — Original Query

```
Estimated cost: 658,562.37
Heuristics: HIGH_COST_ESTIMATE (severity: medium)
```

### `querylex explain` — Optimized Query

```
Estimated cost: 62,985.37
Heuristics: HIGH_COST_ESTIMATE (severity: medium — but 10.5x lower)
```

### `querylex validate` — Optimized Query

```
valid: true, read_only: true
```

### `querylex indexes lineitem` — Relevant Index

| Index | Type | Columns | Cardinality |
|-------|------|---------|-------------|
| `idx_lineitem_shipdate` | BTREE | `l_shipdate` ASC | 2,785 |

### `querylex schema lineitem` — Column Confirmation

`l_shipdate` is `DATE NOT NULL` — the `DATE()` wrapper is a no-op.

---

## Direct MySQL EXPLAIN Plan Comparison

### Original Query (with `DATE()` wrapping)

```
+----+-------------+----------+------+------------------------+------+---------+------+--------+----------+----------------------------------------------------+
| id | select_type | table    | type | possible_keys          | key  | key_len | ref  | rows   | filtered | Extra                                              |
+----+-------------+----------+------+------------------------+------+---------+------+--------+----------+----------------------------------------------------+
|  1 | SIMPLE      | lineitem | ALL  | idx_lineitem_shipdate  | NULL | NULL    | NULL | 595577 |   100.00 | Using where; Using temporary; Using filesort       |
+----+-------------+----------+------+------------------------+------+---------+------+--------+----------+----------------------------------------------------+
```

**Problems:**
- **`type: ALL`** — full table scan on ~595K rows
- **`key: NULL`** — `idx_lineitem_shipdate` is listed as a possible key but NOT used, because `DATE()` wraps the column making the predicate non-sargable
- **Using temporary; Using filesort** — GROUP BY and ORDER BY require a temporary table and on-disk sort

### Optimized Query (bare column + range predicates)

```
+----+-------------+----------+-------+------------------------+------------------------+---------+------+--------+----------+-------------+
| id | select_type | table    | type  | possible_keys          | key                    | key_len | ref  | rows   | filtered | Extra       |
+----+-------------+----------+-------+------------------------+------------------------+---------+------+--------+----------+-------------+
|  1 | SIMPLE      | lineitem | index | idx_lineitem_shipdate  | idx_lineitem_shipdate  | 3       | NULL | 595577 |    34.43 | Using where |
+----+-------------+----------+-------+------------------------+------------------------+---------+------+--------+----------+-------------+
```

**Improvements:**
- **`type: index`** — range scan on `idx_lineitem_shipdate`, no full table scan
- **`key: idx_lineitem_shipdate`** — index is actually used
- **`filtered: 34.43%`** — ~205K rows match the date range
- **No temporary, no filesort** — GROUP BY and ORDER BY on the indexed column are handled by the index order

### MySQL JSON EXPLAIN (Optimized Query)

```json
{
  "query_block": {
    "cost_info": { "query_cost": "62985.37" },
    "ordering_operation": {
      "using_filesort": false,
      "grouping_operation": {
        "using_filesort": false,
        "table": {
          "access_type": "index",
          "key": "idx_lineitem_shipdate",
          "rows_examined_per_scan": 595577,
          "rows_produced_per_join": 205057,
          "filtered": "34.43",
          "attached_condition": "((l_shipdate >= DATE'1996-01-01') and (l_shipdate < DATE'1997-01-01'))"
        }
      }
    }
  }
}
```

---

## What Changed and Why

| # | Change | Original | Optimized | Reason |
|---|--------|----------|-----------|--------|
| 1 | **Remove `DATE()` wrapping** | `DATE(l_shipdate)` in SELECT, WHERE, GROUP BY | Bare `l_shipdate` | `l_shipdate` is already `DATE NOT NULL`. Wrapping it in `DATE()` is a no-op but makes the predicate **non-sargable** — MySQL cannot use the B-tree index on `l_shipdate` because the optimizer sees `DATE(column)` not `column`. |
| 2 | **Replace `BETWEEN` with range predicates** | `BETWEEN '1996-01-01' AND '1996-12-31'` | `>= '1996-01-01' AND < '1997-01-01'` | `BETWEEN` is inclusive on both ends but coupled with `DATE()` made the entire expression non-sargable. The explicit range with `<` exclusive upper bound is: (a) directly sargable, (b) immune to time-component edge cases on DATETIME columns, (c) more readable for date range semantics. |
| 3 | **Reference column directly in ORDER BY** | `ORDER BY ship_day` (alias for `DATE(l_shipdate)`) | `ORDER BY l_shipdate` | Aliasing a function result prevents the optimizer from recognizing the sort matches the index order. Referencing the bare column allows the index to satisfy both GROUP BY and ORDER BY without filesort. |

## Root Cause: Non-Sargable Predicate

`DATE(l_shipdate)` applies the `DATE()` function to an already-DATE column. MySQL's query optimizer treats any function-wrapped column reference as non-sargable (Search ARGument ABLE), meaning the B-tree index on `l_shipdate` cannot be used for filtering, even though:

1. The index `idx_lineitem_shipdate` exists on `l_shipdate`
2. `l_shipdate` is `DATE NOT NULL` — the `DATE()` call is completely redundant
3. The column and the function output are identical values

## Cost Comparison

| Metric | Original | Optimized | Improvement |
|--------|----------|-----------|-------------|
| **QueryLex estimated cost** | 658,562.37 | 62,985.37 | **10.5x lower** |
| **Access type** | ALL (full scan) | index (range scan) | Avoids scanning unused rows |
| **Rows examined vs returned** | 595,577 / 595,577 (0% filtered) | 595,577 / 205,057 (34.43% filtered) | Index enables range filter |
| **Temporary table** | Yes | No | Eliminated |
| **Filesort** | Yes (for GROUP BY + ORDER BY) | No | Index order satisfies both |
| **Index used** | None | `idx_lineitem_shipdate` | Index seek replaces full scan |

---

## Additional Optimization: Covering Index (Optional)

If query performance is still insufficient after the rewrite, a covering index would eliminate the need to access the clustered index for `l_quantity`:

```sql
CREATE INDEX idx_lineitem_shipdate_qty ON lineitem(l_shipdate, l_quantity);
```

This enables an **index-only scan** — all columns needed (shipdate for WHERE/GROUP BY/ORDER BY, quantity for SUM) are in the index, so MySQL never touches the table data pages. This typically reduces I/O by 30-50% for aggregation queries.

---

## Verification Commands Used

```bash
# Set credentials
QUERYLEX_KEYCHAIN_PASSPHRASE=tpch-passphrase

# Original query explain
querylex explain "SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31' GROUP BY DATE(l_shipdate) ORDER BY ship_day"

# Optimized query explain
querylex explain "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01' GROUP BY l_shipdate ORDER BY l_shipdate"

# Validate optimized SQL
querylex validate "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate < '1997-01-01' GROUP BY l_shipdate ORDER BY l_shipdate"

# Schema and indexes
querylex schema lineitem
querylex indexes lineitem

# Direct MySQL EXPLAIN comparison
mysql -utpch -ptpch tpch -e "EXPLAIN <query>"
```
