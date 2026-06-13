# Querylex Optimization Analysis Contract

`ANALYSIS.md` is the normative contract for explain-plan heuristic analysis and optimization strategies. It defines how Querylex turns an execution plan plus schema context into optimization candidates, how it compares plans across database dialects, and when it must report that no safe improvement was found.

**Note:** The heuristic signal detection (Full Scan, Non-Sargable, etc.) and plan normalization are implemented in `internal/analysis/heuristics.go`. The AI-powered SQL rewrite loop, automated index recommendations, and validation loop (`LLM Boundary` section) are design specifications, not implemented in the current codebase.

## Inputs

The optimizer receives a structured analysis bundle:

```json
{
  "active_database": {
    "id": "prod-mysql-main",
    "type": "mysql",
    "version": "8.0.36",
    "dialect": "mysql"
  },
  "original_sql": "SELECT * FROM orders WHERE DATE(created_at) = '2026-05-01'",
  "explain": {
    "analyze": false,
    "execution_plan": []
  },
  "schema": {
    "available": true,
    "tables": []
  },
  "joins": {
    "available": true,
    "joins": [],
    "paths": []
  },
  "stats": {
    "available": true,
    "tables": []
  },
  "indexes": {
    "available": true,
    "tables": []
  },
  "terminology_context": {},
  "warnings": []
}
```

Each context block must include `available: false` and a warning code when the corresponding command failed. Missing context is an analysis signal, not just a logging detail.

## Context Coverage

Before proposing a rewrite, Querylex computes context coverage:

| Signal | Weight |
|---|---:|
| Original explain plan is available | 35 |
| Referenced table and column schema is available | 25 |
| Join graph for referenced tables is available when joins exist | 15 |
| Table statistics are available | 15 |
| Index metadata is available | 10 |

Coverage is the sum of available weights.

- `80-100`: full context. Querylex may rewrite SQL and may recommend indexes.
- `50-79`: partial context. Querylex may rewrite SQL, but must include context warnings and avoid high-risk structural rewrites unless validation and plan comparison support them.
- `35-49`: plan-only context. Querylex may suggest low-risk rewrites only, such as removing non-sargable wrappers or narrowing projections.
- `<35`: insufficient context. Querylex must return an unable-to-optimize result that explains which context sources failed and recommends fixing indexing or permissions first.

If all schema, join, stats, and index lookups fail, Querylex must not present infrastructure tuning as the primary answer. The primary diagnosis is missing local context.

## Plan Normalization

Database engines expose different plan formats. Querylex normalizes each plan into a common structure before comparison:

```json
{
  "estimated_total_cost": 24511.38,
  "actual_total_time_ms": null,
  "estimated_rows_examined": 181223,
  "actual_rows_examined": null,
  "full_scan_tables": ["orders"],
  "index_usage": [
    {
      "table": "orders",
      "index": "idx_orders_created_at",
      "covering": false,
      "access_type": "range"
    }
  ],
  "sort_operations": 0,
  "temp_operations": 0,
  "join_operations": [
    {
      "type": "nested_loop",
      "estimated_rows": 100000
    }
  ],
  "warnings": []
}
```

Dialect adapters map native fields into this normalized form:

| Dialect | Estimated-cost source | Runtime source when analyzed | Notable scan signals |
|---|---|---|---|
| MySQL / MariaDB | `cost_info.query_cost`, `rows_examined`, `filtered`, access type | `EXPLAIN ANALYZE` actual time and rows | `ALL`, `Using temporary`, `Using filesort` |
| PostgreSQL | `Total Cost`, `Plan Rows`, node costs | `Actual Total Time`, `Actual Rows`, loops | `Seq Scan`, large hash/sort nodes, poor row estimates |
| SQLite | `EXPLAIN QUERY PLAN` detail text | Not generally available from explain | `SCAN TABLE`, missing `USING INDEX` |
| SQL Server | estimated subtree cost, estimated rows from SHOWPLAN XML/JSON | actual execution plan runtime stats when permitted | table scan, key lookup explosion, spills, missing index hints |

## What Counts As Better

A rewritten SQL query is considered better only when validation succeeds and one of these conditions is true:

1. With `--analyze`, actual total runtime improves by at least 10% without increasing rows examined by more than 20%.
2. Without `--analyze`, normalized estimated total cost improves by at least 15%.
3. Estimated rows examined improves by at least 25% and the rewrite does not introduce new high-risk operations such as broad sorts, temp tables, cartesian joins, or full scans.
4. A severe qualitative issue is removed, such as a full scan on a large table caused by a non-sargable predicate, and no normalized metric regresses by more than 10%.

If metrics conflict, `--analyze` runtime wins over estimated cost. Without runtime data, Querylex must report the confidence level and the exact metrics used for the decision.

Small changes below these thresholds may be returned as style or maintainability suggestions, but they must not be labeled as optimization wins.

## Heuristic Signals

Querylex looks for these issue classes:

