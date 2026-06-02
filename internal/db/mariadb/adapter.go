package mariadb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	db.Register("mariadb", func(dsn string) (db.Adapter, error) {
		return &MariaDBAdapter{dsn: dsn}, nil
	})
}

type MariaDBAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *MariaDBAdapter) Connect(ctx context.Context, dsn string) error {
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
		return fmt.Errorf("mariadb connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: mariadb ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *MariaDBAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: mariadb ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *MariaDBAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *MariaDBAdapter) DatabaseType() string {
	return "mariadb"
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func (a *MariaDBAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mariadb adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Detect system-versioned tables
	svTables := make(map[string]bool)
	svRows, err := a.conn.QueryContext(queryCtx, "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'SYSTEM VERSIONED'")
	if err == nil {
		defer svRows.Close()
		for svRows.Next() {
			var name string
			if err := svRows.Scan(&name); err == nil {
				svTables[name] = true
			}
		}
	}

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

	args := make([]any, len(tables))
	for i, t := range tables {
		args[i] = t
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mariadb schema: %w", err)
	}
	defer rows.Close()

	type schemaRow struct {
		TableSchema   string
		TableName     string
		ColumnName    string
		OrdinalPos    int
		ColumnType    string
		IsNullable    string
		ColumnDefault *string
		Extra         string
		ColumnComment string
		ColumnKey     string
		RefSchema     string
		RefTable      string
		RefColumn     string
		FKConstraint  string
		IndexName     string
		IdxColumn     string
		NonUnique     *int
		SeqInIndex    *int
		Cardinality   int64
		IndexType     string
		IndexComment  string
		IsVisible     string
	}

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
			return nil, fmt.Errorf("mariadb schema scan: %w", err)
		}

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

			// Mark system-versioned columns
			if svTables[r.TableName] && (r.Extra == "VIRTUAL GENERATED" || strings.Contains(r.ColumnComment, "system versioned")) {
				col.ExtraDef = "system_versioned"
			}

			ta.info.Columns = append(ta.info.Columns, col)
		}

		if r.FKConstraint != "" && r.RefTable != "" {
			key := "fk_" + r.ColumnName
			if cons, exists := ta.constraints[key]; exists {
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
						Name:    r.IndexName,
						Type:    r.IndexType,
						IsUnique: isUnique,
						Primary: r.IndexName == "PRIMARY",
						Visible: isVisible,
						Comment: r.IndexComment,
					},
				}
				ta.indexes[r.IndexName] = idx
			}
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
		return nil, fmt.Errorf("mariadb schema rows: %w", err)
	}

	result := &db.SchemaResult{}
	for _, name := range tableOrder {
		ta := tablesMap[name]

		for _, cons := range ta.constraints {
			ta.info.Constraints = append(ta.info.Constraints, cons)
		}

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

		for _, idx := range ta.indexes {
			ta.info.Indexes = append(ta.info.Indexes, idx.info)
		}

		result.Tables = append(result.Tables, ta.info)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Explain
// ---------------------------------------------------------------------------

type mariadbExplainPlanJSON struct {
	QueryBlock mariadbQueryBlock `json:"query_block"`
}

type mariadbQueryBlock struct {
	SelectID  int              `json:"select_id"`
	CostInfo  *mariadbCostInfo `json:"cost_info,omitempty"`
	Table     *mariadbExplainTable `json:"table,omitempty"`
	Tables    []mariadbExplainTable `json:"tables,omitempty"`
	NestedLoop []mariadbExplainTable `json:"nested_loop,omitempty"`
}

type mariadbCostInfo struct {
	QueryCost string `json:"query_cost"`
}

type mariadbExplainTable struct {
	TableName      string              `json:"table_name"`
	AccessType     string              `json:"access_type"`
	RowsExamined   int64               `json:"rows_examined_per_scan"`
	RowsProduced   int64               `json:"rows_produced_per_join"`
	CostInfo       *mariadbTableCost   `json:"cost_info,omitempty"`
	Key            string              `json:"key"`
	KeyLength      string              `json:"key_length"`
	UsedKeyParts   []string            `json:"used_key_parts,omitempty"`
	UsingIndex     bool                `json:"using_index"`
	SkipScan       bool                `json:"skip_scan"`
}

type mariadbTableCost struct {
	ReadCost string `json:"read_cost"`
	EvalCost string `json:"eval_cost"`
	PrefixCost string `json:"prefix_cost"`
}

func (a *MariaDBAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mariadb adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var explainQuery string
	if analyze {
		explainQuery = "EXPLAIN FORMAT=JSON " + query
	} else {
		explainQuery = "EXPLAIN FORMAT=JSON " + query
	}

	var rawJSON string
	if err := a.conn.QueryRowContext(queryCtx, explainQuery).Scan(&rawJSON); err != nil {
		return nil, fmt.Errorf("%w: mariadb explain: %w", db.ErrExplainFailed, err)
	}

	// Parse MariaDB's EXPLAIN FORMAT=JSON output
	// MariaDB's format differs from MySQL — use a flexible parser
	var planData map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &planData); err != nil {
		return nil, fmt.Errorf("%w: mariadb explain parse: %w", db.ErrExplainFailed, err)
	}

	plan := &db.ExplainPlan{
		Warnings: []string{},
	}

	// Extract query_block
	qb, ok := planData["query_block"].(map[string]any)
	if !ok {
		// Try alternate MariaDB structure
		plan.DialectRaw = rawJSON
		plan.EstimatedTotalCost = float64Ptr(0)
		plan.EstimatedRowsExamined = int64Ptr(0)
		return plan, nil
	}

	// Extract cost info
	if ci, ok := qb["cost_info"].(map[string]any); ok {
		if qc, ok := ci["query_cost"].(string); ok {
			var cost float64
			if _, err := fmt.Sscanf(qc, "%f", &cost); err == nil {
				plan.EstimatedTotalCost = &cost
			}
		}
	}

	// Walk table nodes to extract scan info
	extractMariaDBPlan(qb, plan)

	if plan.EstimatedTotalCost == nil {
		plan.EstimatedTotalCost = float64Ptr(0)
	}
	if plan.EstimatedRowsExamined == nil {
		plan.EstimatedRowsExamined = int64Ptr(0)
	}

	return plan, nil
}

