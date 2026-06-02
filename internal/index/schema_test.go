package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/querylex/querylex/internal/db"
)

func TestBuildSchema(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb",
				Name:   "users",
				Type:   "BASE TABLE",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int(11)", IsNullable: false, IsPrimaryKey: true, Comment: "Primary key"},
					{Name: "name", Ordinal: 2, ColumnType: "varchar(255)", IsNullable: false, Comment: "Full name"},
					{Name: "email", Ordinal: 3, ColumnType: "varchar(355)", IsNullable: true, Default: "NULL", Comment: "Email address"},
				},
				Constraints: []db.ConstraintInfo{
					{Name: "pk_users", Type: "PRIMARY_KEY", Columns: []string{"id"}},
				},
				Indexes: []db.IndexInfo{
					{Name: "users_pkey", Type: "BTREE", IsUnique: true, Primary: true, Columns: []db.IndexColumn{{Name: "id", Order: "ASC", Sequence: 1}}},
				},
			},
		},
	}

	data, err := BuildSchema(result)
	if err != nil {
		t.Fatalf("BuildSchema failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("BuildSchema output is not valid JSON: %v", err)
	}

	// Check top-level fields
	tables, ok := parsed["tables"].([]any)
	if !ok {
		t.Fatal("expected 'tables' array in output")
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	table := tables[0].(map[string]any)
	if table["name"] != "users" {
		t.Fatalf("expected table name 'users', got '%v'", table["name"])
	}
	if table["schema"] != "testdb" {
		t.Fatalf("expected schema 'testdb', got '%v'", table["schema"])
	}

	// Check columns
	columns := table["columns"].([]any)
	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(columns))
	}

	firstCol := columns[0].(map[string]any)
	if firstCol["name"] != "id" {
		t.Fatalf("expected first column 'id', got '%v'", firstCol["name"])
	}
	if firstCol["type"] != "int(11)" {
		t.Fatalf("expected type 'int(11)', got '%v'", firstCol["type"])
	}
	if firstCol["nullable"] != false {
		t.Fatal("expected nullable=false for id column")
	}
	if firstCol["primary_key"] != true {
		t.Fatal("expected primary_key=true for id column")
	}
}

func TestBuildSchema_Empty(t *testing.T) {
	result := &db.SchemaResult{}
	data, err := BuildSchema(result)
	if err != nil {
		t.Fatalf("BuildSchema failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("BuildSchema output is not valid JSON: %v", err)
	}

	// The output should either have a 'tables' key (could be nil for empty) or the JSON
	// may omit it. Either is valid as long as the JSON parses.
	if tables, ok := parsed["tables"]; ok && tables != nil {
		if tblArr, ok := tables.([]any); ok && len(tblArr) != 0 {
			t.Fatalf("expected empty tables, got %d", len(tblArr))
		}
	}
}

func TestBuildSlimSchema(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb",
				Name:   "users",
				Columns: []db.ColumnInfo{
					{Name: "id", ColumnType: "int(11)", IsPrimaryKey: true},
					{Name: "name", ColumnType: "varchar(255)"},
				},
			},
		},
	}

	data, err := BuildSlimSchema(result)
	if err != nil {
		t.Fatalf("BuildSlimSchema failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("BuildSlimSchema output is not valid JSON: %v", err)
	}

	tables, ok := parsed["tables"].([]any)
	if !ok {
		t.Fatal("expected 'tables' array in slim output")
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	table := tables[0].(map[string]any)
	if table["name"] != "users" {
		t.Fatalf("expected table name 'users', got '%v'", table["name"])
	}

	// Slim schema should have columns array but simpler
	columns, ok := table["columns"].([]any)
	if !ok {
		t.Fatal("expected 'columns' in slim table")
	}
	if len(columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(columns))
	}

	firstCol := columns[0].(map[string]any)
	if firstCol["name"] != "id" {
		t.Fatalf("expected first column 'id', got '%v'", firstCol["name"])
	}
	if firstCol["type"] != "int(11)" {
		t.Fatalf("expected type 'int(11)', got '%v'", firstCol["type"])
	}
}

func TestWriteSchema(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"tables":[]}`)

	err := WriteSchema(dir, data)
	if err != nil {
		t.Fatalf("WriteSchema failed: %v", err)
	}

	schemaPath := filepath.Join(dir, "schema", "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		t.Fatal("schema.json not written")
	}

	written, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("cannot read schema.json: %v", err)
	}
	if string(written) != string(data) {
		t.Fatalf("written data mismatch: got %s, expected %s", string(written), string(data))
	}
}
