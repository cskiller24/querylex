package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	db.Register("postgres", func(dsn string) (db.Adapter, error) {
		return &PostgreSQLAdapter{dsn: dsn}, nil
	})
	db.Register("postgresql", func(dsn string) (db.Adapter, error) {
		return &PostgreSQLAdapter{dsn: dsn}, nil
	})
}

type PostgreSQLAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *PostgreSQLAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("pgx", a.dsn)
	if err != nil {
		return fmt.Errorf("postgresql connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: postgresql ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *PostgreSQLAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: postgresql ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *PostgreSQLAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func (a *PostgreSQLAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: postgresql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. Tables and columns
	type colRow struct {
		SchemaName   string
		TableName    string
		RelKind      string
		ColumnName   string
		Ordinal      int
		ColumnType   string
		IsNullable   bool
		DefaultExpr  *string
		Comment      *string
		RowEstimate  int64
	}

	colQuery := `
		SELECT n.nspname,
		       c.relname,
		       c.relkind,
		       a.attname,
		       a.attnum,
		       format_type(a.atttypid, a.atttypmod),
		       NOT a.attnotnull,
		       pg_get_expr(ad.adbin, ad.adrelid),
		       col_description(c.oid, a.attnum),
		       c.reltuples::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid
		LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
		WHERE c.relkind IN ('r','v')
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog','information_schema')
	`
	colArgs := []any{}
	if len(tables) > 0 {
		colQuery += " AND c.relname = ANY($1)"
		colArgs = append(colArgs, tables)
	}
	colQuery += " ORDER BY c.relname, a.attnum"

	var colRows []colRow
	if len(tables) > 0 {
		rows, err := a.conn.QueryContext(queryCtx, colQuery, colArgs...)
		if err != nil {
			return nil, fmt.Errorf("postgresql schema columns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var r colRow
			if err := rows.Scan(&r.SchemaName, &r.TableName, &r.RelKind, &r.ColumnName,
				&r.Ordinal, &r.ColumnType, &r.IsNullable, &r.DefaultExpr, &r.Comment, &r.RowEstimate); err != nil {
				return nil, fmt.Errorf("postgresql schema scan: %w", err)
			}
			colRows = append(colRows, r)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("postgresql schema rows: %w", err)
		}
	}

	// 2. Primary keys
	pkQuery := `
		SELECT tc.table_name, kc.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kc
		  ON tc.constraint_name = kc.constraint_name
		 AND tc.table_schema = kc.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
	`
	if len(tables) > 0 {
		pkQuery += " AND tc.table_name = ANY($1)"
	}
	pkRows, err := a.conn.QueryContext(queryCtx, pkQuery)
	if err != nil {
		return nil, fmt.Errorf("postgresql schema pks: %w", err)
	}
	defer pkRows.Close()

	pkMap := map[string]map[string]bool{} // table -> column -> true
	for pkRows.Next() {
		var t, c string
		if err := pkRows.Scan(&t, &c); err != nil {
			return nil, fmt.Errorf("postgresql pk scan: %w", err)
		}
		if pkMap[t] == nil {
			pkMap[t] = map[string]bool{}
		}
		pkMap[t][c] = true
	}
	if err := pkRows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql pk rows: %w", err)
	}

	// 3. Foreign keys
	fkQuery := `
		SELECT
		    cl.relname AS source_table,
		    a.attname AS source_column,
		    f.confrelid::regclass::text AS target_table,
		    af.attname AS target_column,
		    con.conname AS constraint_name
		FROM pg_constraint con
		JOIN pg_class cl ON cl.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = cl.relnamespace
		JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY(con.conkey)
		JOIN pg_class ft ON ft.oid = con.confrelid
		JOIN pg_attribute af ON af.attrelid = con.confrelid AND af.attnum = ANY(con.confkey)
		WHERE con.contype = 'f'
		  AND n.nspname NOT IN ('pg_catalog','information_schema')
	`
	if len(tables) > 0 {
		fkQuery += " AND cl.relname = ANY($1)"
	}
	fkRows, err := a.conn.QueryContext(queryCtx, fkQuery)
	if err != nil {
		return nil, fmt.Errorf("postgresql schema fks: %w", err)
	}
	defer fkRows.Close()

	type fkRow struct {
		SourceTable    string
		SourceColumn   string
		TargetTable    string
		TargetColumn   string
		ConstraintName string
	}
	var fkList []fkRow
	for fkRows.Next() {
		var r fkRow
		if err := fkRows.Scan(&r.SourceTable, &r.SourceColumn, &r.TargetTable, &r.TargetColumn, &r.ConstraintName); err != nil {
			return nil, fmt.Errorf("postgresql fk scan: %w", err)
		}
		fkList = append(fkList, r)
	}
	if err := fkRows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql fk rows: %w", err)
	}

	// 4. Indexes
	idxQuery := `
		SELECT
		    t.tablename,
		    t.indexname,
		    t.indexdef,
		    i.indisunique,
		    i.indisvalid
		FROM pg_indexes t
		JOIN pg_index i ON i.indexrelid = (t.indexname::regclass)
		WHERE t.schemaname NOT IN ('pg_catalog','information_schema')
	`
	if len(tables) > 0 {
		idxQuery += " AND t.tablename = ANY($1)"
	}
	idxQuery += " ORDER BY t.tablename, t.indexname"
	idxRows, err := a.conn.QueryContext(queryCtx, idxQuery)
	if err != nil {
		return nil, fmt.Errorf("postgresql schema indexes: %w", err)
	}
	defer idxRows.Close()

	type idxInfo struct {
		Name      string
		IsUnique  bool
		IsValid   bool
		IndexDef  string
	}
	indexMap := map[string][]idxInfo{} // table -> indexes
	for idxRows.Next() {
		var tableName, indexName, indexDef string
		var isUnique, isValid bool
		if err := idxRows.Scan(&tableName, &indexName, &indexDef, &isUnique, &isValid); err != nil {
			return nil, fmt.Errorf("postgresql idx scan: %w", err)
		}
		indexMap[tableName] = append(indexMap[tableName], idxInfo{
			Name:     indexName,
			IsUnique: isUnique,
			IsValid:  isValid,
			IndexDef: indexDef,
		})
	}
	if err := idxRows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql idx rows: %w", err)
	}

	// 5. Views
	viewQuery := `
		SELECT n.nspname, c.relname, pg_get_viewdef(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'v'
		  AND n.nspname NOT IN ('pg_catalog','information_schema')
	`
	if len(tables) > 0 {
		viewQuery += " AND c.relname = ANY($1)"
	}
	vRows, err := a.conn.QueryContext(queryCtx, viewQuery)
	if err != nil {
		return nil, fmt.Errorf("postgresql schema views: %w", err)
	}
	defer vRows.Close()

	var views []db.ViewInfo
	for vRows.Next() {
		var schema, name, def string
		if err := vRows.Scan(&schema, &name, &def); err != nil {
			return nil, fmt.Errorf("postgresql view scan: %w", err)
		}
		views = append(views, db.ViewInfo{Schema: schema, Name: name, Def: def})
	}
	if err := vRows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql view rows: %w", err)
	}

	// --- Assemble result ---
	tableMap := map[string]*db.TableInfo{}
	var tableOrder []string

	for _, r := range colRows {
		ti, ok := tableMap[r.TableName]
		if !ok {
			tableType := "BASE TABLE"
			if r.RelKind == "v" {
				tableType = "VIEW"
			}
			ti = &db.TableInfo{
				Schema:          r.SchemaName,
				Name:            r.TableName,
				Type:            tableType,
				RowCountEstimate: r.RowEstimate,
			}
			tableMap[r.TableName] = ti
			tableOrder = append(tableOrder, r.TableName)
		}

		col := db.ColumnInfo{
			Name:       r.ColumnName,
			Ordinal:    r.Ordinal,
			ColumnType: r.ColumnType,
			IsNullable: r.IsNullable,
			IsPrimaryKey: pkMap[r.TableName] != nil && pkMap[r.TableName][r.ColumnName],
		}
		if r.DefaultExpr != nil {
			col.Default = *r.DefaultExpr
		}
		if r.Comment != nil {
			col.Comment = *r.Comment
		}
		ti.Columns = append(ti.Columns, col)
	}

	// Add PK constraints
	for _, ti := range tableMap {
		var pkCols []string
		for _, col := range ti.Columns {
			if col.IsPrimaryKey {
				pkCols = append(pkCols, col.Name)
			}
		}
		if len(pkCols) > 0 {
			ti.Constraints = append(ti.Constraints, db.ConstraintInfo{
				Name:    ti.Name + "_pkey",
				Type:    "PRIMARY_KEY",
				Columns: pkCols,
			})
		}
	}

	// Add FK constraints
	fkGroups := map[string]*db.ConstraintInfo{} // constraintName -> info
	for _, fk := range fkList {
		key := fk.ConstraintName
		if ci, ok := fkGroups[key]; ok {
			ci.Columns = append(ci.Columns, fk.SourceColumn)
			ci.ReferencedColumns = append(ci.ReferencedColumns, fk.TargetColumn)
		} else {
			fkGroups[key] = &db.ConstraintInfo{
				Name:              fk.ConstraintName,
				Type:              "FOREIGN_KEY",
				Columns:           []string{fk.SourceColumn},
				ReferencedTable:   fk.TargetTable,
				ReferencedColumns: []string{fk.TargetColumn},
			}
		}
	}
	// Attach FKs to source tables
	for _, fk := range fkList {
		ti, ok := tableMap[fk.SourceTable]
		if !ok {
			continue
		}
		key := fk.ConstraintName
		found := false
		for i := range ti.Constraints {
			if ti.Constraints[i].Name == key {
				found = true
				break
			}
		}
		if !found {
			ci := fkGroups[key]
			if ci != nil {
				ti.Constraints = append(ti.Constraints, *ci)
			}
		}
	}

	// Attach indexes to tables
	parseIndexDef := func(def string) (string, []string) {
		// Extract index type and columns from CREATE INDEX definition
		idxType := "btree" // PostgreSQL default
		idxTypeLower := strings.ToLower(def)
		if strings.Contains(idxTypeLower, " using hash") {
			idxType = "hash"
		} else if strings.Contains(idxTypeLower, " using gist") {
			idxType = "gist"
		} else if strings.Contains(idxTypeLower, " using gin") {
			idxType = "gin"
		} else if strings.Contains(idxTypeLower, " using brin") {
			idxType = "brin"
		}
		return idxType, nil
	}

	for tname, idxs := range indexMap {
		ti, ok := tableMap[tname]
		if !ok {
			continue
		}
		for _, idx := range idxs {
			idxType, _ := parseIndexDef(idx.IndexDef)
			ii := db.IndexInfo{
				Name:     idx.Name,
				Type:     idxType,
				IsUnique: idx.IsUnique,
				Primary:  strings.HasPrefix(idx.Name, tname+"_pkey"),
				Visible:  idx.IsValid,
			}
			ti.Indexes = append(ti.Indexes, ii)
		}
	}

	// Build result
	result := &db.SchemaResult{Tables: make([]db.TableInfo, 0, len(tableOrder))}
	for _, name := range tableOrder {
		result.Tables = append(result.Tables, *tableMap[name])
	}
	for _, v := range views {
		result.Views = append(result.Views, v)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Explain
// ---------------------------------------------------------------------------

type pgExplainPlanJSON struct {
	Plan pgPlanNode `json:"Plan"`
}

type pgPlanNode struct {
	NodeType     string        `json:"Node Type"`
	RelationName string        `json:"Relation Name"`
	Alias        string        `json:"Alias"`
	StartupCost  float64       `json:"Startup Cost"`
	TotalCost    float64       `json:"Total Cost"`
	PlanRows     float64       `json:"Plan Rows"`
	PlanWidth    int           `json:"Plan Width"`
	ActualStart  float64       `json:"Actual Startup Time"`
	ActualTotal  float64       `json:"Actual Total Time"`
	ActualRows   float64       `json:"Actual Rows"`
	ActualLoops  float64       `json:"Actual Loops"`
	Plans        []pgPlanNode  `json:"Plans,omitempty"`
}

func (a *PostgreSQLAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: postgresql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var explainQuery string
	if analyze {
		explainQuery = "EXPLAIN (ANALYZE, FORMAT JSON) " + query
	} else {
		explainQuery = "EXPLAIN (FORMAT JSON) " + query
	}

	var rawJSON string
	if err := a.conn.QueryRowContext(queryCtx, explainQuery).Scan(&rawJSON); err != nil {
		return nil, fmt.Errorf("%w: postgresql explain: %w", db.ErrExplainFailed, err)
	}

	var plans []pgExplainPlanJSON
	if err := json.Unmarshal([]byte(rawJSON), &plans); err != nil {
		return nil, fmt.Errorf("%w: postgresql explain parse: %w", db.ErrExplainFailed, err)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("%w: empty explain plan", db.ErrExplainFailed)
	}

	return buildExplainPlan(&plans[0].Plan, analyze), nil
}

func buildExplainPlan(node *pgPlanNode, analyze bool) *db.ExplainPlan {
	plan := &db.ExplainPlan{
		EstimatedTotalCost:    &node.TotalCost,
		EstimatedRowsExamined: int64Ptr(int64(node.PlanRows)),
	}

	if analyze && node.ActualTotal > 0 {
		actualTime := node.ActualTotal
		plan.ActualTotalTimeMs = &actualTime
		actualRows := int64(node.ActualRows)
		plan.ActualRowsExamined = &actualRows
	}

	walkPlanNode(node, plan)

	return plan
}

func int64Ptr(v int64) *int64 {
	return &v
}

func walkPlanNode(node *pgPlanNode, plan *db.ExplainPlan) {
	switch node.NodeType {
	case "Seq Scan":
		name := node.RelationName
		if name == "" {
			name = node.Alias
		}
		if name != "" {
			plan.FullScanTables = append(plan.FullScanTables, name)
		}
	case "Index Scan", "Index Only Scan":
		table := node.RelationName
		if table == "" {
			table = node.Alias
		}
		plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
			Table:      table,
			Index:      "", // Not directly available in simple EXPLAIN JSON
			Covering:   node.NodeType == "Index Only Scan",
			AccessType: "index_scan",
		})
	case "Bitmap Heap Scan":
		// covered by child Bitmap Index Scan
	case "Bitmap Index Scan":
		table := node.RelationName
		if table == "" {
			table = node.Alias
		}
		plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
			Table:      table,
			AccessType: "bitmap_scan",
		})
	case "Sort":
		plan.SortOperations++
	case "Hash":
		plan.TempOperations++
	case "Nested Loop":
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Nested Loop",
		})
	case "Hash Join":
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Hash Join",
		})
	case "Merge Join":
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Merge Join",
		})
	}

	for i := range node.Plans {
		walkPlanNode(&node.Plans[i], plan)
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
			// Check that the keyword is at a word boundary
			if len(upper) == len(kw) || (upper[len(kw)] == ' ' || upper[len(kw)] == '\t' || upper[len(kw)] == '\n' || upper[len(kw)] == '(') {
				return true
			}
		}
	}
	return false
}

