# Query Optimization Result

**Database:** TPC-H MySQL (`tpch-mysql`)  
**Dialect:** MySQL 8.0  
**Date:** 2026-06-11  
**Methodology:** QueryLex 12-step workflow (plan analysis via heuristic methodology — MySQL container was not running at analysis time)

---

## Original SQL

```sql
SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, COUNT(*) as order_count
FROM customer c
JOIN orders o ON c.c_custkey = o.o_custkey
WHERE o.o_orderdate >= '1995-01-01'
  AND c.c_mktsegment = 'BUILDING'
GROUP BY c.c_name, c.c_mktsegment, o.o_orderpriority
ORDER BY order_count DESC
LIMIT 100
```

---

## Context Summary

| Source | Status |
|--------|--------|
| Workspace | `tpch-mysql` active, status `indexed` |
| Schema | TPC-H standard: `customer` (150K), `orders` (1.5M) |
| Indexes | `idx_orders_custkey` (o_custkey), `idx_orders_date` (o_orderdate), `idx_customer_nationkey` (c_nationkey) |
| Joins | FK path: `customer.c_custkey` → `orders.o_custkey` |
| Stats | Row counts from TPC-H SF=1 (customer=150K, orders=1.5M) |
| Memory | No cached optimization found |

---

## Baseline Plan Analysis (Expected)

MySQL's optimizer would likely produce this plan for the original query:

```
1. Table scan on orders via idx_orders_date (range on o_orderdate >= '1995-01-01')
   → ~1.1M rows filtered from 1.5M (orders from 1995 onward at SF=1)
2. Nested loop join to customer via idx_orders_custkey / PRIMARY KEY
   → ~1.1M lookups into customer
3. Filter customer by c_mktsegment = 'BUILDING' (applied per-row during join)
4. GROUP BY c_name, c_mktsegment, o_orderpriority
   → Creates temporary table + filesort (Using temporary; Using filesort)
   → Groups ~100K+ rows (BUILDING segment ≈ 5% of 150K = ~7.5K customers × N orders)
5. ORDER BY COUNT(*) DESC → filesort on aggregate result
6. LIMIT 100
```

### Problems Identified

| Priority | Problem | Impact |
|----------|---------|--------|
| **P1** | `Using temporary; Using filesort` for GROUP BY across columns from two tables | Creates temp table on disk for intermediate grouping, major I/O cost |
| **P2** | No index on `customer.c_mktsegment` | Filtering 150K customers without index forces full scan lookups per joined row |
| **P3** | Large intermediate result before aggregation | ~1.1M order rows joined to ~150K customer rows before GROUP BY reduces the data volume |
| **P4** | GROUP BY includes `c_name` (VARCHAR, variable width) | Increases temp table size and makes index-based grouping impossible |

---

## Strategy Attempts

### Strategy 1: Predicate and Projection Rewrite

**Assessment:** The WHERE predicates are already sargable. `o_orderdate >= '1995-01-01'` can use `idx_orders_date`. `c_mktsegment = 'BUILDING'` is also sargable but lacks an index. No function wrapping exists. No `SELECT *` issue.

**Verdict:** No predicate rewrite improves this query. The bottleneck is structural (GROUP BY temp table).

### Strategy 2: Join and Subquery Rewrite

**Attempt 2A — Derived table for customer filter:**

```sql
SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, COUNT(*) as order_count
FROM (
  SELECT c_custkey, c_name, c_mktsegment
  FROM customer
  WHERE c_mktsegment = 'BUILDING'
) c
JOIN orders o ON c.c_custkey = o.o_custkey
WHERE o.o_orderdate >= '1995-01-01'
GROUP BY c.c_name, c.c_mktsegment, o.o_orderpriority
ORDER BY order_count DESC
LIMIT 100
```

**Assessment:** Marginally reduces rows joined (only BUILDING customers enter the join), but without an index on `c_mktsegment`, the derived table still performs a full scan on customer. The GROUP BY temp table problem persists. Not a meaningful improvement.

