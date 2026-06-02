package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
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
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	return nil, db.ErrNotImplemented
}

func (a *MySQLAdapter) DatabaseType() string {
	return "mysql"
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
