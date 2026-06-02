package db

// SchemaResult is the full schema extraction result from an adapter.
type SchemaResult struct {
	Tables    []TableInfo    `json:"tables"`
	Views     []ViewInfo     `json:"views,omitempty"`
	Triggers  []TriggerInfo  `json:"triggers,omitempty"`
	Functions []FunctionInfo `json:"functions,omitempty"`
	Enums     []EnumInfo     `json:"enums,omitempty"`
}

// TableInfo describes a single database table.
type TableInfo struct {
	Schema         string           `json:"schema"`
	Name           string           `json:"name"`
	Type           string           `json:"type"`
	Columns        []ColumnInfo     `json:"columns"`
	Constraints    []ConstraintInfo `json:"constraints"`
	Indexes        []IndexInfo      `json:"indexes"`
	Comment        string           `json:"comment"`
	RowCountEstimate int64          `json:"row_count_estimate"`
}

// ColumnInfo describes a single column in a table.
type ColumnInfo struct {
	Name               string `json:"name"`
	Ordinal            int    `json:"ordinal_position"`
	ColumnType         string `json:"type"`
	IsNullable         bool   `json:"nullable"`
	Default            string `json:"default,omitempty"`
	IsPrimaryKey       bool   `json:"primary_key"`
	IsGenerated        bool   `json:"generated"`
	GeneratedExpression string `json:"generated_expression,omitempty"`
	ExtraDef           string `json:"extra_def,omitempty"`
	Comment            string `json:"comment,omitempty"`
}

// ConstraintInfo describes a constraint on a table.
type ConstraintInfo struct {
	Name              string   `json:"name"`
	Type              string   `json:"type"` // PRIMARY_KEY, FOREIGN_KEY, UNIQUE, CHECK
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referenced_schema,omitempty"`
	ReferencedTable   string   `json:"referenced_table,omitempty"`
	ReferencedColumns []string `json:"referenced_columns,omitempty"`
}

// IndexInfo describes a single index on a table.
type IndexInfo struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`       // BTREE, HASH, GIN, etc.
	IsUnique   bool           `json:"unique"`
	Primary    bool           `json:"primary"`
	Visible    bool           `json:"visible"`
	Columns    []IndexColumn  `json:"columns"`
	Expression string         `json:"expression,omitempty"`
	Comment    string         `json:"comment,omitempty"`
	IsPartial  bool           `json:"is_partial,omitempty"`
	IsFunctional bool         `json:"is_functional,omitempty"`
	DialectSpecific map[string]any `json:"dialect_specific,omitempty"`
}

// IndexColumn describes a column in an index.
type IndexColumn struct {
	Name        string `json:"name"`
	Order       string `json:"order"` // ASC, DESC
	Sequence    int    `json:"sequence"`
	Cardinality int64  `json:"cardinality"`
	PrefixLength *int  `json:"prefix_length,omitempty"`
}

// FKInfo describes a foreign key relationship.
type FKInfo struct {
	ConstraintName   string   `json:"constraint_name"`
	Columns          []string `json:"columns"`
	ReferencedSchema string   `json:"referenced_schema,omitempty"`
	ReferencedTable  string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
}

// JoinEdge describes a join relationship between two tables.
type JoinEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Columns     [][2]string `json:"columns"`
	Confidence  float64 `json:"confidence"`
	SourceType  string  `json:"source_type"` // declared_foreign_key, inferred_naming_match, cross_domain
	Composite   bool    `json:"composite"`
	CrossDomain bool    `json:"cross_domain"`
}

// ExplainPlan holds the normalized execution plan from an EXPLAIN command.
type ExplainPlan struct {
	EstimatedTotalCost   *float64              `json:"estimated_total_cost"`
	ActualTotalTimeMs    *float64              `json:"actual_total_time_ms,omitempty"`
	EstimatedRowsExamined *int64               `json:"estimated_rows_examined"`
	ActualRowsExamined   *int64                `json:"actual_rows_examined,omitempty"`
	FullScanTables       []string              `json:"full_scan_tables"`
	IndexUsage           []IndexUsageEntry     `json:"index_usage"`
	SortOperations       int                   `json:"sort_operations"`
	TempOperations       int                   `json:"temp_operations"`
	JoinOperations       []JoinOperationEntry  `json:"join_operations"`
	Warnings             []string              `json:"warnings"`
	DialectRaw           any                   `json:"dialect_raw,omitempty"`
}

// IndexUsageEntry describes how an index is used in a query plan.
type IndexUsageEntry struct {
	Table      string `json:"table"`
	Index      string `json:"index"`
	Covering   bool   `json:"covering"`
	AccessType string `json:"access_type"`
}

// JoinOperationEntry describes a join operation in a query plan.
type JoinOperationEntry struct {
	Type   string   `json:"type"`
	Tables []string `json:"tables"`
}

// ValidateResult holds the result of SQL validation.
type ValidateResult struct {
	Valid          bool     `json:"valid"`
	NormalizedSQL  string   `json:"normalized_sql,omitempty"`
	StatementType  string   `json:"statement_type,omitempty"`
	ReadOnly       bool     `json:"read_only"`
	Tables         []string `json:"tables,omitempty"`
	Columns        []string `json:"columns,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

// StatsResult holds table statistics from an adapter.
type StatsResult struct {
	Tables []TableStats `json:"tables"`
}

// TableStats describes statistics for a single table.
type TableStats struct {
	Name               string              `json:"table"`
	RowCount           int64               `json:"row_count"`
	CardinalityEstimate int64              `json:"cardinality"`
	DataSizeBytes      int64               `json:"data_length_bytes"`
	IndexSizeBytes     int64               `json:"index_length_bytes"`
	Freshness          string              `json:"freshness"`
	UpdatedAt          string              `json:"last_analyzed_at"`
}

// IndexesResult holds index information from an adapter.
type IndexesResult struct {
	Tables []TableIndexInfo `json:"tables"`
}

// TableIndexInfo describes indexes on a single table.
type TableIndexInfo struct {
	Table   string      `json:"table"`
	Indexes []IndexInfo `json:"indexes"`
}

// JoinsResult holds join information from an adapter.
type JoinsResult struct {
	Edges []JoinEdge `json:"edges"`
}

// ViewInfo describes a database view.
type ViewInfo struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Def    string `json:"definition"`
	Comment string `json:"comment,omitempty"`
}

// TriggerInfo describes a database trigger.
type TriggerInfo struct {
	Name    string `json:"name"`
	Event   string `json:"event"`
	Timing  string `json:"timing"`
	Def     string `json:"definition"`
	Comment string `json:"comment,omitempty"`
}

// FunctionInfo describes a database function or procedure.
type FunctionInfo struct {
	Name       string `json:"name"`
	ReturnType string `json:"return_type"`
	Arguments  string `json:"arguments"`
	Type       string `json:"type"` // FUNCTION, PROCEDURE
}

// EnumInfo describes a database enum type.
type EnumInfo struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}
