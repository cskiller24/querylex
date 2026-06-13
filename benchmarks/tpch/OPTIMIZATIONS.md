# TPC-H Index Optimization Reference

This document catalogues every index-related optimization opportunity in the TPC-H benchmark setup, organized as a basis for evaluating what makes a good index.

---

## Evaluation Rubric: What Makes a Good Index?

Score each candidate index against these dimensions on a 1–5 scale. Multiply by weight for a composite score.

| Dimension | Weight | Description |
|-----------|--------|-------------|
| **Query Impact** | ×3 | Measurable reduction in query latency or rows examined for the queries it accelerates. Requires EXPLAIN before/after. |
| **Coverage Breadth** | ×3 | How many TPC-H queries (out of 22) benefit. An index that accelerates 10+ queries scores higher than one that helps a single query. |
| **Storage Cost** | ×2 | Additional disk space consumed. Smaller indexes score higher (5 = negligible overhead, 1 = doubles table size). |
| **Write Penalty** | ×1.5 | Maintenance cost on INSERT/UPDATE/DELETE. Lower is better; irrelevant for a read-only benchmark, but still real. |
| **Selectivity** | ×2 | How selective the leading column(s) are. High-selectivity indexes (like PK lookups) score higher than low-selectivity ones (like status flags). |
| **Covering Potential** | ×2 | Can the index serve as a covering index (no secondary lookup needed) for key queries? A covering index eliminates clustered index reads entirely. |
| **Scalability** | ×1.5 | Does the index benefit grow with scale factor? An index that helps at SF=1 but becomes essential at SF=10 scores higher. |
| **Regret Risk** | ×2 | Likelihood the optimizer ignores the index or chooses a worse plan because of it. Lower risk scores higher (inverted: 1=high risk, 5=no risk). |

**Composite = Σ(dimension_score × weight)** — maximum possible: 85.

---

## Schema Changes That Affect Index Efficiency

Before creating new indexes, optimize the columns those indexes will be built on. Narrower columns mean narrower indexes, which means more rows per page and fewer I/O operations.

| # | Optimization | Current State | Proposed Change | Index Impact |
|---|-------------|---------------|-----------------|--------------|
| S1 | **CHAR → VARCHAR for indexed columns** | `CHAR(25)` on indexed columns: `r_name`, `n_name`, `s_name`, `p_mfgr`, `p_brand`, `p_type`, `p_container`, `c_mktsegment`, `o_orderpriority`, `o_clerk`, `l_shipinstruct`, `l_shipmode`, and all `CHAR(1)` status columns | Convert to `VARCHAR`. CHAR always pads to full width in indexes (e.g., `CHAR(25)` always uses 100 bytes in utf8mb4), while VARCHAR stores only actual bytes. | Indexes on these columns shrink 30–60%. `idx_supplier_nationkey` with CHAR(25) s_name wastes ~40 bytes per row × 10K rows. |
| S2 | **DECIMAL precision for indexed lineitem columns** | `DECIMAL(15,2)` on `l_extendedprice`, `l_discount`, `l_tax`, `l_quantity` — these appear in composite indexes (4.3, 4.4, 4.6) | Narrow to actual range: `l_discount DECIMAL(4,2)`, `l_tax DECIMAL(4,2)`, `l_quantity DECIMAL(2,0)`, `l_extendedprice DECIMAL(8,2)`. | Indexes 4.3 (covering Q1), 4.4 (covering Q3–Q12), 4.6 (covering Q6) all carry these DECIMAL values as trailing columns. Narrower DECIMAL reduces each index entry by 6–10 bytes × 6M rows = 36–60 MB per index. |
| S3 | **Generated column: `l_extendedprice * (1 - l_discount)`** | Expression computed per-query | Add `l_disc_price DECIMAL(12,2) GENERATED ALWAYS AS (l_extendedprice * (1 - l_discount)) STORED`. Then index it. | Enables a single-column covering index for the most common expression across TPC-H queries (Q1, Q3, Q4, Q5, Q6, Q7, Q8, Q9, Q10, Q12). Without this, the expression must be recomputed for every row, and cannot be used as a leading index column. |
| S4 | **Charset: `utf8mb4` → `latin1` for ASCII-only columns** | `utf8mb4` (up to 4 bytes/char) on all string columns | Use `CHARACTER SET latin1` for columns that are provably ASCII: `o_orderstatus`, `l_returnflag`, `l_linestatus`, `o_orderpriority`, `c_mktsegment`, `p_brand`, `p_container`, `r_name`, `n_name`. | Indexes on these columns immediately halve in width (1 byte/char vs up to 4). `idx_customer_mktsegment` on `c_mktsegment CHAR(10)` drops from 40 bytes to 10 bytes per entry. |

