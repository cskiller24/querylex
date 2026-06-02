package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// IndexData is the response data for the indexes command.
type IndexData struct {
	Tables []IndexTableEntry `json:"tables"`
}

// IndexTableEntry describes indexes for a single table.
type IndexTableEntry struct {
	Table   string      `json:"table"`
	Indexes []IndexDef  `json:"indexes"`
}

// IndexDef describes a single index.
type IndexDef struct {
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Unique  bool             `json:"unique"`
	Primary bool             `json:"primary"`
	Visible bool             `json:"visible"`
	Columns []IndexColDef    `json:"columns"`
	Expression string        `json:"expression,omitempty"`
	Comment  string          `json:"comment,omitempty"`
}

// IndexColDef describes a column in an index.
type IndexColDef struct {
	Name        string `json:"name"`
	Order       string `json:"order"`
	Sequence    int    `json:"sequence"`
	Cardinality int64  `json:"cardinality"`
}

// RunIndexes executes the indexes command.
// When live=false, reads from schema_map.json on disk.
// When live=true, queries the database directly.
func RunIndexes(tables []string, live bool) *format.Response[IndexData] {
	if !live {
		return runIndexesFromDisk(tables)
	}

	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertIndexesError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()
	return runIndexesInternal(preflight.Adapter, tables, true, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runIndexesFromDisk reads indexes from schema_map.json on disk.
func runIndexesFromDisk(tables []string) *format.Response[IndexData] {
	traceID := format.GenerateTraceID()
	start := time.Now()

	home, err := os.UserHomeDir()
	if err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	workspaceFile := filepath.Join(home, ".querylex", "querylex.json")
	wsData, err := os.ReadFile(workspaceFile)
	if err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	var ws struct {
		ActiveDatabaseID *string `json:"active_database_id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeWorkspaceStateInvalid,
			"Failed to parse workspace state",
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	if ws.ActiveDatabaseID == nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeInvalidArgument,
			"No active database set",
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Load schema_map.json
	dbDir := filepath.Join(home, ".querylex", *ws.ActiveDatabaseID)
	schemaMapPath := filepath.Join(dbDir, "schema", "schema_map.json")
	schemaMapData, err := os.ReadFile(schemaMapPath)
	if err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeSchemaParseError,
			fmt.Sprintf("Failed to read schema_map.json: %v", err),
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaMapData, &schemaMap); err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeSchemaParseError,
			"Failed to parse schema_map.json",
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Build index data from schema map entries
	var tableEntries []IndexTableEntry
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t] = true
	}

	for tableName, tableData := range schemaMap {
		if len(tables) > 0 && !tableSet[tableName] {
			continue
		}

		entry, ok := tableData.(map[string]any)
		if !ok {
			continue
		}

		var indexes []IndexDef

		// Add primary key index if PK columns exist
		if pkCols, ok := entry["pk_columns"].([]any); ok && len(pkCols) > 0 {
			var pkIndexCols []IndexColDef
			for i, col := range pkCols {
				if colName, ok := col.(string); ok {
					pkIndexCols = append(pkIndexCols, IndexColDef{
						Name:     colName,
						Order:    "ASC",
						Sequence: i + 1,
					})
				}
			}
			indexes = append(indexes, IndexDef{
				Name:    "PRIMARY",
				Type:    "BTREE",
				Unique:  true,
				Primary: true,
				Visible: true,
				Columns: pkIndexCols,
			})
		}

		// Add indexed columns as simple indexes
		if indexedCols, ok := entry["indexed_columns"].([]any); ok {
			for _, col := range indexedCols {
				if colName, ok := col.(string); ok {
					// Skip if already in PK
					isPK := false
					for _, idx := range indexes {
						for _, idxCol := range idx.Columns {
							if idxCol.Name == colName && idx.Primary {
								isPK = true
								break
							}
						}
						if isPK {
							break
						}
					}
					if !isPK {
						indexes = append(indexes, IndexDef{
							Name:    fmt.Sprintf("idx_%s_%s", tableName, colName),
							Type:    "BTREE",
							Unique:  false,
							Primary: false,
							Visible: true,
							Columns: []IndexColDef{
								{Name: colName, Order: "ASC", Sequence: 1},
							},
						})
					}
				}
			}
		}

		// Add composite indexes
		if compositeIdx, ok := entry["composite_indexes"].([]any); ok {
			for _, comp := range compositeIdx {
				if compCols, ok := comp.([]any); ok && len(compCols) > 0 {
					var idxCols []IndexColDef
					for i, col := range compCols {
						if colName, ok := col.(string); ok {
							idxCols = append(idxCols, IndexColDef{
								Name:     colName,
								Order:    "ASC",
								Sequence: i + 1,
							})
						}
					}
					if len(idxCols) > 0 {
						indexes = append(indexes, IndexDef{
							Name:    fmt.Sprintf("idx_%s_composite", tableName),
							Type:    "BTREE",
							Unique:  false,
							Primary: false,
							Visible: true,
							Columns: idxCols,
						})
					}
				}
			}
		}

		tableEntries = append(tableEntries, IndexTableEntry{
			Table:   tableName,
			Indexes: indexes,
		})
	}

	data := IndexData{Tables: tableEntries}
	resp := format.NewSuccessResponse(data, traceID, ws.ActiveDatabaseID)
	resp.Complete(start)
	return resp
}

// runIndexesInternal executes the indexes command with a provided adapter.
func runIndexesInternal(adapter db.Adapter, tables []string, live bool, traceID string, activeDBID *string) *format.Response[IndexData] {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Indexes(ctx, tables)
	if err != nil {
		resp := format.NewErrorResponse[IndexData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Index extraction failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	tableEntries := make([]IndexTableEntry, 0, len(result.Tables))
	for _, t := range result.Tables {
		indexDefs := make([]IndexDef, 0, len(t.Indexes))
		for _, idx := range t.Indexes {
			colDefs := make([]IndexColDef, 0, len(idx.Columns))
			for _, c := range idx.Columns {
				colDefs = append(colDefs, IndexColDef{
					Name:        c.Name,
					Order:       c.Order,
					Sequence:    c.Sequence,
					Cardinality: c.Cardinality,
				})
			}
			indexDefs = append(indexDefs, IndexDef{
				Name:    idx.Name,
				Type:    idx.Type,
				Unique:  idx.IsUnique,
				Primary: idx.Primary,
				Visible: idx.Visible,
				Columns: colDefs,
				Comment: idx.Comment,
			})
		}
		tableEntries = append(tableEntries, IndexTableEntry{
			Table:   t.Table,
			Indexes: indexDefs,
		})
	}

	data := IndexData{Tables: tableEntries}
	resp := format.NewSuccessResponse(data, traceID, activeDBID)
	resp.Complete(start)
	return resp
}

// convertIndexesError converts an any-typed error response to an IndexData-typed one.
func convertIndexesError(errResp *format.Response[any]) *format.Response[IndexData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[IndexData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[IndexData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
