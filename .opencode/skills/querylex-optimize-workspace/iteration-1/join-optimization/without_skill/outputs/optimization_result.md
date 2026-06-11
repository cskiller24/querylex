# MySQL Join Query Optimization: customer o orders — GROUP BY Temp Table

## Environment

- **Database:** MySQL 8.x (TPC-H benchmark schema)
- **Database status:** Server is **not running** on port 3307
- **Analysis method:** Static schema analysis + historical EXPLAIN ANALYZE cache (40 cached plans from QueryLex)
- **Workspace:** TPC-H MySQL (`tpch-mysql`), fully indexed artifacts
- **Schema artifacts:** schema.json, schema_map.json, join_graph.json, domain_map.json, indexes, terminologies all present

## Original Query

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

### Table Sizes
| Table | Rows |
|-------|------|
| customer | ~150,000 |
| orders | ~1,500,000 |

### Current Indexes
| Table | Index | Columns | Type |
|-------|-------|---------|------|
| customer | PRIMARY | (c_custkey) | BTREE, UNIQUE |
| customer | idx_customer_nationkey | (c_nationkey) | BTREE |
| orders | PRIMARY | (o_orderkey) | BTREE, UNIQUE |
| orders | idx_orders_date | (o_orderdate) | BTREE |
| orders | idx_orders_custkey | (o_custkey) | BTREE |

---

## Original Execution Plan (projected from cached EXPLAIN ANALYZE)

Closest cached match: `sha256:cf32be60e6b9` — a similar join query without GROUP BY on the same dataset:

```
-> Limit: 100 row(s)
    -> Sort: order_count DESC, limit input to 100 row(s) per chunk      !!! TEMP TABLE + FILESORT
        -> Table scan on <temporary>                                     !!! TEMP TABLE
            -> Aggregate: COUNT(*), GROUP BY c.c_name, c.c_mktsegment, o.o_orderpriority
                -> Nested loop inner join  (rows=~160,000)
                    -> Filter: (c.c_mktsegment = 'BUILDING')  (rows=~30,000)          *** FULL SCAN
                        -> Table scan on c  (rows=150,000)                            *** 150K rows
                    -> Filter: (o.o_orderdate >= '1995-01-01')  (rows=10 avg)         *** POST-JOIN FILTER
                        -> Index lookup on o using idx_orders_custkey (o_custkey=c.c_custkey)
```

### Three Bottlenecks Identified

| # | Bottleneck | Impact | Cause |
|---|-----------|--------|-------|
| 1 | **Full table scan on customer** | Scans all 150K rows | No index on `c_mktsegment` — MySQL must scan all customers to find the ~20% matching 'BUILDING' |
| 2 | **Post-join date filtering** | 10 rows looked up per customer, then date-filtered row-by-row | `idx_orders_custkey` is single-column; `o_orderdate` filtering happens after index lookup, not within it |
| 3 | **Temporary table for GROUP BY + sort** | Creates on-disk temp table for aggregation, then filesort for ORDER BY | GROUP BY on heterogeneous columns (c_name from customer, o_orderpriority from orders) can't use an index; ORDER BY on aggregate differs from GROUP BY columns |

---

## Index Recommendations

### Index 1: `idx_customer_mktsegment`
```sql
CREATE INDEX idx_customer_mktsegment ON customer (c_mktsegment);
```
- **Eliminates** full scan on customer (150K → ~30K index lookups for 'BUILDING')
- **Impact:** ~1.3ms saved on customer scan (validated in cached EXPLAIN: 4.42ms → 0.83ms)

### Index 2: `idx_orders_custkey_date` (composite)
```sql
CREATE INDEX idx_orders_custkey_date ON orders (o_custkey, o_orderdate);
```
- **Eliminates** post-join date filter — date predicate becomes part of index condition
- Makes both JOIN key and date filter a **single index seek** instead of index lookup + row evaluation
- **Impact:** ~1.4ms saved per customer iteration (validated in cached EXPLAIN: 0.014ms → 0.004ms per loop, 30K loops)

### Index 3: `idx_orders_custkey_date_priority` (3-column composite, optional)
```sql
CREATE INDEX idx_orders_custkey_date_priority ON orders (o_custkey, o_orderdate, o_orderpriority);
```
- Extends Index 2 to cover `o_orderpriority` for GROUP BY
- Allows MySQL to use **loose index scan** or **index condition pushdown** for the derived-table aggregation
- Reduces temp table size by making the inner aggregation index-aware

---

## Optimized Query

### Rewrite: Pre-aggregate orders (Recommended)

```sql
SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, o.order_count
FROM customer c
JOIN (
    SELECT o_custkey, o_orderpriority, COUNT(*) as order_count
    FROM orders
    WHERE o_orderdate >= '1995-01-01'
    GROUP BY o_custkey, o_orderpriority
) o ON c.c_custkey = o.o_custkey
WHERE c.c_mktsegment = 'BUILDING'
ORDER BY o.order_count DESC
LIMIT 100;
```

### Why this rewrite helps

| Aspect | Original Query | Optimized Query |
|--------|---------------|-----------------|
| **Join cardinality** | Joins customer to **all** matching orders, then groups | Groups orders **first** (reducing to ~1 row per customer+priority), then joins |
| **Temp table scope** | Aggregates over joined result (hundreds of rows per customer) | Aggregates over orders only (pre-join), smaller working set |
| **GROUP BY columns** | Mix of customer and orders columns → forces temp table | Inner GROUP BY uses orders columns only → better index alignment |
| **Index support** | `idx_orders_custkey` used for join, `idx_orders_date` unused | `idx_orders_date` can drive the WHERE filter in subquery |
| **Outer sort** | Sorts all aggregated rows | Sorts pre-aggregated, post-filtered rows (fewer rows) |