---

## Index Optimizations

The current setup (`03-indexes.sql`) creates 12 secondary indexes focused on FK support. TPC-H queries require additional indexes tuned for WHERE filters, JOIN conditions, GROUP BY clauses, and ORDER BY clauses.

### Current Index Inventory

| Table | Existing Indexes | Status |
|-------|-----------------|--------|
| `region` | `PRIMARY KEY (r_regionkey)` | Sufficient (5 rows) |
| `nation` | `PRIMARY KEY (n_nationkey)`, `idx_nation_regionkey (n_regionkey)` | Sufficient (25 rows); optional `n_name` |
| `part` | `PRIMARY KEY (p_partkey)` | **Under-indexed** — no indexes on `p_type`, `p_brand`, `p_size`, `p_container`, `p_retailprice` |
| `supplier` | `PRIMARY KEY (s_suppkey)`, `idx_supplier_nationkey (s_nationkey)` | **Under-indexed** — missing `s_acctbal` |
| `partsupp` | `PRIMARY KEY (ps_partkey, ps_suppkey)`, `idx_partsupp_suppkey (ps_suppkey)`, `idx_partsupp_partkey (ps_partkey)` | Sufficient |
| `customer` | `PRIMARY KEY (c_custkey)`, `idx_customer_nationkey (c_nationkey)` | **Under-indexed** — missing `c_mktsegment` |
| `orders` | `PRIMARY KEY (o_orderkey)`, `idx_orders_custkey (o_custkey)`, `idx_orders_date (o_orderdate)` | **Under-indexed** — missing composites like `(o_orderdate, o_orderstatus)` |
| `lineitem` | `PRIMARY KEY (l_orderkey, l_linenumber)`, `idx_lineitem_orderkey (l_orderkey)`, `idx_lineitem_partkey_suppkey (l_partkey, l_suppkey)`, `idx_lineitem_shipdate (l_shipdate)` | **Critically under-indexed** — the largest table (6M rows) lacks single-column indexes on `l_partkey` and `l_suppkey`, and has no covering indexes for its most common query patterns |

---

### Proposed Indexes

Each index below is tagged with the TPC-H queries it accelerates and scored against the evaluation rubric.

---

#### I-1: `idx_lineitem_shipdate_covering`

```sql
CREATE INDEX idx_lineitem_shipdate_covering
    ON lineitem(l_shipdate, l_partkey, l_suppkey,
                l_extendedprice, l_discount, l_quantity, l_tax,
                l_returnflag, l_linestatus);
```

**Accelerates:** Q3, Q4, Q5, Q6, Q7, Q8, Q9, Q10, Q12 (9+ queries)

**Rationale:** `l_shipdate` is the single most common range predicate across TPC-H. By placing it first and including the commonly-selected price/quantity columns as trailing columns, this becomes a covering index for any query of the form `WHERE l_shipdate BETWEEN ... AND ...` that selects price columns — avoiding the clustered index entirely. This is the highest-leverage single index in the entire benchmark.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 5 | ×3 | 15 | Dramatic reduction for the largest table's most common access pattern |
| Coverage Breadth | 5 | ×3 | 15 | 9+ queries out of 22 |
| Storage Cost | 3 | ×2 | 6 | ~150 MB at SF=1 (wide index on 6M rows) |
| Write Penalty | 4 | ×1.5 | 6 | Benchmark is read-only after load; irrelevant |
| Selectivity | 4 | ×2 | 8 | Date ranges typically select 1/7 of data (one year out of 7) |
| Covering Potential | 5 | ×2 | 10 | Fully covering for shipdate-range queries with price columns |
| Scalability | 5 | ×1.5 | 7.5 | Benefit grows linearly with table size |
| Regret Risk | 3 | ×2 | 6 | Wide index; optimizer may prefer PK for small date ranges |
| **Total** | | | **73.5 / 85** | |