| Code | Trigger | Preferred rewrite |
|---|---|---|
| `FULL_TABLE_SCAN` | Large table scanned without a selective predicate or usable index. | Add selective predicate when semantically required, make predicate sargable, or recommend an index if rewrites cannot help. |
| `NON_SARGABLE_PREDICATE` | Indexed column wrapped in a function or expression. | Move transformation to the constant side or use a range predicate. |
| `UNBOUNDED_SELECT_STAR` | `SELECT *` against large or joined tables. | Project only required columns when output requirements are known. |
| `JOIN_ORDER_RISK` | Large table is joined before selective filters or smaller driving tables. | Reorder joins or push filters into subqueries/CTEs when supported by the dialect. |
| `CARTESIAN_JOIN` | Join has no predicate or a low-confidence inferred relationship. | Add or correct join predicate; stop if relationship is unknown. |
| `LOW_SELECTIVITY_INDEX` | Chosen index has poor cardinality for the predicate. | Prefer a more selective existing index or recommend a composite index. |
| `MISSING_COMPOSITE_INDEX` | Query filters/orders/groups by columns that no existing index covers. | Recommend a dialect-specific composite index only when rewrite attempts are insufficient. |
| `SORT_OR_TEMP_PRESSURE` | Plan uses expensive sort, temp table, hash aggregate spill, or filesort. | Align `ORDER BY`/`GROUP BY` with an index, pre-aggregate, or reduce input rows earlier. |
| `CORRELATED_SUBQUERY_RISK` | Subquery executes per outer row with high estimated loops. | Rewrite as join, derived table, or CTE if equivalent. |
| `OFFSET_PAGINATION_RISK` | Large `OFFSET` requires scanning skipped rows. | Recommend keyset pagination when output semantics allow it. |
| `STALE_STATS` | Statistics are missing or stale. | Recommend refreshing database statistics before trusting plan comparisons. |
| `MISSING_CONTEXT` | Schema, stats, joins, or indexes are unavailable. | Report context failure and re-index/permission guidance before infrastructure tuning. |

## Rewrite Strategy Order

Each optimization attempt must use a distinct strategy:

1. Predicate and projection rewrite: make filters sargable, remove unused columns, simplify redundant predicates.
2. Join and subquery rewrite: reorder joins, push filters, convert correlated subqueries, remove unnecessary derived tables.
3. Aggregation and shape rewrite: use CTEs, pre-aggregation, window functions, or alternative grouping patterns when supported.

The optimizer must stop a strategy early if it cannot preserve semantics. It must not invent columns, relationships, filters, or business rules that are not present in the input context.

## Index Recommendation Rules

Querylex may recommend a new index only when all are true:

- `--no-index` was not set.
- A plan bottleneck maps to a missing or inadequate index.
- Existing indexes do not already cover the access pattern.
- The target table is large enough or queried often enough to justify write overhead.
- The recommendation names the affected query predicate, join, sort, or grouping.

Index recommendations must be dialect-aware:

| Dialect | Recommendation format |
|---|---|
| MySQL / MariaDB | `CREATE INDEX idx_name ON table_name (col1, col2);` |
| PostgreSQL | May include partial or expression indexes when predicates are stable. |
| SQLite | Use simple or expression indexes only when supported by the runtime version. |
| SQL Server | May include nonclustered indexes and included columns when plan evidence supports it. |

All index output must include a warning to test outside production and to review write overhead and storage impact.

## Validation Loop

Every generated rewrite must pass `querylex validate "SQL"` before plan comparison. The validate-fix loop is capped at 3 attempts for each rewrite strategy. If all 3 validation attempts fail for a strategy, Querylex records the validation errors and moves to the next distinct strategy.

## LLM Boundary

The low-level commands are deterministic JSON-producing commands. AI reasoning is allowed only inside slash-skill workflows such as `/querylex-sql` and `/querylex-optimize`.

For `/querylex-optimize`, the model receives only:

- The original SQL.
- Normalized explain output and native plan snippets needed for evidence.
- Schema, join, stats, and index context for referenced tables.
- Dialect and feature flags.
- Prior attempt summaries and validation errors.

The model must return structured analysis:

```json
{
  "issues": [
    {
      "code": "NON_SARGABLE_PREDICATE",
      "severity": "high",
      "evidence": "DATE(created_at) prevents use of idx_orders_created_at.",
      "affected_tables": ["orders"]
    }
  ],
  "rewrite_strategy": "predicate_rewrite",
  "optimized_sql": "SELECT * FROM orders WHERE created_at >= '2026-05-01' AND created_at < '2026-05-02'",
  "expected_plan_changes": [
    "Use range scan on idx_orders_created_at instead of full table scan."
  ],
  "requires_new_index": false,
  "confidence": 0.82
}
```

If the AI service is unavailable, Querylex may fall back to deterministic heuristic rewrites for simple cases. Otherwise it must return a clear error with `AI_SERVICE_UNAVAILABLE` and avoid pretending that optimization analysis was completed.

## Unable-To-Optimize Output

When no better plan is found, Querylex returns:

- Best validated attempt, if any.
- Attempt log with strategy, validation result, plan comparison, and reason rejected.
- Context coverage score and missing context warnings.
- Dialect-aware next steps.

Dialect-aware next steps:

| Dialect | Examples |
|---|---|
| MySQL / MariaDB | Review partitioning for large date/range filters, `innodb_buffer_pool_size`, and `SHOW STATUS LIKE 'Innodb_buffer_pool_reads'`. |
| PostgreSQL | Review table/index bloat, `ANALYZE` freshness, `shared_buffers`, `work_mem`, and `pg_stat_statements`. |
| SQLite | Review `ANALYZE`, database file size, transaction mode, and whether indexes match the query predicates. |
| SQL Server | Review statistics freshness, missing-index DMVs, memory grants, spills, and Query Store regressions. |

Application-level caching and precomputed summary tables are dialect-independent suggestions when the query is expensive, frequent, and can tolerate stale results.