func extractMariaDBPlan(node map[string]any, plan *db.ExplainPlan) {
	// Extract table
	if table, ok := node["table"].(map[string]any); ok {
		processMariaDBTable(table, plan)
	}

	// Extract nested_loop (array of table nodes)
	if nl, ok := node["nested_loop"].([]any); ok {
		for _, item := range nl {
			if itemMap, ok := item.(map[string]any); ok {
				extractMariaDBPlan(itemMap, plan)
			}
		}
	}

	// Extract tables (alternate format)
	if tables, ok := node["tables"].([]any); ok {
		for _, item := range tables {
			if itemMap, ok := item.(map[string]any); ok {
				extractMariaDBPlan(itemMap, plan)
			}
		}
	}

	// Extract dups (for UNION queries)
	if dups, ok := node["dups"].(map[string]any); ok {
		if table, ok := dups["table"].(map[string]any); ok {
			processMariaDBTable(table, plan)
		}
	}

	// Extract select_list subqueries
	if sl, ok := node["select_list"].([]any); ok {
		for _, item := range sl {
			if itemMap, ok := item.(map[string]any); ok {
				if subq, ok := itemMap["sub_query"].(map[string]any); ok {
					if qb, ok := subq["query_block"].(map[string]any); ok {
						extractMariaDBPlan(qb, plan)
					}
				}
			}
		}
	}

	// Extract subqueries in WHERE
	if where, ok := node["where_clause"].(map[string]any); ok {
		if sq, ok := where["sub_queries"].([]any); ok {
			for _, item := range sq {
				if itemMap, ok := item.(map[string]any); ok {
					if qb, ok := itemMap["query_block"].(map[string]any); ok {
						extractMariaDBPlan(qb, plan)
					}
				}
			}
		}
	}
}