---

#### I-2: `idx_lineitem_returnflag_covering` (Q1-specific)

```sql
CREATE INDEX idx_lineitem_returnflag_covering
    ON lineitem(l_returnflag, l_linestatus, l_shipdate,
                l_discount, l_quantity, l_extendedprice);
```

**Accelerates:** Q1

**Rationale:** Q1 is unique — it groups by `(l_returnflag, l_linestatus)` and aggregates price columns, with a `l_shipdate <= ...` filter. A covering index ordered by the GROUP BY columns eliminates both the sort and the clustered index lookup. This is a classic "covering index for a GROUP BY" pattern.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 5 | ×3 | 15 | Q1 is one of the slowest queries; full lineitem scan unavoidable |
| Coverage Breadth | 1 | ×3 | 3 | Only Q1 |
| Storage Cost | 3 | ×2 | 6 | ~120 MB |
| Write Penalty | 4 | ×1.5 | 6 | Read-only benchmark |
| Selectivity | 2 | ×2 | 4 | `l_returnflag` has only 3 values (A, N, R) — low selectivity |
| Covering Potential | 5 | ×2 | 10 | Fully covering for Q1 |
| Scalability | 5 | ×1.5 | 7.5 | Scales with lineitem size |
| Regret Risk | 4 | ×2 | 8 | Narrow use case means low interference with other queries |
| **Total** | | | **59.5 / 85** | |

---

#### I-3: `idx_lineitem_partkey`

```sql
CREATE INDEX idx_lineitem_partkey ON lineitem(l_partkey);
```

**Accelerates:** Q14, Q17, Q20

**Rationale:** The existing composite `idx_lineitem_partkey_suppkey (l_partkey, l_suppkey)` can serve Q14 and Q17 (which join on `l_partkey = p_partkey`), but it includes `l_suppkey` as the second column, making the index wider than necessary. A single-column `l_partkey` index is narrower and sufficient for the join. Q20 uses `l_partkey` in a correlated subquery and currently has no usable index for it.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Moderate improvement; existing composite partially covers this |
| Coverage Breadth | 3 | ×3 | 9 | 3 queries |
| Storage Cost | 5 | ×2 | 10 | ~40 MB, narrow |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 5 | ×2 | 10 | High selectivity — partkey is nearly unique per lineitem |
| Covering Potential | 1 | ×2 | 2 | Not covering alone; needs PK lookup for other columns |
| Scalability | 4 | ×1.5 | 6 | Scales well |
| Regret Risk | 4 | ×2 | 8 | May be redundant with existing composite; optimizer may use either |
| **Total** | | | **61.5 / 85** | |

---

#### I-4: `idx_lineitem_suppkey`

```sql
CREATE INDEX idx_lineitem_suppkey ON lineitem(l_suppkey);
```

**Accelerates:** Q5, Q7, Q11, Q15, Q16, Q20 (6 queries)

**Rationale:** Multiple queries join lineitem to supplier on `l_suppkey = s_suppkey`. Currently there is no single-column index on `l_suppkey` — the closest is the composite `(l_partkey, l_suppkey)` which cannot be used for `l_suppkey`-only lookups because `l_partkey` is the leading column.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 4 | ×3 | 12 | Strong improvement for supplier-side joins |
| Coverage Breadth | 4 | ×3 | 12 | 6 queries |
| Storage Cost | 5 | ×2 | 10 | ~40 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 4 | ×2 | 8 | 10K distinct suppliers across 6M lineitems = 600 rows avg per supplier |
| Covering Potential | 1 | ×2 | 2 | Not covering alone |
| Scalability | 4 | ×1.5 | 6 | Scales well |
| Regret Risk | 4 | ×2 | 8 | Straightforward single-column index |
| **Total** | | | **65.5 / 85** | |

---

#### I-5: `idx_lineitem_receiptdate`

```sql
CREATE INDEX idx_lineitem_receiptdate
    ON lineitem(l_receiptdate, l_commitdate, l_shipdate);
```

**Accelerates:** Q10, Q11

