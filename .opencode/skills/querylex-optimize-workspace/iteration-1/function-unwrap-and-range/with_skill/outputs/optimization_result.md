# SQL Optimization Result

**Active Database:** TPC-H MySQL (`tpch-mysql`, type: `mysql`, status: `indexed`)  
**Source:** `benchmarks/tpch/init/mysql/01-schema.sql`, `03-indexes.sql`  
**Table:** `lineitem` (~6,001,215 rows at SF=1)  
**Note:** Database container was not running at analysis time. Analysis performed using available schema/index definitions and the QueryLex skill methodology. Live validation with `querylex validate` and `querylex explain` should be run once the database is available to confirm plan improvements.

---

## Original SQL

```sql
SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31'
GROUP BY DATE(l_shipdate)
ORDER BY ship_day
```

---

## Optimized SQL

```sql
SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty
FROM lineitem
WHERE l_shipdate >= '1996-01-01' AND l_shipdate <= '1996-12-31'
GROUP BY l_shipdate
ORDER BY l_shipdate
```

---

## What Changed

| # | Change | Reason |
|---|--------|--------|
| 1 | **Removed `DATE()` wrapping** in SELECT, WHERE, and GROUP BY | `l_shipdate` is already a `DATE NOT NULL` column. The `DATE()` function is a no-op on DATE-typed values but MySQL's optimizer treats it as a function call, making the predicate **non-sargable** and preventing the use of `idx_lineitem_shipdate`. |
| 2 | **Converted `BETWEEN '1996-01-01' AND '1996-12-31'`** to `l_shipdate >= '1996-01-01' AND l_shipdate <= '1996-12-31'` | Explicit range conditions are unambiguous and work directly with B-tree index range scans. No semantic difference — both evaluate to the same date range. |
| 3 | **Changed `ORDER BY ship_day`** to `ORDER BY l_shipdate` | Since `ship_day` is now an alias for the bare `l_shipdate` column, referencing the column directly allows the optimizer to recognize that GROUP BY and ORDER BY match the index sort order, enabling a loose index scan. |

---

## Plan Comparison (Expected)

| Metric | Before (Original) | After (Optimized) |
|--------|-------------------|-------------------|
| **Access method** | Full table scan on `lineitem` (~6M rows) | Range scan on `idx_lineitem_shipdate` |
| **Rows examined** | ~6,001,215 | ~365 date-bucket lookups from index |
| **Index used** | None (`NULL` in EXPLAIN key column) | `idx_lineitem_shipdate` |
| **GROUP BY method** | Using temporary; Using filesort | Using index for group-by (loose index scan) |
| **ORDER BY method** | Using filesort | Using index (implicit from group-by) |
| **Estimated cost** | High (sequential scan cost × 6M rows) | Low (B-tree range seek + index-only group-by) |
| **Heuristics warnings** | `FULL_TABLE_SCAN`, `NON_SARGABLE_PREDICATE` | None |

### Why the Index Was Not Used Before

`idx_lineitem_shipdate` is a B-tree index on `l_shipdate` (line 12 of `03-indexes.sql`):

```sql
CREATE INDEX idx_lineitem_shipdate ON lineitem(l_shipdate);
```

The original query wraps the column in `DATE(l_shipdate)`. Even though `l_shipdate` is already a `DATE` type, MySQL's query planner evaluates `DATE(l_shipdate)` as `DATE(l_shipdate)` — a function of the column — not `l_shipdate` itself. Index scans require a bare column reference (or a leading substring of a composite index). Function-wrapped columns require the engine to evaluate the function on every row, forcing a full table scan.

---

## Strategy Used: Strategy 1 — Predicate and Projection Rewrite

From the skill workflow, Strategy 1 targets sargable predicate repair and function-unwrapping:

> "Replace `WHERE YEAR(date_col) = 2020` with `WHERE date_col >= '2020-01-01' AND date_col < '2021-01-01'`"

The same principle applies: `DATE(l_shipdate) BETWEEN ...` → `l_shipdate >= ... AND l_shipdate <= ...`. This makes the filter sargable and allows the B-tree index on `l_shipdate` to drive the query with a range scan.

---

## Index Recommendation (Step 11)

If query performance is still insufficient after enabling the index range scan, consider a covering index for this specific aggregation pattern:

```sql
CREATE INDEX idx_lineitem_shipdate_qty ON lineitem(l_shipdate, l_quantity);
```

**Target:** Eliminates the need to read `l_quantity` from the clustered index (PK) after the secondary index lookup. With this composite index, the entire query — filter on `l_shipdate`, GROUP BY on `l_shipdate`, and SUM on `l_quantity` — can be satisfied entirely from the index, with zero table access (a **covering index scan**).

**Expected impact:** Further reduction in I/O, as the engine reads only the index pages instead of index pages + table pages. The `COUNT(*)` becomes an index entry count rather than a row count.

**Warning:** As always, test any new index in a non-production environment before deploying. Composite indexes add write overhead and storage cost.

---

## Context Coverage

| Source | Status |
|--------|--------|
| Schema (`01-schema.sql`) | Available — `l_shipdate DATE NOT NULL` confirmed |
| Indexes (`03-indexes.sql`) | Available — `idx_lineitem_shipdate ON lineitem(l_shipdate)` confirmed |
| Stats (`querylex stats`) | Not available — database offline |
| Joins (`querylex joins`) | N/A — single-table query, no join analysis needed |
| Validate (`querylex validate`) | Not available — database offline |
| Explain (`querylex explain`) | Not available — database offline |

**Warning:** Plan comparison values are estimated based on known schema/indexes and the expected MySQL optimizer behavior. Run `querylex validate` and `querylex explain` against the live database to confirm.

---

## Recommended Verification Commands

Once the TPC-H MySQL container is running:

```bash
export QUERYLEX_DB_PASSWORD=tpch
export QUERYLEX_KEYCHAIN_PASSPHRASE=tpch-passphrase

# Validate the optimized SQL
querylex validate "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate <= '1996-12-31' GROUP BY l_shipdate ORDER BY l_shipdate"

# Compare explain plans
querylex explain "SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31' GROUP BY DATE(l_shipdate) ORDER BY ship_day"

querylex explain "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate <= '1996-12-31' GROUP BY l_shipdate ORDER BY l_shipdate"

# Save the optimization to memory
querylex save "SELECT DATE(l_shipdate) AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE DATE(l_shipdate) BETWEEN '1996-01-01' AND '1996-12-31' GROUP BY DATE(l_shipdate) ORDER BY ship_day" "SELECT l_shipdate AS ship_day, COUNT(*) AS order_count, SUM(l_quantity) AS total_qty FROM lineitem WHERE l_shipdate >= '1996-01-01' AND l_shipdate <= '1996-12-31' GROUP BY l_shipdate ORDER BY l_shipdate"
```
