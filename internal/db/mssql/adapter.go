package mssql

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/microsoft/go-mssqldb"
)

func init() {
	db.Register("mssql", func(dsn string) (db.Adapter, error) {
		return &MSSQLAdapter{dsn: dsn}, nil
	})
}

type MSSQLAdapter struct {
	dsn  string
	conn *sql.DB
}

func (a *MSSQLAdapter) Connect(ctx context.Context, dsn string) error {
	if a.conn != nil {
		if err := a.conn.PingContext(ctx); err == nil {
			return nil
		}
		a.conn.Close()
	}

	if dsn != "" {
		a.dsn = dsn
	}

	conn, err := sql.Open("sqlserver", a.dsn)
	if err != nil {
		return fmt.Errorf("mssql connect: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return fmt.Errorf("%w: mssql ping: %w", db.ErrConnectionFailed, err)
	}

	a.conn = conn
	return nil
}

func (a *MSSQLAdapter) Ping(ctx context.Context) error {
	if a.conn == nil {
		return fmt.Errorf("%w: not connected", db.ErrConnectionFailed)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.conn.PingContext(pingCtx); err != nil {
		return fmt.Errorf("%w: mssql ping: %w", db.ErrConnectionFailed, err)
	}
	return nil
}

func (a *MSSQLAdapter) Close(ctx context.Context) error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

func (a *MSSQLAdapter) DatabaseType() string {
	return "mssql"
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

type mssqlColumnRow struct {
	SchemaName      string
	TableName       string
	ColumnName      string
	ColumnID        int
	TypeName        string
	MaxLength       int
	Precision       int
	Scale           int
	IsNullable      bool
	IsIdentity      bool
	IsComputed      bool
	DefaultDef      string
	Comment         string
}

func (a *MSSQLAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. Tables
	tableQuery := `
		SELECT s.name AS schema_name, t.name AS table_name
		FROM sys.tables t
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE t.is_ms_shipped = 0
	`
	var tableArgs []any
	if len(tables) > 0 {
		placeholders := make([]string, len(tables))
		tableArgs = make([]any, len(tables))
		for i, t := range tables {
			placeholders[i] = fmt.Sprintf("@p%d", i)
			tableArgs[i] = t
		}
		tableQuery += " AND t.name IN (" + strings.Join(placeholders, ",") + ")"
	}

	tableRows, err := a.conn.QueryContext(queryCtx, tableQuery, tableArgs...)
	if err != nil {
		return nil, fmt.Errorf("mssql schema tables: %w", err)
	}
	defer tableRows.Close()

	type tableDef struct {
		schema string
		name   string
	}
	var userTables []tableDef
	for tableRows.Next() {
		var s, t string
		if err := tableRows.Scan(&s, &t); err != nil {
			return nil, fmt.Errorf("mssql schema table scan: %w", err)
		}
		userTables = append(userTables, tableDef{schema: s, name: t})
	}
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("mssql schema table rows: %w", err)
	}

	result := &db.SchemaResult{}

	for _, ut := range userTables {
		ti := db.TableInfo{
			Schema: ut.schema,
			Name:   ut.name,
			Type:   "BASE TABLE",
		}

		// 2. Columns
		colQuery := `
			SELECT s.name AS schema_name, t.name AS table_name,
			       c.name AS column_name, c.column_id,
			       ty.name AS type_name, c.max_length, c.precision, c.scale,
			       c.is_nullable, c.is_identity, c.is_computed,
			       ISNULL(OBJECT_DEFINITION(c.default_object_id), '') AS default_def,
			       ISNULL(ep.value, '') AS comment
			FROM sys.columns c
			JOIN sys.tables t ON c.object_id = t.object_id
			JOIN sys.schemas s ON t.schema_id = s.schema_id
			JOIN sys.types ty ON c.user_type_id = ty.user_type_id
			LEFT JOIN sys.extended_properties ep
			    ON ep.major_id = c.object_id AND ep.minor_id = c.column_id
			    AND ep.name = 'MS_Description'
			WHERE t.name = @p0 AND s.name = @p1
			ORDER BY c.column_id
		`

		colRows, err := a.conn.QueryContext(queryCtx, colQuery, ut.name, ut.schema)
		if err != nil {
			return nil, fmt.Errorf("mssql schema columns for %s: %w", ut.name, err)
		}

		for colRows.Next() {
			var r mssqlColumnRow
			if err := colRows.Scan(
				&r.SchemaName, &r.TableName,
				&r.ColumnName, &r.ColumnID,
				&r.TypeName, &r.MaxLength, &r.Precision, &r.Scale,
				&r.IsNullable, &r.IsIdentity, &r.IsComputed,
				&r.DefaultDef, &r.Comment,
			); err != nil {
				colRows.Close()
				return nil, fmt.Errorf("mssql schema column scan: %w", err)
			}

			col := db.ColumnInfo{
				Name:       r.ColumnName,
				Ordinal:    r.ColumnID,
				ColumnType: mssqlTypeToString(r.TypeName, r.MaxLength, r.Precision, r.Scale),
				IsNullable: r.IsNullable,
			}
			if r.DefaultDef != "" {
				col.Default = r.DefaultDef
			}
			if r.IsComputed {
				col.IsGenerated = true
				col.GeneratedExpression = r.DefaultDef
			}
			if r.Comment != "" {
				col.Comment = r.Comment
			}
			ti.Columns = append(ti.Columns, col)
		}
		colRows.Close()
		if err := colRows.Err(); err != nil {
			return nil, fmt.Errorf("mssql schema column rows: %w", err)
		}

		// 3. Primary keys
		pkQuery := `
			SELECT s.name AS schema_name, t.name AS table_name,
			       c.name AS column_name, i.name AS constraint_name
			FROM sys.indexes i
			JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
			JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
			JOIN sys.tables t ON i.object_id = t.object_id
			JOIN sys.schemas s ON t.schema_id = s.schema_id
			WHERE i.is_primary_key = 1 AND t.name = @p0 AND s.name = @p1
			ORDER BY ic.key_ordinal
		`
		pkRows, err := a.conn.QueryContext(queryCtx, pkQuery, ut.name, ut.schema)
		if err == nil {
			var pkCols []string
			var pkName string
			for pkRows.Next() {
				var sName, tName, cName, constraintName string
				if err := pkRows.Scan(&sName, &tName, &cName, &constraintName); err == nil {
					pkCols = append(pkCols, cName)
					pkName = constraintName
				}
			}
			pkRows.Close()
			if len(pkCols) > 0 {
				ti.Constraints = append(ti.Constraints, db.ConstraintInfo{
					Name:    pkName,
					Type:    "PRIMARY_KEY",
					Columns: pkCols,
				})
				for i := range ti.Columns {
					for _, pk := range pkCols {
						if ti.Columns[i].Name == pk {
							ti.Columns[i].IsPrimaryKey = true
						}
					}
				}
			}
		}

		// 4. Indexes
		idxQuery := `
			SELECT s.name AS schema_name, t.name AS table_name,
			       i.name AS index_name, i.type_desc, i.is_unique, i.is_primary_key,
			       i.has_filter, ISNULL(i.filter_definition, '') AS filter_definition,
			       c.name AS column_name, ic.key_ordinal,
			       ic.is_descending_key, ic.is_included_column
			FROM sys.indexes i
			JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
			JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
			JOIN sys.tables t ON i.object_id = t.object_id
			JOIN sys.schemas s ON t.schema_id = s.schema_id
			WHERE i.type > 0 AND t.name = @p0 AND s.name = @p1
			ORDER BY i.name, ic.key_ordinal
		`
		idxRows, err := a.conn.QueryContext(queryCtx, idxQuery, ut.name, ut.schema)
		if err == nil {
			idxMap := make(map[string]*db.IndexInfo)
			for idxRows.Next() {
				var sName, tName, idxName, typeDesc, filterDef, colName string
				var isUnique, isPK, hasFilter, isDesc, isIncluded bool
				var keyOrdinal int
				if err := idxRows.Scan(&sName, &tName, &idxName, &typeDesc, &isUnique, &isPK,
					&hasFilter, &filterDef, &colName, &keyOrdinal, &isDesc, &isIncluded); err != nil {
					continue
				}

				ii, exists := idxMap[idxName]
				if !exists {
					order := "ASC"
					if isDesc {
						order = "DESC"
					}
					ii = &db.IndexInfo{
						Name:     idxName,
						Type:     mssqlIndexType(typeDesc),
						IsUnique: isUnique,
						Primary:  isPK,
						Visible:  true,
						IsPartial: hasFilter,
					}
					if !isIncluded {
						ii.Columns = append(ii.Columns, db.IndexColumn{
							Name:     colName,
							Order:    order,
							Sequence: keyOrdinal,
						})
					}
					idxMap[idxName] = ii
				} else if !isIncluded {
					order := "ASC"
					if isDesc {
						order = "DESC"
					}
					ii.Columns = append(ii.Columns, db.IndexColumn{
						Name:     colName,
						Order:    order,
						Sequence: keyOrdinal,
					})
				}
			}
			idxRows.Close()
			for _, ii := range idxMap {
				ti.Indexes = append(ti.Indexes, *ii)
			}
		}

		// 5. Foreign keys
		fkQuery := `
			SELECT fk.name AS fk_name,
			       OBJECT_NAME(fk.parent_object_id) AS source_table,
			       OBJECT_NAME(fk.referenced_object_id) AS target_table,
			       sc.name AS source_column,
			       tc.name AS target_column
			FROM sys.foreign_keys fk
			JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
			JOIN sys.columns sc ON fkc.parent_object_id = sc.object_id AND fkc.parent_column_id = sc.column_id
			JOIN sys.columns tc ON fkc.referenced_object_id = tc.object_id AND fkc.referenced_column_id = tc.column_id
			JOIN sys.tables st ON fk.parent_object_id = st.object_id
			JOIN sys.schemas ss ON st.schema_id = ss.schema_id
			WHERE st.name = @p0 AND ss.name = @p1
			ORDER BY fk.name, fkc.constraint_column_id
		`
		fkRows, err := a.conn.QueryContext(queryCtx, fkQuery, ut.name, ut.schema)
		if err == nil {
			type fkEntry struct {
				from string
				to   string
			}
			fkGroups := make(map[string]*struct {
				target string
				cols   []fkEntry
			})

			for fkRows.Next() {
				var fkName, srcTable, tgtTable, srcCol, tgtCol string
				if err := fkRows.Scan(&fkName, &srcTable, &tgtTable, &srcCol, &tgtCol); err != nil {
					continue
				}
				if fkGroups[fkName] == nil {
					fkGroups[fkName] = &struct {
						target string
						cols   []fkEntry
					}{target: tgtTable}
				}
				fkGroups[fkName].cols = append(fkGroups[fkName].cols, fkEntry{from: srcCol, to: tgtCol})
			}
			fkRows.Close()

			for fkName, fkg := range fkGroups {
				var cols, refCols []string
				for _, c := range fkg.cols {
					cols = append(cols, c.from)
					refCols = append(refCols, c.to)
				}
				ti.Constraints = append(ti.Constraints, db.ConstraintInfo{
					Name:              fkName,
					Type:              "FOREIGN_KEY",
					Columns:           cols,
					ReferencedTable:   fkg.target,
					ReferencedColumns: refCols,
				})
			}
		}

		result.Tables = append(result.Tables, ti)
	}

	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("mssql schema table rows: %w", err)
	}

	// 6. Views
	viewQuery := `
		SELECT s.name AS schema_name, v.name AS view_name
		FROM sys.views v
		JOIN sys.schemas s ON v.schema_id = s.schema_id
		WHERE v.is_ms_shipped = 0
	`
	viewRows, err := a.conn.QueryContext(queryCtx, viewQuery)
	if err == nil {
		defer viewRows.Close()
		for viewRows.Next() {
			var sName, vName string
			if err := viewRows.Scan(&sName, &vName); err == nil {
				result.Views = append(result.Views, db.ViewInfo{
					Schema: sName,
					Name:   vName,
				})
			}
		}
	}

	return result, nil
}

// mssqlTypeToString maps MSSQL system type names to common type strings.
func mssqlTypeToString(typeName string, maxLength, precision, scale int) string {
	upper := strings.ToUpper(typeName)
	switch upper {
	case "INT", "INTEGER":
		return "int"
	case "BIGINT":
		return "bigint"
	case "SMALLINT":
		return "smallint"
	case "TINYINT":
		return "tinyint"
	case "BIT":
		return "bit"
	case "DECIMAL", "NUMERIC":
		if scale > 0 {
			return fmt.Sprintf("decimal(%d,%d)", precision, scale)
		}
		return fmt.Sprintf("decimal(%d)", precision)
	case "MONEY":
		return "money"
	case "SMALLMONEY":
		return "smallmoney"
	case "FLOAT":
		return "float"
	case "REAL":
		return "real"
	case "DATE":
		return "date"
	case "DATETIME":
		return "datetime"
	case "DATETIME2":
		return fmt.Sprintf("datetime2(%d)", scale)
	case "SMALLDATETIME":
		return "smalldatetime"
	case "TIME":
		return fmt.Sprintf("time(%d)", scale)
	case "DATETIMEOFFSET":
		return "datetimeoffset"
	case "CHAR":
		return fmt.Sprintf("char(%d)", maxLength)
	case "VARCHAR":
		if maxLength == -1 || maxLength > 8000 {
			return "varchar(max)"
		}
		return fmt.Sprintf("varchar(%d)", maxLength)
	case "NCHAR":
		return fmt.Sprintf("nchar(%d)", maxLength/2)
	case "NVARCHAR":
		length := maxLength / 2
		if length == -1 || length > 4000 {
			return "nvarchar(max)"
		}
		return fmt.Sprintf("nvarchar(%d)", length)
	case "NTEXT":
		return "ntext"
	case "TEXT":
		return "text"
	case "NVARCHAR(MAX)":
		return "nvarchar(max)"
	case "VARCHAR(MAX)":
		return "varchar(max)"
	case "BINARY":
		return fmt.Sprintf("binary(%d)", maxLength)
	case "VARBINARY":
		if maxLength == -1 {
			return "varbinary(max)"
		}
		return fmt.Sprintf("varbinary(%d)", maxLength)
	case "IMAGE":
		return "image"
	case "UNIQUEIDENTIFIER":
		return "uniqueidentifier"
	case "XML":
		return "xml"
	case "HIERARCHYID":
		return "hierarchyid"
	case "GEOMETRY":
		return "geometry"
	case "GEOGRAPHY":
		return "geography"
	case "SQL_VARIANT":
		return "sql_variant"
	default:
		return strings.ToLower(typeName)
	}
}

// mssqlIndexType maps MSSQL index type descriptions to common type strings.
func mssqlIndexType(typeDesc string) string {
	upper := strings.ToUpper(typeDesc)
	switch {
	case strings.Contains(upper, "CLUSTERED"):
		return "CLUSTERED"
	case strings.Contains(upper, "NONCLUSTERED"):
		return "NONCLUSTERED"
	case strings.Contains(upper, "XML"):
		return "XML"
	case strings.Contains(upper, "SPATIAL"):
		return "SPATIAL"
	default:
		return upper
	}
}

// ---------------------------------------------------------------------------
// Explain (SHOWPLAN_XML)
// ---------------------------------------------------------------------------

// showPlanXML is the root element of MSSQL SHOWPLAN_XML output.
type showPlanXML struct {
	XMLName       xmlName       `xml:"ShowPlanXML"`
	BatchSequence batchSequence `xml:"BatchSequence"`
}

type xmlName struct {
	Local string
}

type batchSequence struct {
	Batch batch `xml:"Batch"`
}

type batch struct {
	Statements statements `xml:"Statements"`
}

type statements struct {
	StmtSimples []stmtSimple `xml:"StmtSimple"`
}

type stmtSimple struct {
	StatementText       string  `xml:"StatementText,attr"`
	StatementSubTreeCost float64 `xml:"StatementSubTreeCost,attr"`
	StatementEstRows    float64 `xml:"StatementEstRows,attr"`
	StatementType       string  `xml:"StatementType,attr"`
	QueryPlan           *queryPlan `xml:"QueryPlan"`
}

type queryPlan struct {
	RelOps []relOp `xml:"RelOp"`
}

type relOp struct {
	NodeID        int     `xml:"NodeId,attr"`
	PhysicalOp    string  `xml:"PhysicalOp,attr"`
	LogicalOp     string  `xml:"LogicalOp,attr"`
	EstimatedRows float64 `xml:"EstimateRows,attr"`
	EstimatedIO   float64 `xml:"EstimatedIO,attr"`
	EstimatedCPU  float64 `xml:"EstimatedCPU,attr"`
	AvgRowSize    float64 `xml:"AvgRowSize,attr"`
	Parallel      bool    `xml:"Parallel,attr"`
	EstimateRebinds float64 `xml:"EstimateRebinds,attr"`
	EstimateRewinds float64 `xml:"EstimateRewinds,attr"`
	EstimatedExecutionMode string `xml:"EstimatedExecutionMode,attr"`

	OutputList []string `xml:"OutputList>ColumnReference"`
	RunTimeInformation *runTimeInformation `xml:"RunTimeInformation"`
	RelOps     []relOp  `xml:"RelOp"`

	// Table scan/index details
	TableScan     *tableScan     `xml:"TableScan"`
	IndexScan     *indexScan     `xml:"IndexScan"`
	IndexSeek     *indexSeek     `xml:"IndexSeek"`
	Sort          *sortOp        `xml:"Sort"`
	Spool         *spoolOp       `xml:"Spool"`
	StreamAggregate *relOp       `xml:"StreamAggregate"`
	Hash          *hashOp        `xml:"Hash"`
	NestedLoops   *nestedLoopsOp `xml:"NestedLoops"`
	MergeJoin     *mergeJoinOp   `xml:"MergeJoin"`
	ComputeScalar *relOp         `xml:"ComputeScalar"`
	Filter        *relOp         `xml:"Filter"`
	ConstantScan  *relOp         `xml:"ConstantScan"`
	Bitmap        *relOp         `xml:"Bitmap"`
	Parallelism   *relOp         `xml:"Parallelism"`
	Sequence      *relOp         `xml:"Sequence"`
}

type runTimeInformation struct {
	RunTimeCounters []runTimeCounter `xml:"RunTimeCountersPerThread"`
}

type runTimeCounter struct {
	ActualRows    int64   `xml:"ActualRows,attr"`
	ActualEndTime string  `xml:"ActualEndTime,attr"`
	ActualRebinds float64 `xml:"ActualRebinds,attr"`
	ActualRewinds float64 `xml:"ActualRewinds,attr"`
}

type tableScan struct {
	Object     object     `xml:"Object"`
	Ordered    bool       `xml:"Ordered,attr"`
	ForcedScan bool       `xml:"ForcedScan,attr"`
	Predicate  *predicate `xml:"Predicate"`
}

type indexScan struct {
	Object        object        `xml:"Object"`
	Ordered       bool          `xml:"Ordered,attr"`
	ScanDirection string        `xml:"ScanDirection,attr"`
	ForcedScan    bool          `xml:"ForcedScan,attr"`
	Predicate     *predicate    `xml:"Predicate"`
}

type indexSeek struct {
	Object        object        `xml:"Object"`
	Ordered       bool          `xml:"Ordered,attr"`
	ScanDirection string        `xml:"ScanDirection,attr"`
	SeekPredicates *seekPredicates `xml:"SeekPredicates"`
}

type sortOp struct {
	DistinctOrder bool  `xml:"DistinctOrder,attr"`
}

type spoolOp struct {
	PrimaryNodeID int  `xml:"PrimaryNodeId,attr"`
	Stacking      bool `xml:"Stacking,attr"`
}

type hashOp struct {
	BuildResidual *predicate `xml:"BuildResidual"`
}

type nestedLoopsOp struct {
	Optimized bool `xml:"Optimized,attr"`
	WithOrder bool `xml:"WithOrder,attr"`
}

type mergeJoinOp struct {
	ManyToMany   bool `xml:"ManyToMany,attr"`
	Unique       bool `xml:"Unique,attr"`
}

type object struct {
	Database string `xml:"Database,attr"`
	Schema   string `xml:"Schema,attr"`
	Table    string `xml:"Table,attr"`
	Index    string `xml:"Index,attr"`
	Alias    string `xml:"Alias,attr"`
	IndexKind string `xml:"IndexKind,attr"`
}

type seekPredicates struct {
	SeekPredicatesNew []seekPredicateNew `xml:"SeekPredicateNew"`
}

type seekPredicateNew struct {
	SeekKeys []seekRange `xml:"SeekKeys>Prefix>RangeExpression|SeekKeys>Prefix>RangeCondition"`
}

type seekRange struct {
	Start string `xml:"Start,attr"`
	End   string `xml:"End,attr"`
}

type predicate struct {
	ScalarOperator *scalarOperator `xml:"ScalarOperator"`
}

type scalarOperator struct {
	ScalarString string `xml:"ScalarString"`
}

func (a *MSSQLAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get a dedicated connection to avoid session state leakage
	// (SHOWPLAN_XML is session-level per Pitfall 3)
	conn, err := a.getConn(queryCtx)
	if err != nil {
		return nil, err
	}

	// Enable SHOWPLAN_XML
	if _, err := conn.ExecContext(queryCtx, "SET SHOWPLAN_XML ON"); err != nil {
		return nil, fmt.Errorf("%w: mssql set showplan on: %w", db.ErrExplainFailed, err)
	}

	// CRITICAL: Always reset SHOWPLAN_XML in defer (per RESEARCH.md Pitfall 3)
	plan := &db.ExplainPlan{}
	var xmlResult string
	var executeErr error

	defer func() {
		// Reset SHOWPLAN_XML — this is the critical session state cleanup
		conn.ExecContext(context.Background(), "SET SHOWPLAN_XML OFF")
	}()

	// Execute query — MSSQL returns XML plan instead of executing
	err = conn.QueryRowContext(queryCtx, query).Scan(&xmlResult)
	if err != nil {
		executeErr = fmt.Errorf("%w: mssql explain: %w", db.ErrExplainFailed, err)
	}

	if executeErr != nil {
		return nil, executeErr
	}

	// Parse XML plan
	var sp showPlanXML
	if err := xml.Unmarshal([]byte(xmlResult), &sp); err != nil {
		return nil, fmt.Errorf("%w: mssql explain xml parse: %w", db.ErrExplainFailed, err)
	}

	// Extract plan data
	plan.DialectRaw = xmlResult

	// Walk through statements
	for _, stmt := range sp.BatchSequence.Batch.Statements.StmtSimples {
		// Cost
		cost := stmt.StatementSubTreeCost
		plan.EstimatedTotalCost = &cost

		// Row estimate
		rows := int64(stmt.StatementEstRows)
		plan.EstimatedRowsExamined = &rows

		// Walk RelOps
		if stmt.QueryPlan != nil {
			for _, op := range stmt.QueryPlan.RelOps {
				walkRelOp(&op, plan)
			}
		}
	}

	if plan.EstimatedTotalCost == nil {
		plan.EstimatedTotalCost = float64Ptr(0)
	}
	if plan.EstimatedRowsExamined == nil {
		plan.EstimatedRowsExamined = int64Ptr(0)
	}

	return plan, nil
}

func walkRelOp(op *relOp, plan *db.ExplainPlan) {
	// Detect table scans
	if op.TableScan != nil {
		table := op.TableScan.Object.Table
		if table != "" {
			plan.FullScanTables = append(plan.FullScanTables, table)
		}
	}

	// Detect index scans/seeks
	if op.IndexScan != nil {
		table := op.IndexScan.Object.Table
		index := op.IndexScan.Object.Index
		if table != "" && index != "" {
			plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
				Table:      table,
				Index:      index,
				AccessType: "index_scan",
			})
		}
	}

	if op.IndexSeek != nil {
		table := op.IndexSeek.Object.Table
		index := op.IndexSeek.Object.Index
		if table != "" && index != "" {
			plan.IndexUsage = append(plan.IndexUsage, db.IndexUsageEntry{
				Table:      table,
				Index:      index,
				AccessType: "index_seek",
			})
		}
	}

	// Detect sort operations
	if op.Sort != nil {
		plan.SortOperations++
	}

	// Detect temp operations (spool)
	if op.Spool != nil {
		plan.TempOperations++
	}

	// Detect join operations
	if op.NestedLoops != nil {
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Nested Loops",
		})
	}
	if op.Hash != nil {
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Hash Match",
		})
	}
	if op.MergeJoin != nil {
		plan.JoinOperations = append(plan.JoinOperations, db.JoinOperationEntry{
			Type: "Merge Join",
		})
	}

	// Recurse into child operations
	for i := range op.RelOps {
		walkRelOp(&op.RelOps[i], plan)
	}
}