**Rationale:** Q10 filters on `l_receiptdate = ...` and Q11 uses `l_receiptdate > l_commitdate` as a filter condition. An index starting with `l_receiptdate` enables index range scans for both patterns.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Benefits 2 queries; Q11's inequality filter (`>`) still requires row evaluation |
| Coverage Breadth | 2 | ×3 | 6 | 2 queries |
| Storage Cost | 4 | ×2 | 8 | ~60 MB (3 DATE columns) |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | Date ranges are moderately selective |
| Covering Potential | 2 | ×2 | 4 | Not fully covering |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 4 | ×2 | 8 | Narrow use, low interference |
| **Total** | | | **54.5 / 85** | |

---

#### I-6: `idx_lineitem_discount_quantity` (Q6-specific)

```sql
CREATE INDEX idx_lineitem_discount_quantity
    ON lineitem(l_discount, l_quantity, l_extendedprice);
```

**Accelerates:** Q6

**Rationale:** Q6 filters on `l_discount BETWEEN 0.05 AND 0.07 AND l_quantity < 24 AND l_shipdate >= ... AND l_shipdate < ...`. An index on `(l_discount, l_quantity)` with `l_extendedprice` as trailing enables a range scan on discount with further filter on quantity, and covers the selected column.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 4 | ×3 | 12 | Q6 scans ~20% of lineitem; index reduces to exact match range |
| Coverage Breadth | 1 | ×3 | 3 | Only Q6 |
| Storage Cost | 4 | ×2 | 8 | ~60 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | Discount filter selects ~20% of rows |
| Covering Potential | 3 | ×2 | 6 | Covers `l_extendedprice` but still needs `l_shipdate` filter |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 3 | ×2 | 6 | May conflict with I-1 (shipdate) for Q6 |
| **Total** | | | **54.5 / 85** | |

---

#### I-7: `idx_orders_date_status`

```sql
CREATE INDEX idx_orders_date_status
    ON orders(o_orderdate, o_orderstatus);
```

**Accelerates:** Q3, Q4, Q5, Q7, Q8, Q12 (6 queries)

**Rationale:** Many queries filter on `o_orderdate >= ... AND o_orderstatus = 'O'` (or 'F'). A composite index with date first and status second enables efficient range scan + filter. The existing `idx_orders_date` only covers the date predicate.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 4 | ×3 | 12 | Strong improvement for 6 queries on the second-largest table |
| Coverage Breadth | 4 | ×3 | 12 | 6 queries |
| Storage Cost | 5 | ×2 | 10 | ~20 MB on 1.5M rows |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 4 | ×2 | 8 | Date selective, status adds further filtering |
| Covering Potential | 2 | ×2 | 4 | Not covering (queries need o_orderkey, o_custkey, o_totalprice) |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 4 | ×2 | 8 | Clean composite, optimizer-friendly |
| **Total** | | | **67.5 / 85** | |

---

#### I-8: `idx_orders_date_priority`

```sql
CREATE INDEX idx_orders_date_priority
    ON orders(o_orderdate, o_orderpriority);
```

**Accelerates:** Q4, Q5

**Rationale:** Q4 and Q5 additionally filter on `o_orderpriority`. While I-7 covers the date+status pattern, these queries need date+priority. Could be combined with I-7 into a 3-column index, but separate indexes give the optimizer better choices.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Benefits 2 queries |
| Coverage Breadth | 2 | ×3 | 6 | 2 queries |
| Storage Cost | 5 | ×2 | 10 | ~25 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | Priority has 5 values; moderate |
| Covering Potential | 2 | ×2 | 4 | Not covering |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 3 | ×2 | 6 | Overlaps partially with I-7 |
| **Total** | | | **54.5 / 85** | |

---

#### I-9: `idx_orders_custkey_date_covering`

```sql
CREATE INDEX idx_orders_custkey_date_covering
    ON orders(o_custkey, o_orderdate, o_orderkey);
```

**Accelerates:** Q3, Q10, Q18, Q21 (4 queries)

