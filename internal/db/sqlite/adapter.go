package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "modernc.org/sqlite"
)

func init() {
	db.Register("sqlite", func(dsn string) (db.Adapter, error) {
		return &SQLiteAdapter{dsn: dsn}, nil
	})
}

type SQLiteAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *SQLiteAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("sqlite", a.dsn)
	if err != nil {
		return fmt.Errorf("sqlite connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: sqlite ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *SQLiteAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: sqlite ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *SQLiteAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Table name validation
// ---------------------------------------------------------------------------

// validateTables checks that all given table names exist in sqlite_master.
// This is a security mitigation against PRAGMA injection (T-02-02b).
func (a *SQLiteAdapter) validateTables(ctx context.Context, tables []string) (map[string]bool, error) {
	if len(tables) == 0 {
		return nil, nil
	}

	rows, err := a.conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("sqlite validate tables: %w", err)
	}
	defer rows.Close()

	valid := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("sqlite validate tables scan: %w", err)
		}
		valid[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite validate tables rows: %w", err)
	}

	for _, t := range tables {
		if !valid[t] {
			return nil, fmt.Errorf("%w: table not found: %s", db.ErrInvalidSQL, t)
		}
	}

	return valid, nil
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func (a *SQLiteAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: sqlite adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get all user tables
	tableQuery := "SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	if len(tables) > 0 {
		// Build IN clause safely (table names pre-validated)
		placeholders := make([]string, len(tables))
		args := make([]any, len(tables))
		for i, t := range tables {
			placeholders[i] = "?"
			args[i] = t
		}
		tableQuery += " AND name IN (" + strings.Join(placeholders, ",") + ")"
		tableQuery += " ORDER BY name"
		tableRows, err := a.conn.QueryContext(queryCtx, tableQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("sqlite schema tables: %w", err)
		}
		defer tableRows.Close()
	} else {
		tableQuery += " ORDER BY name"
	}

	// Query tables
	args := []any{}
	for _, t := range tables {
		args = append(args, t)
	}

	tableRows, err := a.conn.QueryContext(queryCtx, tableQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite schema tables: %w", err)
	}
	defer tableRows.Close()

	type tableDef struct {
		name string
		sql  string
	}
	var userTables []tableDef
	for tableRows.Next() {
		var name, sqlDef string
		if err := tableRows.Scan(&name, &sqlDef); err != nil {
			return nil, fmt.Errorf("sqlite schema table scan: %w", err)
		}
		userTables = append(userTables, tableDef{name: name, sql: sqlDef})
	}
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite schema table rows: %w", err)
	}

	result := &db.SchemaResult{}

	for _, ut := range userTables {
		tableName := ut.name

		// Validate table name before PRAGMA (security)
		if _, err := a.validateTables(ctx, []string{tableName}); err != nil {
			continue
		}

		// PRAGMA table_info — columns
		colRows, err := a.conn.Query("PRAGMA table_info(\"" + tableName + "\")")
		if err != nil {
			return nil, fmt.Errorf("sqlite pragma table_info(%s): %w", tableName, err)
		}

		ti := db.TableInfo{
			Name: tableName,
			Type: "BASE TABLE",
		}

		var colList []db.ColumnInfo
		for colRows.Next() {
			var cid int
			var name, colType string
			var notNull bool
			var defaultVal *string
			var pk int
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
				colRows.Close()
				return nil, fmt.Errorf("sqlite pragma table_info scan: %w", err)
			}

			col := db.ColumnInfo{
				Name:         name,
				Ordinal:      cid + 1,
				ColumnType:   colType,
				IsNullable:   !notNull,
				IsPrimaryKey: pk > 0,
			}
			if defaultVal != nil {
				col.Default = *defaultVal
			}
			colList = append(colList, col)
		}
		colRows.Close()
		ti.Columns = colList

		// Add PK constraint
		var pkCols []string
		for _, c := range colList {
			if c.IsPrimaryKey {
				pkCols = append(pkCols, c.Name)
			}
		}
		if len(pkCols) > 0 {
			ti.Constraints = append(ti.Constraints, db.ConstraintInfo{
				Name:    tableName + "_pkey",
				Type:    "PRIMARY_KEY",
				Columns: pkCols,
			})
		}

		// PRAGMA index_list — indexes
		idxRows, err := a.conn.Query("PRAGMA index_list(\"" + tableName + "\")")
		if err != nil {
			return nil, fmt.Errorf("sqlite pragma index_list(%s): %w", tableName, err)
		}

		for idxRows.Next() {
			var seq int
			var idxName string
			var unique int
			var origin, partial string
			if err := idxRows.Scan(&seq, &idxName, &unique, &origin, &partial); err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("sqlite pragma index_list scan: %w", err)
			}

			ii := db.IndexInfo{
				Name:     idxName,
				Type:     "btree",
				IsUnique: unique == 1,
				Primary:  origin == "pk",
				Visible:  true,
				IsPartial: partial == "1",
			}

			// PRAGMA index_info for columns
			iiRows, err := a.conn.Query("PRAGMA index_info(\"" + idxName + "\")")
			if err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("sqlite pragma index_info(%s): %w", idxName, err)
			}

			for iiRows.Next() {
				var seqno, cid int
				var colName string
				if err := iiRows.Scan(&seqno, &cid, &colName); err != nil {
					iiRows.Close()
					idxRows.Close()
					return nil, fmt.Errorf("sqlite pragma index_info scan: %w", err)
				}
				ii.Columns = append(ii.Columns, db.IndexColumn{
					Name:     colName,
					Order:    "ASC",
					Sequence: seqno,
				})
			}
			iiRows.Close()

			ti.Indexes = append(ti.Indexes, ii)
		}
		idxRows.Close()

		// PRAGMA foreign_key_list — FK constraints
		fkRows, err := a.conn.Query("PRAGMA foreign_key_list(\"" + tableName + "\")")
		if err != nil {
			return nil, fmt.Errorf("sqlite pragma foreign_key_list(%s): %w", tableName, err)
		}

		type fkCol struct {
			from string
			to   string
		}
		fkGroups := make(map[int]*struct {
			table string
			cols  []fkCol
		})

		for fkRows.Next() {
			var id, seq int
			var fkTable, fromCol, toCol string
			var onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &fkTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				fkRows.Close()
				return nil, fmt.Errorf("sqlite pragma foreign_key_list scan: %w", err)
			}

			if fkGroups[id] == nil {
				fkGroups[id] = &struct {
					table string
					cols  []fkCol
				}{table: fkTable}
			}
			fkGroups[id].cols = append(fkGroups[id].cols, fkCol{from: fromCol, to: toCol})
		}
		fkRows.Close()

		for _, fkg := range fkGroups {
			var cols, refCols []string
			for _, c := range fkg.cols {
				cols = append(cols, c.from)
				refCols = append(refCols, c.to)
			}
			ti.Constraints = append(ti.Constraints, db.ConstraintInfo{
				Name:              fmt.Sprintf("fk_%s_%s", tableName, fkg.table),
				Type:              "FOREIGN_KEY",
				Columns:           cols,
				ReferencedTable:   fkg.table,
				ReferencedColumns: refCols,
			})
		}

		result.Tables = append(result.Tables, ti)
	}

	// Views
	viewRows, err := a.conn.QueryContext(queryCtx, "SELECT name, sql FROM sqlite_master WHERE type='view'")
	if err != nil {
		return nil, fmt.Errorf("sqlite schema views: %w", err)
	}
	defer viewRows.Close()

	for viewRows.Next() {
		var name, sqlDef string
		if err := viewRows.Scan(&name, &sqlDef); err != nil {
			return nil, fmt.Errorf("sqlite view scan: %w", err)
		}
		result.Views = append(result.Views, db.ViewInfo{Name: name, Def: sqlDef})
	}
	if err := viewRows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite view rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Explain