// getConn returns a connection from the pool or creates one.
func (a *MSSQLAdapter) getConn(ctx context.Context) (*sql.Conn, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}
	conn, err := a.conn.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("mssql get conn: %w", err)
	}
	return conn, nil
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

func (a *MSSQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
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

	// Layer 2: Use SET SHOWPLAN_XML ON to validate without execution
	conn, err := a.getConn(queryCtx)
	if err != nil {
		return &db.ValidateResult{
			Valid:    true,
			ReadOnly: true,
		}, nil
	}

	// Enable SHOWPLAN_XML
	if _, err := conn.ExecContext(queryCtx, "SET SHOWPLAN_XML ON"); err != nil {
		return &db.ValidateResult{
			Valid:    true,
			ReadOnly: true,
		}, nil
	}
	defer conn.ExecContext(context.Background(), "SET SHOWPLAN_XML OFF")

	var xmlResult string
	err = conn.QueryRowContext(queryCtx, query).Scan(&xmlResult)
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

func (a *MSSQLAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
		    s.name AS schema_name,
		    t.name AS table_name,
		    COALESCE(p.rows, 0) AS row_count,
		    COALESCE(SUM(a.used_pages), 0) AS used_pages
		FROM sys.tables t
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		LEFT JOIN sys.partitions p ON t.object_id = p.object_id AND p.index_id IN (0, 1)
		LEFT JOIN sys.allocation_units a ON p.hobt_id = a.container_id
		WHERE t.is_ms_shipped = 0
	`
	args := []any{}
	if len(tables) > 0 {
		placeholders := make([]string, len(tables))
		for i, t := range tables {
			placeholders[i] = fmt.Sprintf("@p%d", i)
			args = append(args, t)
		}
		query += " AND t.name IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " GROUP BY s.name, t.name, p.rows ORDER BY s.name, t.name"

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql stats: %w", err)
	}
	defer rows.Close()

	result := &db.StatsResult{}
	for rows.Next() {
		var schemaName, tableName string
		var rowCount, usedPages int64

		if err := rows.Scan(&schemaName, &tableName, &rowCount, &usedPages); err != nil {
			return nil, fmt.Errorf("mssql stats scan: %w", err)
		}

		dataSize := usedPages * 8192

		result.Tables = append(result.Tables, db.TableStats{
			Name:                tableName,
			RowCount:            rowCount,
			CardinalityEstimate: rowCount,
			DataSizeBytes:       dataSize,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mssql stats rows: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Indexes
// ---------------------------------------------------------------------------

func (a *MSSQLAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
		    t.name AS table_name,
		    i.name AS index_name,
		    i.type_desc,
		    i.is_unique,
		    i.is_primary_key,
		    i.has_filter,
		    c.name AS column_name,
		    ic.key_ordinal,
		    ic.is_descending_key,
		    ic.is_included_column
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t ON i.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE i.type > 0 AND t.is_ms_shipped = 0
	`
	args := []any{}
	if len(tables) > 0 {
		placeholders := make([]string, len(tables))
		for i, t := range tables {
			placeholders[i] = fmt.Sprintf("@p%d", i)
			args = append(args, t)
		}
		query += " AND t.name IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY t.name, i.name, ic.key_ordinal"

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql indexes: %w", err)
	}
	defer rows.Close()

	tableIdxMap := make(map[string]map[string]*db.IndexInfo)
	var tableOrder []string

	for rows.Next() {
		var tableName, idxName, typeDesc, colName string
		var isUnique, isPK, hasFilter, isDesc, isIncluded bool
		var keyOrdinal int

		if err := rows.Scan(&tableName, &idxName, &typeDesc, &isUnique, &isPK,
			&hasFilter, &colName, &keyOrdinal, &isDesc, &isIncluded); err != nil {
			return nil, fmt.Errorf("mssql indexes scan: %w", err)
		}

		idxMap, ok := tableIdxMap[tableName]
		if !ok {
			idxMap = make(map[string]*db.IndexInfo)
			tableIdxMap[tableName] = idxMap
			tableOrder = append(tableOrder, tableName)
		}

		ii, exists := idxMap[idxName]
		if !exists {
			order := "ASC"
			if isDesc {
				order = "DESC"
			}
			ii = &db.IndexInfo{
				Name:     idxName,
				Type:     mssqlIndexType(typeDesc),
				IsUnique: isUnique,
				Primary:  isPK,
				Visible:  true,
				IsPartial: hasFilter,
			}
			if !isIncluded {
				ii.Columns = append(ii.Columns, db.IndexColumn{
					Name:     colName,
					Order:    order,
					Sequence: keyOrdinal,
				})
			}
			idxMap[idxName] = ii
		} else if !isIncluded {
			order := "ASC"
			if isDesc {
				order = "DESC"
			}
			ii.Columns = append(ii.Columns, db.IndexColumn{
				Name:     colName,
				Order:    order,
				Sequence: keyOrdinal,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mssql indexes rows: %w", err)
	}

	result := &db.IndexesResult{}
	for _, name := range tableOrder {
		idxMap := tableIdxMap[name]
		tableIdx := db.TableIndexInfo{Table: name}
		for _, ii := range idxMap {
			tableIdx.Indexes = append(tableIdx.Indexes, *ii)
		}
		result.Tables = append(result.Tables, tableIdx)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Joins
// ---------------------------------------------------------------------------

func (a *MSSQLAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("%w: mssql adapter not connected", db.ErrConnectionFailed)
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
		    OBJECT_NAME(fk.parent_object_id) AS source_table,
		    OBJECT_NAME(fk.referenced_object_id) AS target_table,
		    sc.name AS source_column,
		    tc.name AS target_column,
		    fk.name AS fk_name
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		JOIN sys.columns sc ON fkc.parent_object_id = sc.object_id AND fkc.parent_column_id = sc.column_id
		JOIN sys.columns tc ON fkc.referenced_object_id = tc.object_id AND fkc.referenced_column_id = tc.column_id
		JOIN sys.tables st ON fk.parent_object_id = st.object_id
		JOIN sys.schemas ss ON st.schema_id = ss.schema_id
		WHERE fk.parent_object_id = fkc.constraint_object_id -- avoid cartesian
	`
	args := []any{}
	if len(tables) > 0 {
		placeholders := make([]string, len(tables))
		for i, t := range tables {
			placeholders[i] = fmt.Sprintf("@p%d", i)
			args = append(args, t)
		}
		query += " AND (st.name IN (" + strings.Join(placeholders, ",") + ")" +
			" OR OBJECT_NAME(fk.referenced_object_id) IN (" + strings.Join(placeholders, ",") + "))"
		// Add second set of args for the referenced table
		for _, t := range tables {
			args = append(args, t)
		}
	}
	query += " ORDER BY fk.name, fkc.constraint_column_id"

	rows, err := a.conn.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql joins: %w", err)
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
		var sourceTable, targetTable, sourceCol, targetCol, fkName string
		if err := rows.Scan(&sourceTable, &targetTable, &sourceCol, &targetCol, &fkName); err != nil {
			return nil, fmt.Errorf("mssql joins scan: %w", err)
		}

		if edge, ok := edgeMap[fkName]; ok {
			edge.Columns = append(edge.Columns, [2]string{sourceCol, targetCol})
		} else {
			edgeMap[fkName] = &fkEdge{
				SourceTable: sourceTable,
				TargetTable: targetTable,
				Columns:     [][2]string{{sourceCol, targetCol}},
				Constraint:  fkName,
			}
			edgeOrder = append(edgeOrder, fkName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mssql joins rows: %w", err)
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

func BuildDSN(host string, port int, database, username, password string, encrypt string) string {
	u := &url.URL{
		Scheme: "sqlserver",
		Host:   fmt.Sprintf("%s:%d", host, port),
	}
	if username != "" {
		if password != "" {
			u.User = url.UserPassword(username, password)
		} else {
			u.User = url.User(username)
		}
	}
	q := u.Query()
	if encrypt != "" {
		q.Set("encrypt", encrypt)
	} else {
		q.Set("encrypt", "true") // Default to encrypt=true per security best practices
	}
	if database != "" {
		q.Set("database", database)
	}
	u.RawQuery = q.Encode()
	return u.String()
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
