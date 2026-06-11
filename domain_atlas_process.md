# Domain & Join Atlas: Comprehensive Process

This document describes, in full technical detail, the process of building a domain-aware join atlas from a relational database schema. The process consumes a compact JSON schema representation and produces three JSON artifacts: a domain-to-table grouping, a per-table enriched metadata map, and a cross-domain join graph.

The pipeline is designed to work with any relational database. Its goal is to infer logical groupings ("domains") of tables based on their relationships and naming conventions, then produce structured artifacts that AI coding assistants can use to generate accurate SQL.

---

## 1. Overview

The process takes a single structured input — a slim schema JSON — and runs through four major phases:

| Phase | Description |
|-------|-------------|
| **Graph Construction** | Build a weighted undirected graph from relations and naming signals |
| **Domain Inference** | Partition the graph into communities (domains) using Louvain clustering |
| **Refinement** | Apply overrides, detect sub-domains, identify bridge tables |
| **Output Assembly** | Produce three structured JSON artifacts |

---

## 2. Input: The Slim Schema JSON

The slim schema is a compact JSON representation produced by an earlier stage of the pipeline. Its structure:

```json
{
  "name": "<database_name>",
  "tables": [...],
  "relations": [...]
}
```

### 2.1 `name`

The database name as a string, carried through for reference.

### 2.2 `tables`

An array of table objects:

```json
{
  "name": "<string>",
  "pk": "<string|null>",
  "columns": [
    {"name": "<string>", "type": "<string>"}
  ],
  "indexes": [
    {"name": "<string>", "columns": ["<col>"]},
    {"name": "<string>", "columns": ["<col1>", "<col2>"]}
  ]
}
```

| Field | Description |
|-------|-------------|
| `name` | The table name |
| `pk` | The primary key column name, or `null` if no primary key exists |
| `columns` | Array of column objects, each with `name` and `type` (e.g., `"int unsigned"`, `"varchar(255)"`, `"datetime"`) |
| `indexes` | Array of index objects. The primary key index is excluded. Each index has a `name` and a `columns` array. Composite indexes contain multiple column names. |

### 2.3 `relations`

An array of relation objects:

```json
{
  "table": "<string>",
  "columns": ["<col>"],
  "parent_table": "<string>",
  "parent_columns": ["<col>"],
  "declared": true
}
```

| Field | Description |
|-------|-------------|
| `table` | The source (child) table |
| `columns` | The foreign key column(s) on the source table |
| `parent_table` | The target (parent) table |
| `parent_columns` | The referenced column(s) on the parent table |
| `declared` | Boolean. `true` means a real `FOREIGN KEY` constraint exists in the database. `false` means the relation was inferred from naming conventions (e.g., a column named `user_id` suggests a relationship to the `users` table). |

Relations are de-duplicated: if the same `(table, columns, parent_table, parent_columns)` tuple appears multiple times from different sources, only one copy is retained.

---

## 3. Phase 1: Graph Construction

The core algorithm begins by building a **weighted undirected graph** where each node represents a table and each edge represents evidence of a relationship between two tables. Edge weight accumulates from multiple independent signals.

### 3.1 Adding Nodes

Every table from `slim_schema.tables` becomes a node in the graph. The graph starts with no edges.

### 3.2 Adding Relation Edges

For each relation in `slim_schema.relations`, an undirected edge is added between the source and target tables.

**Edge weight assignment:**
- **Declared foreign keys** (real `FOREIGN KEY` constraints in the database): weight **2.0**
- **Inferred relations** (naming convention matches, no actual constraint): weight **1.0**

**Additive weights:** If an edge already exists between the same pair of tables (from a previous relation in the list), the new weight is **added** to the existing weight. For example, if table A has both a declared FK to table B (weight 2.0) and an inferred relation to table B (weight 1.0), the final edge weight is 3.0. If two declared FKs both go from A to B, the weight is 4.0 and so on.