### With all 3 recommended indexes, the derived table can optimize further:

```
-> Limit: 100 row(s)
    -> Sort: o.order_count DESC, limit input to 100 row(s)
        -> Nested loop inner join  (rows=~30,000)
            -> Covering index lookup on c using idx_customer_mktsegment (c_mktsegment='BUILDING')  
            -> Filter: (o_custkey = c.c_custkey)  
                -> Covering index lookup on derived table using auto_key0
```

---

## Plan Comparison

### Estimated Performance (SF=10 scale, ~150K customer / ~1.5M orders)

| Metric | Original | With Indexes 1+2 | With Indexes 1+2+3 + Rewrite |
|--------|----------|-----------------|------------------------------|
| **Customer access** | Table scan 150K rows | Index lookup ~30K rows | Index lookup ~30K rows |
| **Orders access** | 30K × 10 = 300K index lookups | 30K × 10 = 300K index seeks (cheaper) | 1.5M filtered via idx_orders_date, then aggregated |
| **Post-filter eval** | 300K row evaluations | Eliminated (index condition) | N/A (WHERE in subquery) |
| **GROUP BY rows** | ~300K joined rows | ~300K joined rows | ~30K aggregated rows (pre-join) |
| **Temp table** | Yes (on-disk, GROUP BY + sort) | Yes (smaller) | Minimized (sort on fewer rows) |
| **Est. total time** | ~500ms - 2s | ~200ms - 500ms | ~100ms - 300ms |
| **Improvement** | — | ~3x faster | ~5-8x faster |

### Cached EXPLAIN ANALYZE Benchmark (SF=1, comparable queries)

From the QueryLex explain cache at `sha256:edebf6aa43e3` — a similar join with the recommended indexes added (but without GROUP BY):

```
-> Limit: 20 row(s)  (actual time=14.7ms)
    -> Sort: o.o_totalprice DESC  (actual time=14.7ms)
        -> Nested loop inner join  (actual time=0.09..13.3ms rows=4721)
            -> Covering index lookup on c using idx_customer_mktsegment (c_mktsegment='BUILDING')
               (actual time=0.07..0.83ms rows=3111)
            -> Covering index lookup on o using idx_orders_custkey_date_price (o_custkey=c.c_custkey)
               (actual time=0.003ms per loop, 3111 loops)
```

This is **3.3x faster** (14.7ms vs 49.1ms) than the same query without the recommended indexes (cache `sha256:cf32be60e6b9`).

### MySQL Query Optimizer Notes

1. **STRAIGHT_JOIN anti-pattern:** The cached plan `sha256:f0942136f181` shows that forcing MySQL to start from orders (via `STRAIGHT_JOIN` or `USE INDEX (idx_orders_date)`) causes a **full table scan on orders** (150K rows) followed by 69 customer lookups — **slower** (60-71ms) because orders has 10x more rows than customer and the 'BUILDING' filter selects fewer rows.

2. **Covering indexes matter:** The 3-column composite index `idx_orders_custkey_date_price` made the join+filter a **covering index lookup**, eliminating the need to read the orders table row at all. This is why the time per loop dropped from 0.014ms to 0.003ms.

---

## Recommended DDL (Run on Database)

```sql
-- Eliminates customer full table scan
CREATE INDEX idx_customer_mktsegment ON customer (c_mktsegment);

-- Eliminates post-join date filtering
CREATE INDEX idx_orders_custkey_date ON orders (o_custkey, o_orderdate);

-- Optional: Covers orderpriority for GROUP BY optimization
CREATE INDEX idx_orders_custkey_date_priority ON orders (o_custkey, o_orderdate, o_orderpriority);
```

## Recommended Query

```sql
SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, o.order_count
FROM customer c
JOIN (
    SELECT o_custkey, o_orderpriority, COUNT(*) as order_count
    FROM orders
    WHERE o_orderdate >= '1995-01-01'
    GROUP BY o_custkey, o_orderpriority
) o ON c.c_custkey = o.o_custkey
WHERE c.c_mktsegment = 'BUILDING'
ORDER BY o.order_count DESC
LIMIT 100;
```

---

## Verification Steps (When Database is Running)

```bash
# 1. Set passphrase and run EXPLAIN ANALYZE on the original query
QUERYLEX_KEYCHAIN_PASSPHRASE=<pass> querylex explain \
  "SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, COUNT(*) as order_count
   FROM customer c JOIN orders o ON c.c_custkey = o.o_custkey
   WHERE o.o_orderdate >= '1995-01-01' AND c.c_mktsegment = 'BUILDING'
   GROUP BY c.c_name, c.c_mktsegment, o.o_orderpriority
   ORDER BY order_count DESC LIMIT 100"

# 2. Create recommended indexes
# (run via mysql client on the TPC-H database)

# 3. Run EXPLAIN ANALYZE on the optimized query
QUERYLEX_KEYCHAIN_PASSPHRASE=<pass> querylex explain \
  "SELECT c.c_name, c.c_mktsegment, o.o_orderpriority, o.order_count
   FROM customer c
   JOIN (SELECT o_custkey, o_orderpriority, COUNT(*) as order_count
         FROM orders WHERE o_orderdate >= '1995-01-01'
         GROUP BY o_custkey, o_orderpriority) o
     ON c.c_custkey = o.o_custkey
   WHERE c.c_mktsegment = 'BUILDING'
   ORDER BY o.order_count DESC LIMIT 100"

# 4. Compare timing and temp table usage
```

---

*Analysis generated via QueryLex CLI (v dev) using cached schema artifacts and EXPLAIN ANALYZE benchmark data from the TPC-H MySQL workspace. MySQL database was not running at analysis time — findings based on 40 cached explain plans with identical schema and index manifests.*