func (a *PostgreSQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	// Layer 1: keyword scan
	if isDML(query) {
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: false,
			Errors:   []string{"DML/DCL statements are not permitted"},
		}, nil
	}

	if a.conn == nil {
		// Without connection, we can still do keyword validation
		return &db.ValidateResult{
			Valid:    true,
			ReadOnly: true,
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Layer 2: Execute EXPLAIN (FORMAT JSON) as schema check
	var rawJSON string
	err := a.conn.QueryRowContext(queryCtx, "EXPLAIN (FORMAT JSON) "+query).Scan(&rawJSON)
	if err != nil {
		errMsg := err.Error()
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: true,
			Errors:   []string{errMsg},
		}, nil
	}

	// Parse plan to extract table/column references
	result := &db.ValidateResult{
		Valid:    true,
		ReadOnly: true,
	}

	var plans []pgExplainPlanJSON
	if json.Unmarshal([]byte(rawJSON), &plans) == nil && len(plans) > 0 {
		collectTableRefs(&plans[0].Plan, result)
	}

	return result, nil
}

func collectTableRefs(node *pgPlanNode, result *db.ValidateResult) {
	if node.RelationName != "" {
		result.Tables = append(result.Tables, node.RelationName)
	}
	for i := range node.Plans {
		collectTableRefs(&node.Plans[i], result)
	}
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func (a *PostgreSQLAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: postgresql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
		    s.relname,
		    COALESCE(s.n_live_tup, 0),
		    COALESCE(c.reltuples, 0)::bigint,
		    COALESCE(c.relpages, 0),
		    s.last_analyze,
		    s.last_autoanalyze,
		    COALESCE(pg_indexes_size(c.oid), 0)
		FROM pg_stat_user_tables s
		JOIN pg_class c ON c.oid = s.relid
	`
	if len(tables) > 0 {
		query += " WHERE s.relname = ANY($1)"
	}
	query += " ORDER BY s.relname"

	args := []any{}
	if len(tables) > 0 {
		args = append(args, tables)
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgresql stats: %w", err)
	}
	defer rows.Close()

	result := &db.StatsResult{}
	for rows.Next() {
		var tableName string
		var liveTup, relTuples, relPages, idxSize int64
		var lastAnalyze, lastAutoAnalyze *string

		if err := rows.Scan(&tableName, &liveTup, &relTuples, &relPages, &lastAnalyze, &lastAutoAnalyze, &idxSize); err != nil {
			return nil, fmt.Errorf("postgresql stats scan: %w", err)
		}

		stats := db.TableStats{
			Name:                tableName,
			RowCount:            liveTup,
			CardinalityEstimate: relTuples,
			DataSizeBytes:       relPages * 8192,
			IndexSizeBytes:      idxSize,
		}
		if lastAnalyze != nil && *lastAnalyze != "" {
			stats.Freshness = "fresh"
			stats.UpdatedAt = *lastAnalyze
		} else if lastAutoAnalyze != nil && *lastAutoAnalyze != "" {
			stats.Freshness = "fresh"
			stats.UpdatedAt = *lastAutoAnalyze
		}

		result.Tables = append(result.Tables, stats)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql stats rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Indexes
// ---------------------------------------------------------------------------

func (a *PostgreSQLAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: postgresql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
		    t.tablename,
		    t.indexname,
		    t.indexdef,
		    i.indisunique,
		    i.indisvalid,
		    COALESCE(s.idx_scan, 0),
		    COALESCE(s.idx_tup_read, 0)
		FROM pg_indexes t
		JOIN pg_index i ON i.indexrelid = (t.indexname::regclass)
		LEFT JOIN pg_stat_user_indexes s ON s.indexrelid = i.indexrelid
		WHERE t.schemaname NOT IN ('pg_catalog','information_schema')
	`
	if len(tables) > 0 {
		query += " AND t.tablename = ANY($1)"
	}
	query += " ORDER BY t.tablename, t.indexname"

	args := []any{}
	if len(tables) > 0 {
		args = append(args, tables)
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgresql indexes: %w", err)
	}
	defer rows.Close()

	type idxAccum struct {
		idx db.IndexInfo
	}
	tableIdxMap := map[string][]*idxAccum{}
	var tableOrder []string

	for rows.Next() {
		var tableName, indexName, indexDef string
		var isUnique, isValid bool
		var idxScan, idxTupRead int64

		if err := rows.Scan(&tableName, &indexName, &indexDef, &isUnique, &isValid, &idxScan, &idxTupRead); err != nil {
			return nil, fmt.Errorf("postgresql indexes scan: %w", err)
		}

		idxType := "btree"
		defLower := strings.ToLower(indexDef)
		switch {
		case strings.Contains(defLower, " using hash"):
			idxType = "hash"
		case strings.Contains(defLower, " using gist"):
			idxType = "gist"
		case strings.Contains(defLower, " using gin"):
			idxType = "gin"
		case strings.Contains(defLower, " using brin"):
			idxType = "brin"
		case strings.Contains(defLower, " using sp-gist"):
			idxType = "sp-gist"
		}

		ii := db.IndexInfo{
			Name:     indexName,
			Type:     idxType,
			IsUnique: isUnique,
			Primary:  strings.HasSuffix(indexName, "_pkey"),
			Visible:  isValid,
		}
		_ = idxScan
		_ = idxTupRead

		if _, ok := tableIdxMap[tableName]; !ok {
			tableIdxMap[tableName] = []*idxAccum{}
			tableOrder = append(tableOrder, tableName)
		}
		tableIdxMap[tableName] = append(tableIdxMap[tableName], &idxAccum{idx: ii})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresql indexes rows: %w", err)
	}

	result := &db.IndexesResult{}
	for _, name := range tableOrder {
		tableIdx := db.TableIndexInfo{Table: name}
		for _, ia := range tableIdxMap[name] {
			tableIdx.Indexes = append(tableIdx.Indexes, ia.idx)
		}
		result.Tables = append(result.Tables, tableIdx)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Joins
// ---------------------------------------------------------------------------

func (a *PostgreSQLAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: postgresql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Query foreign key constraints as join edges
	query := `
		SELECT
		    cl.relname AS source_table,
		    a.attname AS source_column,
		    ft.relname AS target_table,
		    af.attname AS target_column,
		    con.conname AS constraint_name
		FROM pg_constraint con
		JOIN pg_class cl ON cl.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = cl.relnamespace
		JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY(con.conkey)
		JOIN pg_class ft ON ft.oid = con.confrelid
		JOIN pg_attribute af ON af.attrelid = con.confrelid AND af.attnum = ANY(con.confkey)
		WHERE con.contype = 'f'
		  AND n.nspname NOT IN ('pg_catalog','information_schema')
	`
	if len(tables) > 0 {
		query += " AND (cl.relname = ANY($1) OR ft.relname = ANY($1))"
	}

	args := []any{}
	if len(tables) > 0 {
		args = append(args, tables)
	}

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgresql joins: %w", err)
	}
	defer rows.Close()

	// Group FK columns by constraint name to detect composite FKs
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
			return nil, fmt.Errorf("postgresql joins scan: %w", err)
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
		return nil, fmt.Errorf("postgresql joins rows: %w", err)
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
// DatabaseType / BuildDSN
// ---------------------------------------------------------------------------

func (a *PostgreSQLAdapter) DatabaseType() string {
	return "postgresql"
}

func BuildDSN(host string, port int, database, username, password string, sslMode string) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   database,
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	q := u.Query()
	if sslMode != "" {
		q.Set("sslmode", sslMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