**Broken relations:** A relation whose source or target table does not exist in the graph is handled differently depending on its type:
- **Inferred relations** to missing tables are recorded as "broken virtual relations" and reported in the final output. They do not halt processing.
- **Declared FKs** to missing tables are silently skipped.

### 3.3 Adding Prefix-Penalty Edges

Tables that share a common name prefix receive an additional **0.5** weight on their edge. This biases tables with similar naming toward the same domain.

**Prefix extraction:**
1. Split the table name on underscore (`_`)
2. Take the first token
3. Normalize to lowercase
4. If the token has fewer than 3 characters, it is ignored (no prefix assigned)

For example: `order_items` → prefix `"order"`, `user_sessions` → prefix `"user"`, `xy_log` → no prefix (token `"xy"` is only 2 characters).

**Edge creation:** For every group of tables sharing the same prefix (minimum 2 tables), every pair within the group gets an additional 0.5 weight. If the edge already exists (from relations), the 0.5 is added to its existing weight. If no edge exists, a new edge with weight 0.5 is created.

**Motivation:** Tables that share a name prefix (e.g., `order_items`, `order_payments`, `order_shipments`) are likely related even if no explicit foreign key relationship exists between them. The prefix signal nudges them toward the same community without overwhelming the stronger FK-based signals.

### 3.4 Graph Properties Summary

After construction, the graph has these properties:
- **Nodes:** All tables from the slim schema
- **Edges:** Weighted undirected edges, weight ≥ 0.5
- **Weakest edges:** Prefix-penalty edges with no relation backing (weight 0.5)
- **Strongest edges:** Tables connected by multiple declared FKs (weight can be 4.0, 6.0, etc.)
- **Isolated nodes:** Tables with degree 0 (no relations, no prefix peers)

---

## 4. Phase 2: Domain Inference

Domains are inferred by partitioning the weighted graph into communities using the Louvain algorithm, then naming each community using a prefix-based voting scheme.

### 4.1 Louvain Community Detection

The **Louvain method** is a greedy optimization algorithm that partitions a graph to maximize **modularity** — a measure of how densely connected nodes are within communities compared to how they would be in a random graph. The algorithm uses edge weights to determine community membership, so tables connected by strong edges (declared FKs, multiple relations) are more likely to end up in the same community.

**Parameters:**
- **Weight attribute**: `"weight"` — each edge's accumulated weight drives the partition
- **Resolution**: A tunable parameter (default `1.0`) that controls community granularity:
  - **Lower resolution (< 1.0)**: Fewer, larger communities. Tables are more aggressively grouped.
  - **Higher resolution (> 1.0)**: More, smaller communities. Each domain is more tightly focused.
  - **Default 1.0**: Standard Louvain modularity. Balances cohesion and separation.
- **Seed**: The algorithm is deterministic — given the same graph and resolution, it always produces the same communities (enforced via a fixed random seed of `42`).

**Result ordering:** Communities are sorted by size (largest first), then alphabetically within each size group.

### 4.2 Naming Communities

Each community (a set of table names) is given a human-readable domain name through prefix-based voting.

**Step 1 — Handle singletons:**
If a community contains exactly one table and that table has degree 0 (no edges in the graph), it is named `"misc"`. Tables that are completely disconnected form no meaningful domain.

**Step 2 — Prefix voting:**
For all other communities, extract the prefix from each member table (using the same `_`-splitting, ≥3-character rule from graph construction). Count how many tables share each prefix. If any prefix appears in **≥ 40%** of the community's member tables, use that prefix as the domain name.

For example, in a community of 10 tables where 5 have prefix `"order"` and 5 have various other prefixes, `"order"` wins with 50% ≥ 40%.

**Step 3 — Fallback:**
If no prefix meets the 40% threshold, the community is named after the **highest-degree node** in the community — the table with the most edges.

### 4.3 Domain Name Resolution

Once a winning prefix or fallback name is chosen, it is transformed into a final domain name:

1. If the prefix itself matches a table name in the community, use the prefix as-is (e.g., prefix `"order"` matches table `order` → domain `"order"`).
2. If the prefix itself is not a table in the community, try the **plural form** of the prefix (e.g., prefix `"order"` with no table named `order` → domain `"orders"`).
3. If the plural form is also not a member table, use the plural form anyway (it is still the best semantic label).

### 4.4 Deduplication

Domain names must be unique across all communities. If two communities would receive the same domain name, a numeric suffix is appended: the second occurrence gets `_2`, the third gets `_3`, and so on.

**Exception:** The name `"misc"` is exempt from deduplication. If multiple communities get the name `"misc"`, they are instead **merged** into a single `"misc"` domain rather than receiving suffixes.

### 4.5 Misc Merging

The `"misc"` domain receives special treatment:
- All singleton, disconnected tables are assigned to `"misc"`
- If multiple communities independently get the name `"misc"`, all their tables are merged into one domain
- The `"misc"` domain serves as a catch-all for tables that do not fit cleanly into any logical domain

### 4.6 Domain Assignment Output

The result of this phase is a **table-to-domain mapping** — a dictionary associating each table name with its domain name. This mapping is inverted after refinement steps to produce the domain-to-tables grouping used in the final `domain_map.json` output.

---

## 5. Phase 3: Refinement

After community detection and naming, three refinement steps are applied: manual overrides, sub-domain detection, and bridge table identification.

### 5.1 Manual Overrides

Users can provide a JSON file containing manual domain assignments. Each override specifies a table name and the domain it should belong to.

**Format:**
```json
{
  "overrides": [
    {"table": "some_table_name", "domain": "desired_domain_name"},
    {"table": "another_table", "domain": "other_domain"}
  ]
}
```

**Processing:**
- The override file is loaded and parsed into a `{table_name: domain_name}` mapping.
- If the file does not exist, an empty mapping is used — no error.
- Each override is applied to the table-to-domain mapping: for each `(table, domain)` entry, if the table exists in the mapping, its domain assignment is overwritten.
- Tables not found in the mapping (i.e., not in the slim schema) are silently ignored.
- Override domain names are used as-is — they do not go through deduplication.

**Re-grouping:** After applying overrides, the table-to-domain mapping is re-inverted to produce the domain-to-tables grouping used in the final output. This ensures the manual assignments are reflected in the domain groupings.

### 5.2 Sub-Domain Detection

Large domains (those containing more than 15 tables) are further partitioned into **sub-domains**. This provides an additional level of organization within large, complex domains.

**Eligibility:**
- The domain must have **more than 15 tables**
- The `"misc"` domain is **never** sub-domained, regardless of size
- The subgraph formed by the domain's tables must have **at least one edge** (no relationships to cluster on means no meaningful sub-structure exists)

**Algorithm:**
1. Extract the induced subgraph for the domain (only tables in the domain and edges between them)
2. Run Louvain community detection on this subgraph with **resolution = 1.5**
   - The higher resolution (1.5 vs the default 1.0 for top-level domains) produces finer-grained, smaller sub-communities
   - This is hardcoded — the user-specified resolution parameter affects only the top-level partition
3. If only **one subcommunity** forms, skip — the domain has no meaningful internal structure
4. Name each subcommunity using the same prefix-voting, fallback-to-highest-degree logic as top-level domains
5. Any subcommunity named `"misc"` is renamed to `"misc_subdomain"` to distinguish it from the top-level `"misc"` domain
6. Subdomain names are deduplicated (same `_2`, `_3` suffixing rule)

**Output:** The result is a mapping from each table (in sub-domained domains) to its subdomain name, plus a domain-to-subdomains grouping.

### 5.3 Bridge Table Detection

A **bridge table** is a table whose immediate neighbors belong to two or more different domains. Bridge tables are important for understanding cross-domain joins and query planning.

