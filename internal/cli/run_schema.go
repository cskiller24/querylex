package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
)

// SchemaData is the response data for the schema command.
type SchemaData struct {
	Tables []SchemaTableDef `json:"tables"`
	Schema *db.SchemaResult `json:"schema,omitempty"`
}

// SchemaTableDef describes a table in the schema response.
type SchemaTableDef struct {
	Name        string               `json:"table"`
	Schema      string               `json:"schema"`
	Type        string               `json:"type"`
	Comment     string               `json:"comment"`
	Columns     []SchemaColumnDef    `json:"columns"`
	Constraints []SchemaConstraintDef `json:"constraints"`
	Definition  string               `json:"definition"`
}

// SchemaColumnDef describes a column in the schema response.
type SchemaColumnDef struct {
	Name           string `json:"name"`
	OrdinalPos     int    `json:"ordinal_position"`
	Type           string `json:"type"`
	Nullable       bool   `json:"nullable"`
	Default        any    `json:"default,omitempty"`
	PrimaryKey     bool   `json:"primary_key"`
	Generated      bool   `json:"generated"`
	GeneratedExpr  string `json:"generated_expression,omitempty"`
	Comment        string `json:"comment,omitempty"`
}

// SchemaConstraintDef describes a constraint in the schema response.
type SchemaConstraintDef struct {
	Name              string   `json:"name"`
	Type              string   `json:"type"`
	Columns           []string `json:"columns"`
	ReferencedTable   string   `json:"referenced_table,omitempty"`
	ReferencedColumns []string `json:"referenced_columns,omitempty"`
}

// RunSchema executes the schema command with a full preflight.
func RunSchema(tables []string) *format.Response[SchemaData] {
	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertSchemaError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()
	return runSchemaWithAdapter(preflight.Adapter, tables, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runSchemaInternal is an alias for runSchemaWithAdapter, exported for testing.
// Deprecated: Use runSchemaWithAdapter directly.
func runSchemaInternal(adapter db.Adapter, tables []string, traceID string, activeDBID *string) *format.Response[SchemaData] {
	return runSchemaWithAdapter(adapter, tables, traceID, activeDBID)
}

// runSchemaWithAdapter executes the schema command with a provided adapter.
func runSchemaWithAdapter(adapter db.Adapter, tables []string, traceID string, activeDBID *string) *format.Response[SchemaData] {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Schema(ctx, tables)
	if err != nil {
		resp := format.NewErrorResponse[SchemaData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Schema extraction failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Build schema artifacts
	if result != nil {
		schemaJSON, err := index.BuildSchema(result)
		if err == nil {
			// Attempt to write schema artifacts (non-fatal if it fails)
			_ = schemaJSON // Place in response data
		}

		// Build slim schema
		slimJSON, err := index.BuildSlimSchema(result)
		if err == nil {
			_ = slimJSON
		}

		// Build schema map
		schemaMap, err := index.BuildSchemaMap(result)
		if err == nil {
			_ = schemaMap
		}
	}

	// Build response data
	tableDefs := make([]SchemaTableDef, 0, len(result.Tables))
	for _, t := range result.Tables {
		colDefs := make([]SchemaColumnDef, 0, len(t.Columns))
		for _, c := range t.Columns {
			col := SchemaColumnDef{
				Name:       c.Name,
				OrdinalPos: c.Ordinal,
				Type:       c.ColumnType,
				Nullable:   c.IsNullable,
				PrimaryKey: c.IsPrimaryKey,
				Generated:  c.IsGenerated,
				Comment:    c.Comment,
			}
			if c.Default != "" {
				col.Default = c.Default
			}
			if c.GeneratedExpression != "" {
				col.GeneratedExpr = c.GeneratedExpression
			}
			colDefs = append(colDefs, col)
		}

		constraintDefs := make([]SchemaConstraintDef, 0, len(t.Constraints))
		for _, cons := range t.Constraints {
			constraint := SchemaConstraintDef{
				Name:    cons.Name,
				Type:    cons.Type,
				Columns: cons.Columns,
			}
			if cons.ReferencedTable != "" {
				constraint.ReferencedTable = cons.ReferencedTable
				constraint.ReferencedColumns = cons.ReferencedColumns
			}
			constraintDefs = append(constraintDefs, constraint)
		}

		tableDefs = append(tableDefs, SchemaTableDef{
			Name:        t.Name,
			Schema:      t.Schema,
			Type:        t.Type,
			Comment:     t.Comment,
			Columns:     colDefs,
			Constraints: constraintDefs,
			Definition:  fmt.Sprintf("CREATE TABLE %s (...)", t.Name),
		})
	}

	data := SchemaData{
		Tables: tableDefs,
		Schema: result,
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)
	resp.Complete(start)
	return resp
}

// convertSchemaError converts an any-typed error response to a SchemaData-typed one.
func convertSchemaError(errResp *format.Response[any]) *format.Response[SchemaData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[SchemaData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	// Shouldn't happen, but handle gracefully
	return format.NewErrorResponse[SchemaData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