// ---------------------------------------------------------------------------

var (
	scanTableRe   = regexp.MustCompile(`SCAN TABLE\s+(\S+)`)
	usingIdxRe    = regexp.MustCompile(`USING COVERING INDEX\s+(\S+)`)
	usingIdx2Re   = regexp.MustCompile(`USING INDEX\s+(\S+)`)
	searchIdxRe   = regexp.MustCompile(`SEARCH TABLE\s+(\S+)\s+USING INDEX\s+(\S+)`)
)

func (a *SQLiteAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: sqlite adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if analyze {
		// SQLite has no runtime plan analysis — warn but proceed
		plan, err := a.doExplain(queryCtx, query)
		if plan != nil {
			plan.Warnings = append(plan.Warnings, "SQLite does not support runtime plan analysis (ANALYZE mode). Estimated plan shown.")
		}
		return plan, err
	}

	return a.doExplain(queryCtx, query)
}

func (a *SQLiteAdapter) doExplain(ctx context.Context, query string) (*db.ExplainPlan, error) {
	explainQuery := "EXPLAIN QUERY PLAN " + query

	rows, err := a.conn.QueryContext(ctx, explainQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: sqlite explain: %w", db.ErrExplainFailed, err)
	}
	defer rows.Close()

	plan := &db.ExplainPlan{
		EstimatedTotalCost:    float64Ptr(0),
		EstimatedRowsExamined: int64Ptr(0),
	}

	// Estimated cost heuristics:
	// SCAN TABLE → high cost (1000 per scan)
	// USING COVERING INDEX → low cost (10)
	// USING INDEX → medium cost (100)
	cost := 0.0
	rowsExamined := int64(0)
	scanCount := 0

	for rows.Next() {
		var id, parent int
		var detail string
		var notUsed *string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			return nil, fmt.Errorf("sqlite explain scan: %w", err)
		}

		plan.DialectRaw = detail

		// Parse SCAN TABLE
		if m := scanTableRe.FindStringSubmatch(detail); len(m) > 0 {
			plan.FullScanTables = append(plan.FullScanTables, m[1])
			cost += 1000
			rowsExamined += 1000
			scanCount++
		}

		// Parse SEARCH TABLE ... USING INDEX (index seek)
		if m := searchIdxRe.FindStringSubmatch(detail); len(m) > 0 {
			plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
				Table:      m[1],
				Index:      m[2],
				AccessType: "index_seek",
			})
			cost += 50
			rowsExamined += 10
		}

		// Parse USING COVERING INDEX
		if m := usingIdxRe.FindStringSubmatch(detail); len(m) > 0 {
			plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
				Index:      m[1],
				Covering:   true,
				AccessType: "covering_index",
			})
			cost += 10
		}

		// Parse USING INDEX (non-covering)
		if m := usingIdx2Re.FindStringSubmatch(detail); len(m) > 0 {
			// Check if this isn't already covered by searchIdxRe
			alreadyFound := false
			for _, iu := range plan.IndexUsage {
				if iu.Index == m[1] {
					alreadyFound = true
					break
				}
			}
			if !alreadyFound {
				plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
					Index:      m[1],
					AccessType: "index_scan",
				})
				cost += 100
				rowsExamined += 100
			}
		}

		// Detect sort operations
		if strings.Contains(detail, "ORDER BY") || strings.Contains(detail, "USE TEMP") {
			plan.SortOperations++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite explain rows: %w", err)
	}

	if cost > 0 {
		plan.EstimatedTotalCost = &cost
	}
	if rowsExamined > 0 {
		plan.EstimatedRowsExamined = &rowsExamined
	}

	return plan, nil
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