**Algorithm:**
1. Build an **adjacency map** from all relations: for each relation where both the source and target tables exist in the domain mapping, add each table to the other table's neighbor set.
2. For each table, collect the set of domains its neighbors belong to.
3. A table is a "bridge" if the cardinality of this domain set is **≥ 2**.

**Output:** A mapping `{table_name: [sorted_domain_names]}` indicating which domains each bridge table connects.

**Example:** If `reviews` has a declared FK to `products.id` (in the `products` domain) and another FK to `users.id` (in the `users` domain), then `reviews` is a bridge table connecting `products` and `users`.

---

## 6. Phase 4: Output Assembly

Three structured JSON artifacts are produced from all accumulated data.

### 6.1 `domain_map.json` — Domain-to-Table Grouping

A top-level summary of the domain structure:

```json
{
  "metadata": {
    "table_count": 120,
    "domain_count": 10,
    "subdomain_count": 14
  },
  "domains": {
    "orders": {
      "tables": [
        "order_items",
        "order_payments",
        "order_shipments"
      ],
      "sub_domains": {
        "order_fulfillment": [
          "order_shipments",
          "order_tracking"
        ],
        "order_finance": [
          "order_invoices",
          "order_payments"
        ]
      }
    },
    "misc": {
      "tables": [
        "feature_flags",
        "schema_migrations",
        "system_config"
      ]
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `metadata.table_count` | Total number of tables in the slim schema |
| `metadata.domain_count` | Number of top-level domains (including `"misc"`) |
| `metadata.subdomain_count` | Total subdomains across all domains |
| `domains.<name>.tables` | Sorted array of table names in the domain |
| `domains.<name>.sub_domains` | Object keyed by subdomain name, each value is a sorted array of table names. Present only if the domain has sub-domains. |

### 6.2 `schema_map.json` — Per-Table Enriched Metadata

This is the richest artifact. The top-level `"tables"` object is keyed by table name:

```json
{
  "tables": {
    "reviews": {
      "domain": "products",
      "sub_domain": null,
      "pk": "id",
      "bridge": true,
      "bridge_domains": ["products", "users"],
      "fk_out": [
        {"to": "products.id", "declared": true},
        {"to": "users.id", "declared": true}
      ],
      "fk_in": [
        "review_votes.review_id"
      ],
      "indexed_columns": [
        "product_id",
        "user_id"
      ],
      "composite_indexes": []
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `domain` | string | The domain name the table belongs to |
| `sub_domain` | string or null | The subdomain name, or `null` if none |
| `pk` | string or null | Primary key column name, or `null` |
| `bridge` | boolean | `true` if the table connects two or more domains |
| `bridge_domains` | array of strings | Sorted list of domain names this table bridges. Empty array `[]` if not a bridge. |
| `fk_out` | array of objects | Forward relations — where this table is the source. Each object has: `"to"` (formatted as `"table.column"` for single-column or `"table.(col1,col2)"` for composite keys) and `"declared"` (boolean) |
| `fk_in` | array of strings | Reverse relations — tables that reference this one, formatted as `"table.column"` |
| `indexed_columns` | array of strings | Columns appearing in single-column indexes, deduplicated in order of first occurrence |
| `composite_indexes` | array of arrays | Multi-column indexes, each as an array of column name strings |

### 6.3 `join_graph.json` — Cross-Domain Join Relationships

A flat list of all relationships, annotated with cross-domain information:

```json
{
  "relationships": [
    {
      "from": "reviews.product_id",
      "to": "products.id",
      "declared": true,
      "cross_domain": false
    },
    {
      "from": "reviews.user_id",
      "to": "users.id",
      "declared": true,
      "cross_domain": true
    },
    {
      "from": "order_shipments.(order_id,item_id)",
      "to": "order_items.(order_id,item_id)",
      "declared": false,
      "cross_domain": false
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `from` | Source endpoint. Single column: `"table.column"`. Composite: `"table.(col1,col2)"`. |
| `to` | Target endpoint. Same format as `from`. |
| `declared` | `true` if a real `FOREIGN KEY` constraint exists in the database |
| `cross_domain` | `true` when the source table's domain differs from the target table's domain |

**Cross-domain determination:** `cross_domain` is `true` when the source table's domain differs from the target table's domain. This makes it immediately clear which joins cross domain boundaries.

**Inclusion criteria:** Only relationships where **both** the source and target tables exist in the domain mapping are included. Relations to tables not in the graph (broken virtual relations) are excluded.

---

## 7. Modularity Calculation

After the complete partition is produced, the **modularity** of the top-level domain partition is computed:

- Modularity measures how well the graph is partitioned into communities, ranging from approximately -1 to 1
- Higher values indicate stronger community structure (dense internal connections, sparse external connections)
- The modularity score is classified as:
  - **< 0.3**: Weak community structure
  - **0.3 – 0.5**: Moderate community structure
  - **> 0.5**: Strong community structure

This score provides a quality signal: a high modularity means the Louvain algorithm found natural, well-separated groupings. A low modularity suggests the schema has a more mesh-like structure where clear domain boundaries are harder to find.

---

## 8. Parameter Reference

| Parameter | Default | Purpose |
|-----------|---------|---------|
| `resolution` (float) | `1.0` | Controls top-level Louvain community granularity. Lower = fewer/larger domains, higher = more/smaller domains |
| `overrides` (file path) | `domain_overrides.json` | JSON file with manual `{table: domain}` assignments applied after community detection |
| `generate_summaries` (flag) | `false` | Flag for AI-generated natural language summaries of each domain |

---

## 9. Algorithm Summary

| Step | Algorithm | Key Detail |
|------|-----------|------------|
| Graph edges from relations | Weight sum over all relations between a pair | Declared FK = 2.0, Inferred = 1.0, Additive |
| Graph edges from naming | Prefix-based bias among tables sharing a name prefix | +0.5 per shared prefix (≥3 chars, ≥2 tables) |
| Community detection | Louvain modularity optimization | Weighted, deterministic (seed=42), resolution-tunable |
| Community naming | Prefix voting with 40% threshold | Falls back to highest-degree node if no prefix dominates |
| Name resolution | Direct match → plural form → plural | Uses linguistic pluralization for human-readable names |
| Deduplication | Numeric suffixes (`_2`, `_3`, ...) | `"misc"` exempt: duplicates are merged instead |
| Sub-domain detection | Recursive Louvain on large domain subgraphs | Resolution=1.5 (hardcoded), >15 table threshold |
| Bridge detection | Neighbor domain-set cardinality check | ≥2 distinct domains among a table's FK neighbors |
| Modularity evaluation | Partition quality metric | Classified as weak / moderate / strong |

---

## 10. Edge Cases

| Scenario | Behavior |
|----------|----------|
| Empty or missing slim schema input | Fatal error |
| Relation to a table not in the graph (inferred) | Recorded as "broken", reported but not fatal |
| Relation to a table not in the graph (declared FK) | Silently skipped |
| Singleton table with no relations or prefix peers | Assigned to `"misc"` domain |
| Community with no clear prefix (below 40% threshold) | Named after highest-degree node |
| Two communities get the same name | Second gets `_2`, third gets `_3`, etc. |
| Multiple "misc" communities | Merged into a single `"misc"` domain (no suffixing) |
| Domain with 15 or fewer tables | No sub-domain analysis attempted |
| Domain with no edges in its subgraph | Sub-domain analysis skipped (no structure to find) |
| Sub-community named "misc" | Renamed to `"misc_subdomain"` |
| Only one subcommunity found | Sub-domain creation skipped |
| Override file does not exist | Empty overrides mapping, not an error |
| Override for a table not in the graph | Silently ignored |
| AI summary generation requested without API key | Warning printed, not an error |