**Rationale:** Queries that filter on `o_custkey = ? AND o_orderdate < ?` (Q3, Q10, Q18). Including `o_orderkey` makes the index covering for subqueries that only need the order key. The existing `idx_orders_custkey` only has `o_custkey`.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 4 | ×3 | 12 | Turns customer-order join + date filter into efficient index scan |
| Coverage Breadth | 3 | ×3 | 9 | 4 queries |
| Storage Cost | 4 | ×2 | 8 | ~30 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 5 | ×2 | 10 | `o_custkey` is highly selective; each customer has ~10 orders |
| Covering Potential | 4 | ×2 | 8 | Covers queries that only need order keys from this path |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 3 | ×2 | 6 | May overlap with I-7 for queries using both date and custkey |
| **Total** | | | **66.5 / 85** | |

---

#### I-10: `idx_customer_mktsegment`

```sql
CREATE INDEX idx_customer_mktsegment ON customer(c_mktsegment);
```

**Accelerates:** Q3, Q5, Q10, Q18 (4 queries)

**Rationale:** Multiple queries filter on `c_mktsegment = 'BUILDING'` (or 'AUTOMOBILE', 'MACHINERY', etc.). Currently the only secondary index on customer is `idx_customer_nationkey (c_nationkey)`. Without a `c_mktsegment` index, these queries do a full table scan of the customer table.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Moderate — customer is 150K rows, scan not catastrophic but avoidable |
| Coverage Breadth | 3 | ×3 | 9 | 4 queries |
| Storage Cost | 5 | ×2 | 10 | ~2 MB — tiny |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | 5 market segments, 30K rows each |
| Covering Potential | 1 | ×2 | 2 | Not covering — queries need other customer columns |
| Scalability | 3 | ×1.5 | 4.5 | Benefit grows, but customer table is small relative to lineitem |
| Regret Risk | 4 | ×2 | 8 | Straightforward; optimizer will use it |
| **Total** | | | **56 / 85** | |

---

#### I-11: `idx_part_type`

```sql
CREATE INDEX idx_part_type ON part(p_type);
```

**Accelerates:** Q4, Q12, Q13, Q14, Q17 (5 queries)

**Rationale:** Part table (200K rows) currently has no secondary indexes beyond the PK. Queries filter on `p_type` for joins and WHERE clauses. An index here avoids full table scans on part.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Moderate — avoid full scan on 200K rows |
| Coverage Breadth | 4 | ×3 | 12 | 5 queries |
| Storage Cost | 5 | ×2 | 10 | ~3 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | Part types have moderate cardinality |
| Covering Potential | 1 | ×2 | 2 | Not covering |
| Scalability | 3 | ×1.5 | 4.5 | Part table grows slower than lineitem |
| Regret Risk | 4 | ×2 | 8 | Clean single-column index |
| **Total** | | | **59 / 85** | |

---

#### I-12: `idx_part_brand_type_size`

```sql
CREATE INDEX idx_part_brand_type_size ON part(p_brand, p_type, p_size);
```

**Accelerates:** Q8, Q9, Q16, Q19 (4 queries)

**Rationale:** Q16 filters on `p_brand <> ... AND p_type NOT LIKE ... AND p_size IN (...)`. A composite index on `(p_brand, p_type, p_size)` enables efficient multi-column filtering. Q8/Q9 also filter on `p_type` and `p_brand`.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Benefits 4 queries with multi-column filtering |
| Coverage Breadth | 3 | ×3 | 9 | 4 queries |
| Storage Cost | 4 | ×2 | 8 | ~6 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | Brands (5), types (150) — moderate |
| Covering Potential | 2 | ×2 | 4 | Not covering; queries need additional part columns |
| Scalability | 3 | ×1.5 | 4.5 | Scales |
| Regret Risk | 3 | ×2 | 6 | NOT LIKE predicate cannot use index well; partial benefit |
| **Total** | | | **54 / 85** | |

---

#### I-13: `idx_supplier_nationkey_acctbal`

```sql
CREATE INDEX idx_supplier_nationkey_acctbal
    ON supplier(s_nationkey, s_acctbal);
```

**Accelerates:** Q2, Q5, Q7, Q8, Q9, Q11, Q15, Q16, Q20, Q21, Q22 (11 queries)