func (a *SQLiteAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
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

	// Layer 2: Execute EXPLAIN <query> (bytecode version)
	// This validates that the query parses correctly
	rows, err := a.conn.QueryContext(queryCtx, "EXPLAIN "+query)
	if err != nil {
		return &db.ValidateResult{
			Valid:    false,
			ReadOnly: true,
			Errors:   []string{err.Error()},
		}, nil
	}
	defer rows.Close()

	// Query parsed successfully
	result := &db.ValidateResult{
		Valid:    true,
		ReadOnly: true,
	}

	// Extract table references from the original query (simple heuristic)
	// More sophisticated extraction would parse the EXPLAIN output
	upper := strings.ToUpper(query)
	if strings.Contains(upper, "FROM") || strings.Contains(upper, "JOIN") {
		// Table references exist — basic extraction
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func (a *SQLiteAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: sqlite adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get all user tables
	tableQuery := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	if len(tables) > 0 {
		placeholders := make([]string, len(tables))
		args := make([]any, len(tables))
		for i, t := range tables {
			placeholders[i] = "?"
			args[i] = t
		}
		tableQuery += " AND name IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := a.conn.QueryContext(queryCtx, tableQuery)
	if err != nil {
		return nil, fmt.Errorf("sqlite stats tables: %w", err)
	}
	defer rows.Close()

	result := &db.StatsResult{}
	now := time.Now().UTC().Format(time.RFC3339)

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("sqlite stats scan: %w", err)
		}

		// COUNT(*) per table — can be expensive on large tables
		var rowCount int64
		countErr := a.conn.QueryRowContext(queryCtx, "SELECT COUNT(*) FROM \""+tableName+"\"").Scan(&rowCount)
		if countErr != nil {
			// Non-fatal — proceed with 0
			rowCount = 0
		}

		result.Tables = append(result.Tables, db.TableStats{
			Name:                tableName,
			RowCount:            rowCount,
			CardinalityEstimate: rowCount,
			DataSizeBytes:       0, // Not available in SQLite
			IndexSizeBytes:      0, // Not available in SQLite
			Freshness:           "current",
			UpdatedAt:           now,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite stats rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Indexes
// ---------------------------------------------------------------------------

func (a *SQLiteAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: sqlite adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get table list
	var tableNames []string
	if len(tables) > 0 {
		tableNames = tables
	} else {
		rows, err := a.conn.QueryContext(queryCtx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			return nil, fmt.Errorf("sqlite indexes tables: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, fmt.Errorf("sqlite indexes table scan: %w", err)
			}
			tableNames = append(tableNames, name)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("sqlite indexes table rows: %w", err)
		}
	}

	result := &db.IndexesResult{}

	for _, tableName := range tableNames {
		// Validate table before PRAGMA
		if _, err := a.validateTables(ctx, []string{tableName}); err != nil {
			continue
		}

		tableIdx := db.TableIndexInfo{Table: tableName}

		// PRAGMA index_list
		idxRows, err := a.conn.Query("PRAGMA index_list(\"" + tableName + "\")")
		if err != nil {
			return nil, fmt.Errorf("sqlite pragma index_list(%s): %w", tableName, err)
		}

		for idxRows.Next() {
			var seq int
			var idxName string
			var unique int
			var origin, partial string
			if err := idxRows.Scan(&seq, &idxName, &unique, &origin, &partial); err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("sqlite pragma index_list scan: %w", err)
			}

			ii := db.IndexInfo{
				Name:     idxName,
				Type:     "btree",
				IsUnique: unique == 1,
				Primary:  origin == "pk",
				Visible:  true,
				IsPartial: partial == "1",
			}

			// PRAGMA index_info
			iiRows, err := a.conn.Query("PRAGMA index_info(\"" + idxName + "\")")
			if err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("sqlite pragma index_info(%s): %w", idxName, err)
			}

			for iiRows.Next() {
				var seqno, cid int
				var colName string
				if err := iiRows.Scan(&seqno, &cid, &colName); err != nil {
					iiRows.Close()
					idxRows.Close()
					return nil, fmt.Errorf("sqlite pragma index_info scan: %w", err)
				}
				ii.Columns = append(ii.Columns, db.IndexColumn{
					Name:     colName,
					Order:    "ASC",
					Sequence: seqno,
				})
			}
			iiRows.Close()

			tableIdx.Indexes = append(tableIdx.Indexes, ii)
		}
		idxRows.Close()

		result.Tables = append(result.Tables, tableIdx)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Joins
// ---------------------------------------------------------------------------

func (a *SQLiteAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: sqlite adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get all user tables
	var tableNames []string
	if len(tables) > 0 {
		tableNames = tables
	} else {
		rows, err := a.conn.QueryContext(queryCtx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			return nil, fmt.Errorf("sqlite joins tables: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, fmt.Errorf("sqlite joins table scan: %w", err)
			}
			tableNames = append(tableNames, name)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("sqlite joins table rows: %w", err)
		}
	}

	result := &db.JoinsResult{}

	for _, tableName := range tableNames {
		// Validate table before PRAGMA
		if _, err := a.validateTables(ctx, []string{tableName}); err != nil {
			continue
		}

		// PRAGMA foreign_key_list
		fkRows, err := a.conn.Query("PRAGMA foreign_key_list(\"" + tableName + "\")")
		if err != nil {
			return nil, fmt.Errorf("sqlite pragma foreign_key_list(%s): %w", tableName, err)
		}

		type fkEntry struct {
			from string
			to   string
		}
		fkGroups := make(map[int]*struct {
			target string
			cols   []fkEntry
		})

		for fkRows.Next() {
			var id, seq int
			var fkTable, fromCol, toCol string
			var onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &fkTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
				fkRows.Close()
				return nil, fmt.Errorf("sqlite pragma foreign_key_list scan: %w", err)
			}

			if fkGroups[id] == nil {
				fkGroups[id] = &struct {
					target string
					cols   []fkEntry
				}{target: fkTable}
			}
			fkGroups[id].cols = append(fkGroups[id].cols, fkEntry{from: fromCol, to: toCol})
		}
		fkRows.Close()

		for _, fkg := range fkGroups {
			var colPairs [][2]string
			for _, c := range fkg.cols {
				colPairs = append(colPairs, [2]string{c.from, c.to})
			}
			result.Edges = append(result.Edges, db.JoinEdge{
				Source:     tableName,
				Target:     fkg.target,
				Columns:    colPairs,
				Confidence: 1.0,
				SourceType: "declared_foreign_key",
				Composite:  len(fkg.cols) > 1,
			})
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// DatabaseType / Helpers
// ---------------------------------------------------------------------------

func (a *SQLiteAdapter) DatabaseType() string {
	return "sqlite"
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func BuildDSN(dbPath string) string {
	return dbPath
}
