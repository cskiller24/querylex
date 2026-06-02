package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cskiller24/querylex/internal/db"
)

// BuildSchema transforms a SchemaResult into the full schema.json artifact.
func BuildSchema(result *db.SchemaResult) ([]byte, error) {
	type columnDef struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Nullable  bool   `json:"nullable"`
		Default   any    `json:"default,omitempty"`
		PrimaryKey bool  `json:"primary_key"`
		Generated bool   `json:"generated"`
		Comment   string `json:"comment,omitempty"`
	}

	type constraintDef struct {
		Name              string   `json:"name"`
		Type              string   `json:"type"`
		Columns           []string `json:"columns"`
		ReferencedTable   string   `json:"referenced_table,omitempty"`
		ReferencedColumns []string `json:"referenced_columns,omitempty"`
	}

	type indexDef struct {
		Name    string   `json:"name"`
		Def     string   `json:"def"`
		Table   string   `json:"table"`
		Columns []string `json:"columns"`
		Comment string   `json:"comment,omitempty"`
	}

	type tableDef struct {
		Name        string           `json:"name"`
		Schema      string           `json:"schema"`
		Type        string           `json:"type"`
		Comment     string           `json:"comment"`
		Columns     []columnDef      `json:"columns"`
		Constraints []constraintDef  `json:"constraints"`
		Indexes     []indexDef       `json:"indexes"`
		Definition  string           `json:"definition"`
	}

	type schemaOutput struct {
		Name   string     `json:"name"`
		Desc   string     `json:"desc"`
		Tables []tableDef `json:"tables"`
	}

	output := schemaOutput{
		Name:   "querylex",
		Desc:   "Imported database schema",
		Tables: []tableDef{}, // ensure non-nil for consistent JSON output
	}

	for _, t := range result.Tables {
		table := tableDef{
			Name:   t.Name,
			Schema: t.Schema,
			Type:   t.Type,
			Comment: t.Comment,
		}

		for _, c := range t.Columns {
			col := columnDef{
				Name:       c.Name,
				Type:       c.ColumnType,
				Nullable:   c.IsNullable,
				PrimaryKey: c.IsPrimaryKey,
				Generated:  c.IsGenerated,
				Comment:    c.Comment,
			}
			if c.Default != "" {
				col.Default = c.Default
			}
			table.Columns = append(table.Columns, col)
		}

		for _, c := range t.Constraints {
			constraint := constraintDef{
				Name:    c.Name,
				Type:    c.Type,
				Columns: c.Columns,
			}
			if c.ReferencedTable != "" {
				constraint.ReferencedTable = c.ReferencedTable
				constraint.ReferencedColumns = c.ReferencedColumns
			}
			table.Constraints = append(table.Constraints, constraint)
		}

		for _, idx := range t.Indexes {
			colNames := make([]string, len(idx.Columns))
			for i, c := range idx.Columns {
				colNames[i] = c.Name
			}
			def := fmt.Sprintf("%s (%s)", idx.Name, strings.Join(colNames, ", "))
			index := indexDef{
				Name:    idx.Name,
				Def:     def,
				Table:   t.Name,
				Columns: colNames,
				Comment: idx.Comment,
			}
			table.Indexes = append(table.Indexes, index)
		}

		output.Tables = append(output.Tables, table)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("build schema json: %w", err)
	}

	return data, nil
}

// BuildSlimSchema produces a compact schema with only table names, column names, and types.
func BuildSlimSchema(result *db.SchemaResult) ([]byte, error) {
	type slimColumn struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	type slimTable struct {
		Name    string       `json:"name"`
		Columns []slimColumn `json:"columns"`
	}

	type slimOutput struct {
		Tables []slimTable `json:"tables"`
	}

	output := slimOutput{
		Tables: []slimTable{},
	}
	for _, t := range result.Tables {
		table := slimTable{
			Name: t.Name,
		}
		for _, c := range t.Columns {
			table.Columns = append(table.Columns, slimColumn{
				Name: c.Name,
				Type: c.ColumnType,
			})
		}
		output.Tables = append(output.Tables, table)
	}

	data, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("build slim schema json: %w", err)
	}

	return data, nil
}

// WriteSchema writes JSON data to <dbDir>/schema/schema.json.
func WriteSchema(dbDir string, data []byte) error {
	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}
	return os.WriteFile(filepath.Join(schemaDir, "schema.json"), data, 0644)
}
