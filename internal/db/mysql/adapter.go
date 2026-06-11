package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/db"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	db.Register("mysql", func(dsn string) (db.Adapter, error) {
		return &MySQLAdapter{dsn: dsn}, nil
	})
}

type MySQLAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *MySQLAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("mysql", a.dsn)
	if err != nil {
		return fmt.Errorf("mysql connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: mysql ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *MySQLAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: mysql ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *MySQLAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *MySQLAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mysql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT c.TABLE_SCHEMA, c.TABLE_NAME, c.COLUMN_NAME, c.ORDINAL_POSITION,
		       c.COLUMN_TYPE, c.IS_NULLABLE, c.COLUMN_DEFAULT, c.EXTRA,
		       c.COLUMN_COMMENT, c.COLUMN_KEY,
		       COALESCE(k.REFERENCED_TABLE_SCHEMA, '') as REFERENCED_TABLE_SCHEMA,
		       COALESCE(k.REFERENCED_TABLE_NAME, '') as REFERENCED_TABLE_NAME,
		       COALESCE(k.REFERENCED_COLUMN_NAME, '') as REFERENCED_COLUMN_NAME,
		       COALESCE(k.CONSTRAINT_NAME, '') as FK_CONSTRAINT_NAME,
		       COALESCE(s.INDEX_NAME, '') as INDEX_NAME,
		       COALESCE(s.COLUMN_NAME, '') as IDX_COLUMN,
		       s.NON_UNIQUE, s.SEQ_IN_INDEX,
		       COALESCE(s.CARDINALITY, 0) as CARDINALITY,
		       COALESCE(s.INDEX_TYPE, '') as INDEX_TYPE,
		       COALESCE(s.INDEX_COMMENT, '') as INDEX_COMMENT,
		       COALESCE(s.IS_VISIBLE, 'YES') as IS_VISIBLE
		FROM information_schema.COLUMNS c
		LEFT JOIN information_schema.KEY_COLUMN_USAGE k
		    ON c.TABLE_SCHEMA = k.TABLE_SCHEMA AND c.TABLE_NAME = k.TABLE_NAME
		    AND c.COLUMN_NAME = k.COLUMN_NAME
		    AND k.REFERENCED_TABLE_NAME IS NOT NULL
		LEFT JOIN information_schema.STATISTICS s
		    ON c.TABLE_SCHEMA = s.TABLE_SCHEMA AND c.TABLE_NAME = s.TABLE_NAME
		    AND c.COLUMN_NAME = s.COLUMN_NAME
		WHERE c.TABLE_SCHEMA = DATABASE()
	`
	if len(tables) > 0 {
		placeholders := strings.Repeat("?,", len(tables))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND c.TABLE_NAME IN (" + placeholders + ")"
	}
	query += " ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION"

	// Convert tables to []any for QueryContext
	args := make([]any, len(tables))
	for i, t := range tables {
		args[i] = t
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql schema: %w", err)
	}
	defer rows.Close()

	// Intermediate row type
	type schemaRow struct {
		TableSchema    string
		TableName      string
		ColumnName     string
		OrdinalPos     int
		ColumnType     string
		IsNullable     string
		ColumnDefault  *string
		Extra          string
		ColumnComment  string
		ColumnKey      string
		RefSchema      string
		RefTable       string
		RefColumn      string
		FKConstraint   string
		IndexName      string
		IdxColumn      string
		NonUnique      *int
		SeqInIndex     *int
		Cardinality    int64
		IndexType      string
		IndexComment   string
		IsVisible      string
	}

	// Maps to group by table
	type idxAccum struct {
		info db.IndexInfo
	}

	type tableAccum struct {
		info        db.TableInfo
		constraints map[string]db.ConstraintInfo
		indexes     map[string]*idxAccum
		colMap      map[string]bool
	}

	tablesMap := make(map[string]*tableAccum)
	var tableOrder []string

	for rows.Next() {
		var r schemaRow
		if err := rows.Scan(
			&r.TableSchema, &r.TableName, &r.ColumnName, &r.OrdinalPos,
			&r.ColumnType, &r.IsNullable, &r.ColumnDefault, &r.Extra,
			&r.ColumnComment, &r.ColumnKey,
			&r.RefSchema, &r.RefTable, &r.RefColumn, &r.FKConstraint,
			&r.IndexName, &r.IdxColumn, &r.NonUnique, &r.SeqInIndex,
			&r.Cardinality, &r.IndexType, &r.IndexComment, &r.IsVisible,
		); err != nil {
			return nil, fmt.Errorf("mysql schema scan: %w", err)
		}

		// Get or create table accumulator
		ta, ok := tablesMap[r.TableName]
		if !ok {
			ta = &tableAccum{
				info: db.TableInfo{
					Schema: r.TableSchema,
					Name:   r.TableName,
					Type:   "BASE TABLE",
				},
				constraints: make(map[string]db.ConstraintInfo),
				indexes:     make(map[string]*idxAccum),
				colMap:      make(map[string]bool),
			}
			tablesMap[r.TableName] = ta
			tableOrder = append(tableOrder, r.TableName)
		}

		// Add column if not already added (JOIN can produce duplicate rows)
		if !ta.colMap[r.ColumnName] {
			ta.colMap[r.ColumnName] = true
			col := db.ColumnInfo{
				Name:         r.ColumnName,
				Ordinal:      r.OrdinalPos,
				ColumnType:   r.ColumnType,
				IsNullable:   r.IsNullable == "YES",
				IsPrimaryKey: r.ColumnKey == "PRI",
				ExtraDef:     r.Extra,
				Comment:      r.ColumnComment,
			}
			if r.ColumnDefault != nil {
				col.Default = *r.ColumnDefault
			}
			if r.Extra == "DEFAULT_GENERATED" || r.Extra == "VIRTUAL GENERATED" || r.Extra == "STORED GENERATED" {
				col.IsGenerated = true
			}
			ta.info.Columns = append(ta.info.Columns, col)
		}

		// Add FK constraint if present
		if r.FKConstraint != "" && r.RefTable != "" {
			key := "fk_" + r.ColumnName
			if cons, exists := ta.constraints[key]; exists {
				// Append to existing FK columns
				cons.Columns = append(cons.Columns, r.ColumnName)
				cons.ReferencedColumns = append(cons.ReferencedColumns, r.RefColumn)
				ta.constraints[key] = cons
			} else {
				ta.constraints[key] = db.ConstraintInfo{
					Name:              r.FKConstraint,
					Type:              "FOREIGN_KEY",
					Columns:           []string{r.ColumnName},
					ReferencedSchema:  r.RefSchema,
					ReferencedTable:   r.RefTable,
					ReferencedColumns: []string{r.RefColumn},
				}
			}
		}

		// Add index if present
		if r.IndexName != "" {
			idx, exists := ta.indexes[r.IndexName]
			if !exists {
				isUnique := true
				if r.NonUnique != nil && *r.NonUnique != 0 {
					isUnique = false
				}
				isVisible := r.IsVisible == "YES"
				idx = &idxAccum{
					info: db.IndexInfo{
						Name:      r.IndexName,
						Type:      r.IndexType,
						IsUnique:  isUnique,
						Primary:   r.IndexName == "PRIMARY",
						Visible:   isVisible,
						Comment:   r.IndexComment,
					},
				}
				ta.indexes[r.IndexName] = idx
			}
			// Check if column already in index columns
			alreadyInIndex := false
			for _, c := range idx.info.Columns {
				if c.Name == r.IdxColumn {
					alreadyInIndex = true
					break
				}
			}
			if !alreadyInIndex && r.IdxColumn != "" {
				seq := 0
				if r.SeqInIndex != nil {
					seq = *r.SeqInIndex
				}
				idx.info.Columns = append(idx.info.Columns, db.IndexColumn{
					Name:        r.IdxColumn,
					Order:       "ASC",
					Sequence:    seq,
					Cardinality: r.Cardinality,
				})
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql schema rows: %w", err)
	}

	// Assemble result
	result := &db.SchemaResult{}
	for _, name := range tableOrder {
		ta := tablesMap[name]

		// Flatten constraints map
		for _, cons := range ta.constraints {
			ta.info.Constraints = append(ta.info.Constraints, cons)
		}

		// Add PK constraint if any column is PK
		hasPK := false
		for _, cons := range ta.info.Constraints {
			if cons.Type == "PRIMARY_KEY" {
				hasPK = true
				break
			}
		}
		if !hasPK {
			var pkCols []string
			for _, col := range ta.info.Columns {
				if col.IsPrimaryKey {
					pkCols = append(pkCols, col.Name)
				}
			}
			if len(pkCols) > 0 {
				ta.info.Constraints = append(ta.info.Constraints, db.ConstraintInfo{
					Name:    "PRIMARY",
					Type:    "PRIMARY_KEY",
					Columns: pkCols,
				})
			}
		}

		// Flatten indexes map
		for _, idx := range ta.indexes {
			ta.info.Indexes = append(ta.info.Indexes, idx.info)
		}

		result.Tables = append(result.Tables, ta.info)
	}

	return result, nil
}

func (a *MySQLAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mysql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if analyze {
		return a.explainAnalyze(queryCtx, query)
	}
	return a.explainBasic(queryCtx, query)
}

// explainBasic runs EXPLAIN FORMAT=JSON and parses the output into ExplainPlan.
func (a *MySQLAdapter) explainBasic(ctx context.Context, query string) (*db.ExplainPlan, error) {
	explainQuery := "EXPLAIN FORMAT=JSON " + query
	return a.executeExplain(ctx, explainQuery, false)
}

// explainAnalyze runs EXPLAIN ANALYZE (MySQL 8.0.18+) which executes the query
// and provides actual runtime metrics. Output is TREE format text stored in
// DialectRaw since it's not structured JSON.
func (a *MySQLAdapter) explainAnalyze(ctx context.Context, query string) (*db.ExplainPlan, error) {
	explainQuery := "EXPLAIN ANALYZE " + query
	plan, err := a.executeExplain(ctx, explainQuery, true)
	if err != nil {
		// Fallback: if EXPLAIN ANALYZE fails (older MySQL), attempt FORMAT=JSON
		// with a warning.
		fallback, fbErr := a.explainBasic(ctx, query)
		if fbErr != nil {
			return nil, fmt.Errorf("%w: explain analyze failed (%v), fallback also failed (%v)",
				db.ErrExplainFailed, err, fbErr)
		}
		fallback.Warnings = append(fallback.Warnings,
			"EXPLAIN ANALYZE not available, showing estimated plan")
		return fallback, nil
	}
	return plan, nil
}

// executeExplain runs an EXPLAIN query, parses FORMAT=JSON output into ExplainPlan.
// For ANALYZE mode (TREE format), stores raw text in DialectRaw.
func (a *MySQLAdapter) executeExplain(ctx context.Context, explainQuery string, isAnalyze bool) (*db.ExplainPlan, error) {
	var rawResult string
	err := a.conn.QueryRowContext(ctx, explainQuery).Scan(&rawResult)
	if err != nil {
		return nil, fmt.Errorf("%w: mysql explain query: %w", db.ErrExplainFailed, err)
	}

	if isAnalyze {
		// EXPLAIN ANALYZE returns TREE format text, not JSON.
		// Store the raw output in DialectRaw and return a plan with
		// minimal parsed fields.
		plan := &db.ExplainPlan{
			EstimatedTotalCost:    float64Ptr(0),
			EstimatedRowsExamined: int64Ptr(0),
			DialectRaw:            rawResult,
			Warnings:              []string{"EXPLAIN ANALYZE TREE output stored in dialect_raw"},
		}
		return plan, nil
	}

	// Parse FORMAT=JSON output
	plan, err := parseExplainJSON(rawResult)
	if err != nil {
		return nil, fmt.Errorf("%w: mysql explain parse: %w", db.ErrExplainFailed, err)
	}
	return plan, nil
}

// parseExplainJSON parses MySQL's EXPLAIN FORMAT=JSON output into ExplainPlan.
func parseExplainJSON(rawJSON string) (*db.ExplainPlan, error) {
	var root mysqlExplainRoot
	if err := json.Unmarshal([]byte(rawJSON), &root); err != nil {
		return nil, fmt.Errorf("unmarshal explain json: %w", err)
	}

	plan := &db.ExplainPlan{
		EstimatedTotalCost:    float64Ptr(0),
		EstimatedRowsExamined: int64Ptr(0),
	}

	if root.QueryBlock.CostInfo != nil {
		cost := parseCost(root.QueryBlock.CostInfo.QueryCost)
		plan.EstimatedTotalCost = &cost
	}

	// Walk the query block tree to extract table access patterns
	walkExplainNode(&root.QueryBlock, plan, "")

	return plan, nil
}

// mysqlExplainRoot is the top-level wrapper for MySQL EXPLAIN FORMAT=JSON.
type mysqlExplainRoot struct {
	QueryBlock mysqlQueryBlock `json:"query_block"`
}

// mysqlQueryBlock represents a query block in the EXPLAIN JSON.
type mysqlQueryBlock struct {
	SelectID   int                `json:"select_id"`
	CostInfo   *mysqlCostInfo     `json:"cost_info,omitempty"`
	Table      *mysqlExplainTable `json:"table,omitempty"`
	NestedLoop []mysqlQueryBlock  `json:"nested_loop,omitempty"`
	Union      []mysqlQueryBlock  `json:"union_result,omitempty"`
	Message    string             `json:"message,omitempty"`
}

// mysqlCostInfo represents cost information in EXPLAIN JSON.
type mysqlCostInfo struct {
	QueryCost string `json:"query_cost"`
}

// mysqlExplainTable represents a table access node in EXPLAIN JSON.
type mysqlExplainTable struct {
	TableName            string               `json:"table_name"`
	AccessType           string               `json:"access_type"`
	PossibleKeys         []string             `json:"possible_keys,omitempty"`
	Key                  string               `json:"key,omitempty"`
	KeyLength            string               `json:"key_length,omitempty"`
	UsedKeyParts         []string             `json:"used_key_parts,omitempty"`
	RowsExaminedPerScan  int64                `json:"rows_examined_per_scan"`
	RowsProducedPerJoin  int64                `json:"rows_produced_per_join"`
	Filtered             string               `json:"filtered,omitempty"`
	CostInfo             *mysqlCostInfo       `json:"cost_info,omitempty"`
	UsedColumns          []string             `json:"used_columns,omitempty"`
	AttachedCondition    string               `json:"attached_condition,omitempty"`
	Materialized         *mysqlExplainTable   `json:"materialized_from_subquery,omitempty"`
}

// walkExplainNode recursively walks a MySQL EXPLAIN JSON tree to populate ExplainPlan.
func walkExplainNode(node *mysqlQueryBlock, plan *db.ExplainPlan, parentAccess string) {
	if node.Table != nil {
		t := node.Table

		// Detect full table scans
		if t.AccessType == "ALL" {
			plan.FullScanTables = append(plan.FullScanTables, t.TableName)
		}

		// Index usage
		if t.Key != "" {
			plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
				Table:      t.TableName,
				Index:      t.Key,
				Covering:   t.AccessType == "index" && len(t.UsedColumns) > 0,
				AccessType: t.AccessType,
			})
		}

		// Update row estimate
		if t.RowsExaminedPerScan > 0 {
			total := t.RowsExaminedPerScan
			if plan.EstimatedRowsExamined != nil {
				total += *plan.EstimatedRowsExamined
			}
			plan.EstimatedRowsExamined = &total
		}

		// Update cost from per-table cost_info
		if t.CostInfo != nil && t.CostInfo.QueryCost != "" {
			tableCost := parseCost(t.CostInfo.QueryCost)
			if plan.EstimatedTotalCost != nil {
				*plan.EstimatedTotalCost += tableCost
			}
		}
	}

	// Detect sort operations from "Using filesort" in attached condition
	if node.Message != "" {
		if strings.Contains(node.Message, "filesort") {
			plan.SortOperations++
		}
		if strings.Contains(node.Message, "temporary") {
			plan.TempOperations++
		}
	}

	// Recurse into nested loop children
	for i := range node.NestedLoop {
		walkExplainNode(&node.NestedLoop[i], plan, "nested_loop")
	}

	// Recurse into union children
	for i := range node.Union {
		walkExplainNode(&node.Union[i], plan, "union")
	}
}

// parseCost parses a MySQL cost string like "1.00" into float64.
func parseCost(costStr string) float64 {
	var cost float64
	if _, err := fmt.Sscanf(costStr, "%f", &cost); err != nil {
		return 0
	}
	return cost
}

func (a *MySQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	// Layer 1: DML/DCL keyword scan (client-side, no DB connection needed)
	if isDML(query) {
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: false,
			Errors:   []string{"DML/DCL statements are not permitted"},
		}, nil
	}

	if a.conn == nil {
		// Without connection, keyword validation is all we can do
		return &db.ValidateResult{
			Valid:    true,
			ReadOnly: true,
		}, nil
	}

	// Layer 2: EXPLAIN-based validation against live MySQL
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := a.conn.ExecContext(queryCtx, "EXPLAIN "+query)
	if err != nil {
		errMsg := err.Error()
		result := &db.ValidateResult{
			Valid:    false,
			ReadOnly: true,
			Errors:   []string{errMsg},
		}

		// Differentiate error codes based on MySQL error message patterns:
		// - "Table '...' doesn't exist" → TABLE_NOT_FOUND
		// - "Unknown column '...' in ..." → COLUMN_NOT_FOUND
		// - Everything else → INVALID_SQL (handled by caller)

		return result, nil
	}

	return &db.ValidateResult{
		Valid:    true,
		ReadOnly: true,
	}, nil
}

func (a *MySQLAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mysql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH,
		       COALESCE(UPDATE_TIME, '') as UPDATE_TIME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
	`
	if len(tables) > 0 {
		placeholders := strings.Repeat("?,", len(tables))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND TABLE_NAME IN (" + placeholders + ")"
	}

	args := make([]any, len(tables))
	for i, t := range tables {
		args[i] = t
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql stats: %w", err)
	}
	defer rows.Close()

	type cardinalityRow struct {
		ColumnName  string
		Cardinality int64
	}

	result := &db.StatsResult{}

	for rows.Next() {
		var tableName, updateTime string
		var tableRows, dataLength, indexLength int64

		if err := rows.Scan(&tableName, &tableRows, &dataLength, &indexLength, &updateTime); err != nil {
			return nil, fmt.Errorf("mysql stats scan: %w", err)
		}

		// Get per-column cardinality for this table
		cardinalities, err := a.getColumnCardinality(queryCtx, tableName)
		if err != nil {
			// Non-fatal - proceed without cardinality
			cardinalities = nil
		}

		stats := db.TableStats{
			Name:               tableName,
			RowCount:           tableRows,
			CardinalityEstimate: tableRows,
			DataSizeBytes:      dataLength,
			IndexSizeBytes:     indexLength,
		}

		if updateTime != "" {
			stats.UpdatedAt = updateTime
			stats.Freshness = "fresh"
		}

		// Use max cardinality from column-level stats if available
		for _, c := range cardinalities {
			if c.Cardinality > stats.CardinalityEstimate {
				stats.CardinalityEstimate = c.Cardinality
			}
		}

		result.Tables = append(result.Tables, stats)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql stats rows: %w", err)
	}

	return result, nil
}

// getColumnCardinality queries column-level cardinality from information_schema.STATISTICS.
func (a *MySQLAdapter) getColumnCardinality(ctx context.Context, tableName string) ([]struct{ ColumnName string; Cardinality int64 }, error) {
	cardQuery := `
		SELECT COLUMN_NAME, MAX(CARDINALITY) as CARDINALITY
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		GROUP BY COLUMN_NAME
	`
	cardRows, err := a.conn.QueryContext(ctx, cardQuery, tableName)
	if err != nil {
		return nil, err
	}
	defer cardRows.Close()

	var results []struct{ ColumnName string; Cardinality int64 }
	for cardRows.Next() {
		var cName string
		var card int64
		if err := cardRows.Scan(&cName, &card); err != nil {
			continue
		}
		results = append(results, struct{ ColumnName string; Cardinality int64 }{cName, card})
	}
	return results, nil
}

func (a *MySQLAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mysql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME,
		       COALESCE(CARDINALITY, 0) as CARDINALITY,
		       COALESCE(INDEX_TYPE, '') as INDEX_TYPE,
		       COALESCE(INDEX_COMMENT, '') as INDEX_COMMENT,
		       COALESCE(IS_VISIBLE, 'YES') as IS_VISIBLE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
	`
	if len(tables) > 0 {
		placeholders := strings.Repeat("?,", len(tables))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND TABLE_NAME IN (" + placeholders + ")"
	}
	query += " ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX"

	args := make([]any, len(tables))
	for i, t := range tables {
		args[i] = t
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql indexes: %w", err)
	}
	defer rows.Close()

	type tableIndexAccum struct {
		tableName string
		indexes   map[string]*db.IndexInfo
	}

	tableMap := make(map[string]*tableIndexAccum)
	var tableOrder []string

	for rows.Next() {
		var tableName, indexName, columnName, indexType, indexComment, isVisible string
		var nonUnique, seqInIndex int
		var cardinality int64

		if err := rows.Scan(&tableName, &indexName, &nonUnique, &seqInIndex, &columnName,
			&cardinality, &indexType, &indexComment, &isVisible); err != nil {
			return nil, fmt.Errorf("mysql indexes scan: %w", err)
		}

		ta, ok := tableMap[tableName]
		if !ok {
			ta = &tableIndexAccum{
				tableName: tableName,
				indexes:   make(map[string]*db.IndexInfo),
			}
			tableMap[tableName] = ta
			tableOrder = append(tableOrder, tableName)
		}

		idx, exists := ta.indexes[indexName]
		if !exists {
			isUnique := nonUnique == 0
			idx = &db.IndexInfo{
				Name:    indexName,
				Type:    indexType,
				IsUnique: isUnique,
				Primary: indexName == "PRIMARY",
				Visible: isVisible == "YES",
				Comment: indexComment,
			}
			ta.indexes[indexName] = idx
		}

		// Add column to index
		idx.Columns = append(idx.Columns, db.IndexColumn{
			Name:        columnName,
			Order:       "ASC",
			Sequence:    seqInIndex,
			Cardinality: cardinality,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql indexes rows: %w", err)
	}

	result := &db.IndexesResult{}
	for _, name := range tableOrder {
		ta := tableMap[name]
		tableIdx := db.TableIndexInfo{
			Table: name,
		}
		for _, idx := range ta.indexes {
			tableIdx.Indexes = append(tableIdx.Indexes, *idx)
		}
		result.Tables = append(result.Tables, tableIdx)
	}

	return result, nil
}

func (a *MySQLAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mysql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Query foreign key constraints from information_schema.
	// This returns one row per FK column; composite FKs (multiple columns)
	// are grouped by CONSTRAINT_NAME.
	query := `
		SELECT
		    k.TABLE_NAME AS source_table,
		    k.COLUMN_NAME AS source_column,
		    k.REFERENCED_TABLE_NAME AS target_table,
		    k.REFERENCED_COLUMN_NAME AS target_column,
		    k.CONSTRAINT_NAME AS constraint_name
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA = DATABASE()
		  AND k.REFERENCED_TABLE_NAME IS NOT NULL
	`
	if len(tables) > 0 {
		placeholders := strings.Repeat("?,", len(tables))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND (k.TABLE_NAME IN (" + placeholders + ") OR k.REFERENCED_TABLE_NAME IN (" + placeholders + "))"
	}
	query += " ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION"

	args := make([]any, 0)
	if len(tables) > 0 {
		for _, t := range tables {
			args = append(args, t)
		}
		for _, t := range tables {
			args = append(args, t)
		}
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql joins: %w", err)
	}
	defer rows.Close()

	// Group FK columns by constraint name to detect composite FKs
	type fkEdgeAccum struct {
		SourceTable string
		TargetTable string
		Columns     [][2]string
		Constraint  string
	}
	edgeMap := make(map[string]*fkEdgeAccum)
	var edgeOrder []string

	for rows.Next() {
		var sourceTable, sourceCol, targetTable, targetCol, constraintName string
		if err := rows.Scan(&sourceTable, &sourceCol, &targetTable, &targetCol, &constraintName); err != nil {
			return nil, fmt.Errorf("mysql joins scan: %w", err)
		}

		key := constraintName
		if edge, ok := edgeMap[key]; ok {
			edge.Columns = append(edge.Columns, [2]string{sourceCol, targetCol})
		} else {
			edgeMap[key] = &fkEdgeAccum{
				SourceTable: sourceTable,
				TargetTable: targetTable,
				Columns:     [][2]string{{sourceCol, targetCol}},
				Constraint:  constraintName,
			}
			edgeOrder = append(edgeOrder, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql joins rows: %w", err)
	}

	result := &db.JoinsResult{}
	for _, key := range edgeOrder {
		edge := edgeMap[key]
		result.Edges = append(result.Edges, db.JoinEdge{
			Source:     edge.SourceTable,
			Target:     edge.TargetTable,
			Columns:    edge.Columns,
			Confidence: 1.0,
			SourceType: "declared_foreign_key",
			Composite:  len(edge.Columns) > 1,
		})
	}

	return result, nil
}

func (a *MySQLAdapter) TestConnect(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return db.PingContext(pingCtx)
}

func (a *MySQLAdapter) DatabaseType() string {
	return "mysql"
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

var dmlKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE",
	"MERGE", "GRANT", "REVOKE", "CREATE", "REPLACE",
}

func isDML(query string) bool {
	upper := strings.TrimSpace(strings.ToUpper(query))
	for _, kw := range dmlKeywords {
		if strings.HasPrefix(upper, kw) {
			if len(upper) == len(kw) || upper[len(kw)] == ' ' || upper[len(kw)] == '\t' || upper[len(kw)] == '\n' || upper[len(kw)] == '(' {
				return true
			}
		}
	}
	return false
}

func BuildDSN(host string, port int, database, username, password string, sslMode string) string {
	params := url.Values{}
	if sslMode != "" && sslMode != "disable" {
		params.Set("tls", sslMode)
	}
	if sslMode == "disable" {
		params.Set("tls", "false")
	}
	paramStr := params.Encode()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, port, database)
	if paramStr != "" {
		dsn += "?" + paramStr
	}
	return dsn
}