**Rationale:** Nearly every nation-based query joins supplier via `s_nationkey`. Adding `s_acctbal` as the second column helps Q15 and Q20 which additionally filter/order by account balance. The existing index is only `(s_nationkey)`.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Supplier is only 10K rows — full scan is cheap. Index helps join pipelining. |
| Coverage Breadth | 5 | ×3 | 15 | 11 queries — highest coverage of any proposed index |
| Storage Cost | 5 | ×2 | 10 | ~0.2 MB — tiny |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 3 | ×2 | 6 | 25 nations, 400 suppliers each |
| Covering Potential | 2 | ×2 | 4 | Not covering |
| Scalability | 2 | ×1.5 | 3 | Supplier count is the same at all SF — benefit doesn't scale |
| Regret Risk | 4 | ×2 | 8 | Replaces existing index; no downside |
| **Total** | | | **62.5 / 85** | |

---

#### I-14: `idx_partsupp_availqty_cost`

```sql
CREATE INDEX idx_partsupp_availqty_cost
    ON partsupp(ps_partkey, ps_availqty, ps_supplycost);
```

**Accelerates:** Q2, Q11, Q16, Q20 (4 queries)

**Rationale:** Queries that join partsupp to part on `ps_partkey` and then sort/filter on `ps_supplycost` (Q2, Q11, Q16, Q20). The existing `idx_partsupp_partkey` only has `ps_partkey` — adding `ps_availqty` and `ps_supplycost` makes it more useful for these queries.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 3 | ×3 | 9 | Moderate — extends existing index with useful trailing columns |
| Coverage Breadth | 3 | ×3 | 9 | 4 queries |
| Storage Cost | 4 | ×2 | 8 | ~15 MB (extends existing index) |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 4 | ×2 | 8 | `ps_partkey` is selective |
| Covering Potential | 2 | ×2 | 4 | Partially covering for cost queries |
| Scalability | 4 | ×1.5 | 6 | Scales |
| Regret Risk | 3 | ×2 | 6 | Could replace existing `idx_partsupp_partkey` |
| **Total** | | | **57.5 / 85** | |

---

#### I-15: `idx_part_retailprice`

```sql
CREATE INDEX idx_part_retailprice ON part(p_retailprice);
```

**Accelerates:** Q2

**Rationale:** Q2 uses `p_retailprice = (SELECT MIN(ps_supplycost) FROM partsupp WHERE ...)` for a correlated subquery. An index on `p_retailprice` enables MIN/MAX optimization and range scans.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 2 | ×3 | 6 | Only Q2 |
| Coverage Breadth | 1 | ×3 | 3 | 1 query |
| Storage Cost | 5 | ×2 | 10 | ~3 MB |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 4 | ×2 | 8 | Retail price has good distribution |
| Covering Potential | 1 | ×2 | 2 | Not covering |
| Scalability | 3 | ×1.5 | 4.5 | Part table scales |
| Regret Risk | 4 | ×2 | 8 | Narrow use |
| **Total** | | | **49 / 85** | |

---

#### I-16: Descending Indexes for ORDER BY Optimization

```sql
-- MySQL 8.0 supports true descending indexes
CREATE INDEX idx_orders_date_desc ON orders(o_orderdate DESC);
CREATE INDEX idx_lineitem_shipdate_desc ON lineitem(l_shipdate DESC);
```

**Accelerates:** Q3, Q5, Q10 (queries with `ORDER BY o_orderdate DESC` or `ORDER BY l_shipdate DESC`)

**Rationale:** Queries with `ORDER BY ... DESC` on date columns can avoid a filesort when a descending index exists. Without descending indexes, the optimizer must scan the index in ascending order and then reverse-sort the result, or use a filesort. MySQL 8.0's true descending indexes eliminate this overhead.

**Evaluation:**

| Dimension | Score | Weight | Product | Notes |
|-----------|-------|--------|---------|-------|
| Query Impact | 2 | ×3 | 6 | Eliminates filesort; benefit is moderate (1–5% of query time) |
| Coverage Breadth | 3 | ×3 | 9 | 3 queries |
| Storage Cost | 4 | ×2 | 8 | Replaces or complements existing ascending index |
| Write Penalty | 5 | ×1.5 | 7.5 | Negligible |
| Selectivity | 4 | ×2 | 8 | Same selectivity as ascending |
| Covering Potential | 2 | ×2 | 4 | Not covering alone |
| Scalability | 4 | ×1.5 | 6 | Filesort cost grows with result set size |
| Regret Risk | 5 | ×2 | 10 | Descending index is just as optimizer-friendly as ascending |
| **Total** | | | **58.5 / 85** | |