**Attempt 2B — STRAIGHT_JOIN from customer to orders:**

```sql
SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, COUNT(*) as order_count
FROM customer c
STRAIGHT_JOIN orders o ON c.c_custkey = o.o_custkey
WHERE c.c_mktsegment = 'BUILDING'
  AND o.o_orderdate >= '1995-01-01'
GROUP BY c.c_name, c.c_mktsegment, o.o_orderpriority
ORDER BY order_count DESC
LIMIT 100
```

**Assessment:** Forces MySQL to start from customer (150K full scan → filter to ~7.5K BUILDING rows), then join to orders. Still has GROUP BY temp table. Without an index on `c_mktsegment`, the customer scan is still expensive.

### Strategy 3: Aggregation and Query-Shape Rewrite ★ WINNER

**Optimized SQL:**

```sql
SELECT c.c_name, c.c_mktsegment, agg.o_orderpriority, agg.order_count
FROM customer c
JOIN (
  SELECT o_custkey, o_orderpriority, COUNT(*) AS order_count
  FROM orders
  WHERE o_orderdate >= '1995-01-01'
  GROUP BY o_custkey, o_orderpriority
) agg ON c.c_custkey = agg.o_custkey
WHERE c.c_mktsegment = 'BUILDING'
ORDER BY agg.order_count DESC
LIMIT 100
```

**What changed:**

1. **Pre-aggregate orders before the join.** The subquery groups orders by `(o_custkey, o_orderpriority)` directly, producing one row per customer+priority combination instead of one row per order. This reduces the result from ~1.1M order rows to ~50K–100K aggregated rows (one per customer+priority with date filter).

2. **The GROUP BY in the subquery works on a single table** (orders) with a date range filter using `idx_orders_date`. MySQL can use the index for the range scan and then group — still uses a temp table, but over a much smaller working set in a simpler single-table context.

3. **The outer query has no GROUP BY.** After joining the pre-aggregated data to customer, there is no aggregation left to do — each (customer, priority) pair already has its count. This eliminates the post-join temp table entirely.

4. **Semantic safety:** `c.custkey` is the PRIMARY KEY of customer, so `c_name` and `c_mktsegment` are functionally dependent. Grouping by `o_custkey` in the subquery is equivalent to grouping by `c_name, c_mktsegment` in the original, since each `o_custkey` maps to exactly one customer name and segment.

**Why this works better than Strategy 1–2 rewrites:**
- Moves aggregation before the join, radically reducing the join input size
- Eliminates the post-join GROUP BY (which was the primary temp-table bottleneck)
- The orders subquery can still use `idx_orders_date` for the range filter
- The remaining outer query is a simple filtered join + sort with LIMIT 100

---

## Plan Comparison (Expected)

### Baseline Plan (Original SQL)

```
id | select_type | table | type   | possible_keys                    | key              | rows    | Extra
1  | SIMPLE      | o     | range  | idx_orders_date,idx_orders_cust  | idx_orders_date  | ~1.1M   | Using index condition
1  | SIMPLE      | c     | eq_ref | PRIMARY                          | PRIMARY          | 1       | Using where
                                                        ↑ Using temporary; Using filesort ↑
Estimated cost:    ~100,000  (dominated by temp table sort on 1.1M joined rows)
Rows examined:     ~1.1M from orders + 1.1M customer lookups
Temp table:        Yes (on-disk, wide VARCHAR key for c_name)
Filesort:          Yes (for ORDER BY COUNT(*) DESC)
```

### Optimized SQL Plan (Expected)

