package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/querylex/querylex/internal/db"
)

// TableMapEntry is the per-table entry in the schema map.
type TableMapEntry struct {
	Table      string          `json:"table"`
	Schema     string          `json:"schema"`
	PKColumns  []string        `json:"pk_columns"`
	FKIn       []FKEdge        `json:"fk_in,omitempty"`
	FKOut      []FKEdge        `json:"fk_out,omitempty"`
	IndexedColumns []string    `json:"indexed_columns,omitempty"`
	CompositeIndexes [][]string `json:"composite_indexes,omitempty"`
}

// FKEdge describes a foreign key edge.
type FKEdge struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// SchemaMap is the map of table name to TableMapEntry.
type SchemaMap map[string]*TableMapEntry

// BuildSchemaMap produces a fast lookup map from SchemaResult.
func BuildSchemaMap(result *db.SchemaResult) (SchemaMap, error) {
	sm := make(SchemaMap)

	for _, t := range result.Tables {
		entry := &TableMapEntry{
			Table:  t.Name,
			Schema: t.Schema,
		}

		// Extract PK columns
		for _, c := range t.Columns {
			if c.IsPrimaryKey {
				entry.PKColumns = append(entry.PKColumns, c.Name)
			}
		}

		// Extract FK outbound (this table references others)
		for _, cons := range t.Constraints {
			if cons.Type == "FOREIGN_KEY" && cons.ReferencedTable != "" {
				for _, col := range cons.Columns {
					entry.FKOut = append(entry.FKOut, FKEdge{
						Table:  cons.ReferencedTable,
						Column: col,
					})
				}
			}
		}

		// Extract indexed columns
		for _, idx := range t.Indexes {
			if len(idx.Columns) == 1 {
				entry.IndexedColumns = append(entry.IndexedColumns, idx.Columns[0].Name)
			} else if len(idx.Columns) > 1 {
				composite := make([]string, len(idx.Columns))
				for i, c := range idx.Columns {
					composite[i] = c.Name
					// Also add individual columns to indexed_columns
					alreadyIn := false
					for _, ec := range entry.IndexedColumns {
						if ec == c.Name {
							alreadyIn = true
							break
						}
					}
					if !alreadyIn {
						entry.IndexedColumns = append(entry.IndexedColumns, c.Name)
					}
				}
				entry.CompositeIndexes = append(entry.CompositeIndexes, composite)
			}
		}

		sm[t.Name] = entry
	}

	// Build FKIn (others reference this table)
	for _, t := range result.Tables {
		for _, cons := range t.Constraints {
			if cons.Type == "FOREIGN_KEY" && cons.ReferencedTable != "" {
				for _, col := range cons.Columns {
					if target, ok := sm[cons.ReferencedTable]; ok {
						target.FKIn = append(target.FKIn, FKEdge{
							Table:  t.Name,
							Column: col,
						})
					}
				}
			}
		}
	}

	return sm, nil
}

// WriteSchemaMap writes the schema map to <dbDir>/schema/schema_map.json.
func WriteSchemaMap(dbDir string, sm SchemaMap) error {
	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}

	data, err := json.MarshalIndent(sm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schema map: %w", err)
	}

	return os.WriteFile(filepath.Join(schemaDir, "schema_map.json"), data, 0644)
}