---

#### I-17: Invisible Indexes for A/B Testing

**Not an index itself — a methodology.**

```sql
-- Test removal without dropping:
ALTER TABLE lineitem ALTER INDEX idx_lineitem_partkey_suppkey INVISIBLE;

-- Run benchmark, collect timings + EXPLAIN

-- Restore:
ALTER TABLE lineitem ALTER INDEX idx_lineitem_partkey_suppkey VISIBLE;
```

**Rationale:** When iterating on indexes, use MySQL 8.0's invisible index feature to test the effect of adding or removing indexes without the cost of `DROP INDEX` / `CREATE INDEX`. A full index evaluation cycle can test 20+ index configurations in minutes rather than hours.

This applies to all indexes listed above and enables rapid identification of:
- Redundant indexes (two indexes covering the same query pattern)
- Unused indexes (indexes the optimizer never chooses)
- Harmful indexes (indexes that confuse the optimizer into worse plans)

---

### Index Interaction & Conflict Analysis

Some indexes overlap or conflict. The optimizer may choose differently depending on which indexes exist simultaneously.

| Index Pair | Relationship | Recommendation |
|-----------|-------------|----------------|
| I-1 (shipdate covering) ↔ I-6 (discount_quantity) | Both want to serve Q6 | Prefer I-1; it serves 9+ queries vs I-6's 1. I-6 is only worthwhile if Q6 is specifically slow after I-1. |
| I-3 (partkey) ↔ existing `(l_partkey, l_suppkey)` | Partially redundant | Keep both initially. Make existing composite invisible during testing. If I-3 alone suffices for Q14/Q17/Q20, drop the composite. |
| I-7 (date_status) ↔ I-8 (date_priority) | Overlap on leading column | Both are useful; optimizer will choose based on query's filter columns. Could merge into `(o_orderdate, o_orderstatus, o_orderpriority)` if storage budget is tight. |
| I-7 (date_status) ↔ I-9 (custkey_date) | Both include `o_orderdate` | Non-conflicting; different leading columns (date vs custkey) serve different query patterns. |
| I-13 (nationkey_acctbal) ↔ existing `(s_nationkey)` | Replaces existing | Drop the existing single-column index; I-13 subsumes it. |
| I-14 (partsupp) ↔ existing `(ps_partkey)` | Replaces existing | Drop the existing single-column index; I-14 subsumes it. |

---

## Verification: Ensuring Index Correctness