```
id | select_type | table      | type   | possible_keys              | key              | rows    | Extra
1  | PRIMARY     | <derived2> | ALL    |                            |                  | ~70K    |
1  | PRIMARY     | c          | eq_ref | PRIMARY                    | PRIMARY          | 1       | Using where
2  | DERIVED     | o          | range  | idx_orders_date            | idx_orders_date  | ~1.1M   | Using where; Using temporary; Using filesort
                                                        ↑ temp table on subquery only ↑
Estimated cost:    ~35,000  (reduced join size, no outer GROUP BY)
Rows examined:     ~1.1M orders + ~70K derived join + ~7.5K customer rows
Temp table:        Yes (subquery only — smaller, single-table, no VARCHAR customer name)
Filesort:          Yes (but on pre-aggregated ~70K result set, not 1.1M)
```

### Key Metric Changes

| Metric | Baseline | Optimized | Delta |
|--------|----------|-----------|-------|
| Post-join GROUP BY temp table | Yes (1.1M rows, wide) | **Eliminated** | Major win |
| Join input size (non-customer) | ~1.1M raw order rows | ~70K aggregated rows | **~15x reduction** |
| Customer rows in join | ~1.1M lookups | ~7.5K filtered rows | **~150x reduction** |
| Sort input size for ORDER BY | ~100K grouped rows | ~5K-10K post-join rows | **~10x reduction** |
| Filesort location | Outer query (after join) | Subquery only | Better locality |

---

## Index Recommendation

The SQL rewrite addresses the temp-table bottleneck, but a missing index on `customer.c_mktsegment` remains a secondary concern. This index benefits both the original and optimized queries.

### Recommended Index

```sql
CREATE INDEX idx_customer_mktsegment_custkey ON customer(c_mktsegment, c_custkey);
```

**What it targets:**
- The `WHERE c.c_mktsegment = 'BUILDING'` filter — converts a full table scan (in the optimized query, or 1.1M individual PK lookups with WHERE pushdown in the baseline) into an index range scan
- The composite with `c_custkey` makes the index slightly covering, letting MySQL avoid a second PK lookup for the join column

**Expected impact:**
- `type: ALL` on customer → `type: ref` on idx_customer_mktsegment_custkey
- Customer filter cost reduced from ~150K row scan to ~7.5K index range scan
- Additional ~10–15% cost reduction on top of the SQL rewrite improvement

**Warning:** Create and test in a non-production environment first. Verify with `EXPLAIN` that the optimizer actually uses this index. On MySQL 8.0, run `ANALYZE TABLE customer` after creating the index to update histogram statistics.

### Optional: Composite Orders Index (if write overhead is acceptable)

```sql
CREATE INDEX idx_orders_date_custkey_priority ON orders(o_orderdate, o_custkey, o_orderpriority);
```

This would serve as a covering index for the pre-aggregation subquery, eliminating the need to read `orders` table rows at all (index-only scan). However, at 3 columns on a 1.5M-row table with frequent inserts, evaluate the write-amplification trade-off.

---

## Warnings

- **Database not accessible during analysis.** The explain plans above are expected/heuristic, not measured. Actual MySQL 8.0 optimizer behavior may differ depending on `innodb_buffer_pool_size`, `tmp_table_size`, `max_heap_table_size`, and statistics freshness. Run `EXPLAIN` and `EXPLAIN ANALYZE` against the live database to confirm.
- **Embeddings unavailable** (embeddings service not configured). Memory similarity used lexical matching only.
- **Stale artifacts risk is low** since TPC-H is a static benchmark schema.

---

## Summary

| Item | Detail |
|------|--------|
| **Winning strategy** | Strategy 3 — Aggregation and Query-Shape Rewrite |
| **Primary change** | Pre-aggregate orders by `(o_custkey, o_orderpriority)` inside a derived table before joining to customer |
| **Why it wins** | Eliminates the post-join GROUP BY temp table (the #1 bottleneck), reduces join input by ~15x |
| **Index recommendation** | `CREATE INDEX idx_customer_mktsegment_custkey ON customer(c_mktsegment, c_custkey)` |
| **Estimated cost reduction** | ~65% (100K → ~35K estimated cost units) |
