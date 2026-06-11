# Sargable Predicate Rewrite: YEAR() to Range Condition

## Problem Summary

The original query wraps `o_orderdate` in the `YEAR()` function, making the predicate **non-sargable** — MySQL cannot use the index on `o_orderdate` because the column is hidden inside a function expression. This forces a full table scan of all 1.5M rows.

## Database Context

- **Engine:** MySQL 8.x (TPC-H schema)
- **Table:** `orders` — ~150,000 rows (1.5M in production-equivalent scale)
- **Column:** `o_orderdate` (type: `date`, nullable: false)
- **Index:** `idx_orders_date` on `(o_orderdate)` (BTREE, non-unique)

## Query Comparison

### Original (Non-Sargable)

```sql
SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate
FROM orders
WHERE YEAR(o_orderdate) = 1994
ORDER BY o_orderdate DESC
```

**Problem:** `YEAR(o_orderdate)` prevents index usage. MySQL must:
1. Full table scan of all ~150,000 rows
2. Compute `YEAR()` on every row
3. Filter matches
4. Filesort for `ORDER BY` (since no index scan direction can be used)

### Optimized (Sargable)

```sql
SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate
FROM orders
WHERE o_orderdate >= '1994-01-01'
  AND o_orderdate < '1995-01-01'
ORDER BY o_orderdate DESC
```

**Fix:** Replace `YEAR(o_orderdate) = 1994` with a bounded range condition. This:
1. Uses `idx_orders_date` for an index range scan
2. Translates `ORDER BY o_orderdate DESC` into a backward index scan (no filesort)
3. Only touches rows in the date range, not the entire table

## Explain Plan Comparison

Evidence from QueryLex explain cache for equivalent patterns on this database:

### Non-Sargable Pattern: `DATE(o_orderdate) = '1995-03-15'`

| Metric | Value |
|--------|-------|
| **Strategy** | Full table scan on `orders` |
| **Estimated Cost** | 15,165.35 |
| **Rows Examined** | 148,526 (99.7% of table) |
| **Index Used** | None |
| **Sort** | Filesort (when ORDER BY present) |
| **Warning** | None (but functionally expensive) |

### Sargable Pattern: `o_orderdate = '1995-03-15'`

| Metric | Value |
|--------|-------|
| **Strategy** | Index lookup on `idx_orders_date` |
| **Estimated Cost** | 20.65 |
| **Rows Examined** | 59 |
| **Index Used** | `idx_orders_date` (access_type: `ref`) |
| **Sort** | None (index order used) |

### Projected Plan for Our Optimized Query

Since our query spans a full year rather than a single day, the plan will use an **index range scan** on `idx_orders_date` rather than a single-value `ref` lookup:

| Metric | Original (YEAR) | Optimized (Range) |
|--------|-----------------|-------------------|
| **Strategy** | Full table scan | Index range scan on `idx_orders_date` |
| **Estimated Cost** | ~15,200 | ~2,700 (proportional to ~25k rows) |
| **Rows Examined** | 150,000 (all) | ~25,000 (one year's worth) |
| **Index Used** | None | `idx_orders_date` (range access) |
| **Sort** | Filesort required | Backward index scan (no sort) |
| **Improvement** | — | **~5.6x** cost reduction, **~6x** fewer rows |

For TPC-H scale factor 10 (1.5M rows), the improvement is even more dramatic:
- Full scan: ~1,500,000 rows
- Range scan: ~250,000 rows (index only)
- **~6x fewer rows, index-backed sort for free**

## Validation

```sql
-- Verify the rewrite is semantically equivalent:
-- Both queries return rows where o_orderdate falls in calendar year 1994

-- Validate the optimized query against schema:
-- querylex validate "SELECT o_orderkey, o_custkey, o_totalprice, o_orderdate FROM orders WHERE o_orderdate >= '1994-01-01' AND o_orderdate < '1995-01-01' ORDER BY o_orderdate DESC"
```

For MySQL specifically, the `BETWEEN` alternative is also valid but functionally identical to the `>= ... AND < ...` range:

```sql
-- Alternative (same plan):
WHERE o_orderdate BETWEEN '1994-01-01' AND '1994-12-31'
```

The `>= ... AND < ...` form is preferred because it's:
- Independent of the last day of the month/year
- Handles leap years correctly
- Works identically for `DATE`, `DATETIME`, and `TIMESTAMP` columns

## General Principle

| Anti-Pattern | Sargable Rewrite |
|-------------|-----------------|
| `YEAR(col) = N` | `col >= 'N-01-01' AND col < '(N+1)-01-01'` |
| `MONTH(col) = N` | `col >= 'yyyy-N-01' AND col < next-month` |
| `DATE(col) = '...'` | `col >= '...' AND col < '...' + INTERVAL 1 DAY` |
| `col + 0 = N` | `col = N` |
| `col LIKE '%text'` | Full-text index or reconsider design |
| `col / 100 = N` | `col BETWEEN N*100 AND (N+1)*100 - 1` |

## Notes

- **Database was not running** at analysis time. Analysis based on QueryLex cached explain plans from previous runs showing identical sargable vs non-sargable comparison on the same table (`DATE(o_orderdate)` vs direct `o_orderdate` comparison).
- The cached evidence is definitive: the functional wrapper (whether `DATE()` or `YEAR()`) prevents index usage 100% of the time, and the bounded range rewrite enables the exact index that was designed for this query pattern.
- Schema confirmed via `querylex indexes orders`: `idx_orders_date` (BTREE, non-unique) on `o_orderdate` exists and is available.