func processMariaDBTable(table map[string]any, plan *db.ExplainPlan) {
	tableName, _ := table["table_name"].(string)
	accessType, _ := table["access_type"].(string)

	// Detect full table scan
	if accessType == "ALL" && tableName != "" {
		plan.FullScanTables = append(plan.FullScanTables, tableName)
	}

	// Detect index usage
	key, _ := table["key"].(string)
	if key != "" && key != "<auto_key>" {
		covering := false
		if ui, ok := table["using_index"].(bool); ok {
			covering = ui
		}
		entry := db.IndexUsageEntry{
			Table:      tableName,
			Index:      key,
			Covering:   covering,
			AccessType: accessType,
		}
		plan.IndexUsage = append(plan.IndexUsage, entry)
	}

	// Extract row estimates
	if re, ok := table["rows_examined_per_scan"].(float64); ok {
		rows := int64(re)
		plan.EstimatedRowsExamined = int64PtrAdd(plan.EstimatedRowsExamined, rows)
	}

	// Detect temp operations (Using temporary)
	if mt, ok := table["message_type"].(string); ok && mt == "TMP_TABLE" {
		plan.TempOperations++
	}

	// Detect sort operations
	if orderBy, ok := table["order_by"].(map[string]any); ok {
		if _, ok := orderBy["using_filesort"].(bool); ok {
			plan.SortOperations++
		}
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

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

func (a *MariaDBAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	// Layer 1: keyword scan
	if isDML(query) {
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: false,
			Errors:   []string{"DML/DCL statements are not permitted"},
		}, nil
	}

	if a.conn == nil {
		return &db.ValidateResult{
			Valid:    true,
			ReadOnly: true,
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Layer 2: EXPLAIN as schema check
	var rawJSON string
	err := a.conn.QueryRowContext(queryCtx, "EXPLAIN FORMAT=JSON "+query).Scan(&rawJSON)
	if err != nil {
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: true,
			Errors:   []string{err.Error()},
		}, nil
	}

	return &db.ValidateResult{
		Valid:    true,
		ReadOnly: true,
	}, nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func (a *MariaDBAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mariadb adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, TABLE_ROWS, AVG_ROW_LENGTH, DATA_LENGTH, INDEX_LENGTH,
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
		return nil, fmt.Errorf("mariadb stats: %w", err)
	}
	defer rows.Close()

	result := &db.StatsResult{}

	for rows.Next() {
		var tableName, updateTime string
		var tableRows, dataLength, indexLength int64

		if err := rows.Scan(&tableName, &tableRows, new(int64), &dataLength, &indexLength, &updateTime); err != nil {
			return nil, fmt.Errorf("mariadb stats scan: %w", err)
		}

		stats := db.TableStats{
			Name:                tableName,
			RowCount:            tableRows,
			CardinalityEstimate: tableRows,
			DataSizeBytes:       dataLength,
			IndexSizeBytes:      indexLength,
		}

		if updateTime != "" {
			stats.UpdatedAt = updateTime
			stats.Freshness = "fresh"
		}

		result.Tables = append(result.Tables, stats)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mariadb stats rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Indexes
// ---------------------------------------------------------------------------

func (a *MariaDBAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mariadb adapter not connected", db.ErrConnectionFailed)
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
		return nil, fmt.Errorf("mariadb indexes: %w", err)
	}
	defer rows.Close()

	tableMap := make(map[string]map[string]*db.IndexInfo)
	var tableOrder []string

	for rows.Next() {
		var tableName, indexName, columnName, indexType, indexComment, isVisible string
		var nonUnique, seqInIndex int
		var cardinality int64

		if err := rows.Scan(&tableName, &indexName, &nonUnique, &seqInIndex, &columnName,
			&cardinality, &indexType, &indexComment, &isVisible); err != nil {
			return nil, fmt.Errorf("mariadb indexes scan: %w", err)
		}

		idxMap, ok := tableMap[tableName]
		if !ok {
			idxMap = make(map[string]*db.IndexInfo)
			tableMap[tableName] = idxMap
			tableOrder = append(tableOrder, tableName)
		}

		idx, exists := idxMap[indexName]
		if !exists {
			isUnique := nonUnique == 0
			idx = &db.IndexInfo{
				Name: indexName,
				Type: indexType,
				IsUnique: isUnique,
				Primary: indexName == "PRIMARY",
				Visible: isVisible == "YES",
				Comment: indexComment,
			}
			idxMap[indexName] = idx
		}

		idx.Columns = append(idx.Columns, db.IndexColumn{
			Name:        columnName,
			Order:       "ASC",
			Sequence:    seqInIndex,
			Cardinality: cardinality,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mariadb indexes rows: %w", err)
	}

	result := &db.IndexesResult{}
	for _, name := range tableOrder {
		idxMap := tableMap[name]
		tableIdx := db.TableIndexInfo{
			Table: name,
		}
		for _, idx := range idxMap {
			tableIdx.Indexes = append(tableIdx.Indexes, *idx)
		}
		result.Tables = append(result.Tables, tableIdx)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Joins
// ---------------------------------------------------------------------------

func (a *MariaDBAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mariadb adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT k.TABLE_NAME, k.COLUMN_NAME,
		       k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME,
		       k.CONSTRAINT_NAME
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

	doubleArgs := make([]any, 0, len(tables)*2)
	for _, t := range tables {
		doubleArgs = append(doubleArgs, t)
	}
	for _, t := range tables {
		doubleArgs = append(doubleArgs, t)
	}

	rows, err := a.conn.QueryContext(queryCtx, query, doubleArgs...)
	if err != nil {
		return nil, fmt.Errorf("mariadb joins: %w", err)
	}
	defer rows.Close()

	type fkEdge struct {
		SourceTable string
		TargetTable string
		Columns     [][2]string
		Constraint  string
	}
	edgeMap := map[string]*fkEdge{}
	var edgeOrder []string

	for rows.Next() {
		var sourceTable, sourceCol, targetTable, targetCol, constraintName string
		if err := rows.Scan(&sourceTable, &sourceCol, &targetTable, &targetCol, &constraintName); err != nil {
			return nil, fmt.Errorf("mariadb joins scan: %w", err)
		}

		key := constraintName
		if edge, ok := edgeMap[key]; ok {
			edge.Columns = append(edge.Columns, [2]string{sourceCol, targetCol})
		} else {
			edgeMap[key] = &fkEdge{
				SourceTable: sourceTable,
				TargetTable: targetTable,
				Columns:     [][2]string{{sourceCol, targetCol}},
				Constraint:  constraintName,
			}
			edgeOrder = append(edgeOrder, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mariadb joins rows: %w", err)
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

// ---------------------------------------------------------------------------
// BuildDSN
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func int64PtrAdd(ptr *int64, v int64) *int64 {
	if ptr == nil {
		return &v
	}
	sum := *ptr + v
	return &sum
}