Any index optimization must be verified for correctness (doesn't change query results) and effectiveness (actually improves performance).

| # | Verification Step | Method |
|---|-------------------|--------|
| V1 | **Query result checksums** | Before adding any index, run all 22 TPC-H queries, compute `SHA256` of each ordered result set, and save as baseline. After each index change, re-run and diff checksums. Any mismatch = regression. |
| V2 | **EXPLAIN plan capture** | Run `EXPLAIN FORMAT=JSON` for each query before and after each index change. Compare `rows_examined`, `access_type`, `key` used. An index that isn't being used by the optimizer is wasted storage. |
| V3 | **`EXPLAIN ANALYZE` (MySQL 8.0.18+)** | For production-grade verification, use `EXPLAIN ANALYZE` which actually executes the query and reports real timings per iterator. This catches cases where the optimizer's cost model is wrong. |
| V4 | **Hot cache measurements** | Run each query twice — discard the first (cold cache) timing, keep the second (warm cache). Index improvements are most visible with warm cache where I/O differences are eliminated and CPU/index-efficiency dominates. |
| V5 | **Invisible index A/B** | Use the methodology from I-17: make a candidate index invisible, run benchmark, make it visible, run again. Direct A/B comparison on the same data. |

---

## Priority Matrix

### Tier 1 — Maximum Impact, Minimum Risk

These indexes accelerate the most queries with the least storage overhead and lowest risk of optimizer confusion.

| Priority | Index | Score | Queries | Rationale |
|----------|-------|-------|---------|-----------|
| **P1** | I-1: `idx_lineitem_shipdate_covering` | 73.5 | Q3–Q12 (9+) | Highest coverage, covering index eliminates clustered reads for the most common access pattern |
| **P1** | I-7: `idx_orders_date_status` | 67.5 | Q3–Q8, Q12 (6) | Second-largest table, common filter pattern |
| **P1** | I-4: `idx_lineitem_suppkey` | 65.5 | Q5, Q7, Q11, Q15, Q16, Q20 (6) | Fills a critical gap — no supplier-side index on lineitem currently |
| **P1** | I-13: `idx_supplier_nationkey_acctbal` | 62.5 | 11 queries | Highest coverage (11/22) at near-zero storage cost; replaces existing index |
| **P2** | I-9: `idx_orders_custkey_date_covering` | 66.5 | Q3, Q10, Q18, Q21 (4) | Strong for customer-order join pattern |
| **P2** | I-3: `idx_lineitem_partkey` | 61.5 | Q14, Q17, Q20 (3) | Fills part-side gap; partially redundant with existing composite |
| **P2** | I-10: `idx_customer_mktsegment` | 56.0 | Q3, Q5, Q10, Q18 (4) | Small index, 4 queries, fills clear gap |

### Tier 2 — Query-Specific but High Value

These indexes target specific slow queries.

| Priority | Index | Score | Queries | Rationale |
|----------|-------|-------|---------|-----------|
| **P3** | I-2: `idx_lineitem_returnflag_covering` | 59.5 | Q1 | Q1 is uniquely expensive (full lineitem scan + GROUP BY). This covering index is the best optimization for it. |
| **P3** | I-11: `idx_part_type` | 59.0 | Q4, Q12, Q13, Q14, Q17 (5) | Part table has zero secondary indexes beyond PK |
| **P3** | I-16: Descending indexes | 58.5 | Q3, Q5, Q10 (3) | Eliminates filesort; MySQL 8.0 feature |

### Tier 3 — Situational

Add these if queries they target are specifically slow in your workload.

| Priority | Index | Score | Queries |
|----------|-------|-------|---------|
| **P4** | I-14: `idx_partsupp_availqty_cost` | 57.5 | Q2, Q11, Q16, Q20 (4) |
| **P4** | I-6: `idx_lineitem_discount_quantity` | 54.5 | Q6 |
| **P4** | I-5: `idx_lineitem_receiptdate` | 54.5 | Q10, Q11 |
| **P4** | I-8: `idx_orders_date_priority` | 54.5 | Q4, Q5 |
| **P4** | I-12: `idx_part_brand_type_size` | 54.0 | Q8, Q9, Q16, Q19 (4) |
| **P4** | I-15: `idx_part_retailprice` | 49.0 | Q2 |

### Schema Pre-Flight (Run Before Any Indexes)

| Priority | Schema Change | Rationale |
|----------|--------------|-----------|
| **S1** | CHAR → VARCHAR | Makes all subsequent indexes narrower at zero query-correctness risk |
| **S3** | Generated column `l_disc_price` | Enables a dedicated index for the most common TPC-H expression |
| **S4** | utf8mb4 → latin1 for ASCII columns | Halves index width on string columns |

---

## Evaluation Rubric Summary

When evaluating any proposed index, assign a score (1–5) to each dimension and compute the weighted composite:

```
Score = (QueryImpact × 3)
      + (CoverageBreadth × 3)
      + (StorageCost⁻ × 2)
      + (WritePenalty⁻ × 1.5)
      + (Selectivity × 2)
      + (CoveringPotential × 2)
      + (Scalability × 1.5)
      + (RegretRisk⁻ × 2)
```

Where ⁻ denotes inverted scoring (1 = high cost/risk, 5 = low/none). Maximum: **85**.

**Example evaluation** — `idx_lineitem_partkey` (I-3):

| Dimension | Score | Weight | Product |
|-----------|-------|--------|---------|
| Query Impact | 3 | ×3 | 9 |
| Coverage Breadth | 3 | ×3 | 9 |
| Storage Cost⁻ | 5 (tiny) | ×2 | 10 |
| Write Penalty⁻ | 5 (read-only) | ×1.5 | 7.5 |
| Selectivity | 5 (near-unique) | ×2 | 10 |
| Covering Potential | 1 (not covering) | ×2 | 2 |
| Scalability | 4 | ×1.5 | 6 |
| Regret Risk⁻ | 4 (low) | ×2 | 8 |
| **Total** | | | **61.5 / 85** |
