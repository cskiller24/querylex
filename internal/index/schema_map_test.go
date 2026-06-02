package index

import (
	"testing"

	"github.com/querylex/querylex/internal/db"
)

func TestBuildSchemaMap(t *testing.T) {
	result := &db.SchemaResult{
		Tables: []db.TableInfo{
			{
				Schema: "testdb",
				Name:   "users",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int(11)", IsPrimaryKey: true},
					{Name: "name", Ordinal: 2, ColumnType: "varchar(255)"},
					{Name: "email", Ordinal: 3, ColumnType: "varchar(355)", IsNullable: true},
				},
				Constraints: []db.ConstraintInfo{
					{Name: "pk_users", Type: "PRIMARY_KEY", Columns: []string{"id"}},
				},
				Indexes: []db.IndexInfo{
					{Name: "users_pkey", Type: "BTREE", IsUnique: true, Primary: true, Columns: []db.IndexColumn{{Name: "id", Order: "ASC", Sequence: 1}}},
					{Name: "users_email_idx", Type: "BTREE", IsUnique: true, Columns: []db.IndexColumn{{Name: "email", Order: "ASC", Sequence: 1}}},
				},
			},
			{
				Schema: "testdb",
				Name:   "orders",
				Columns: []db.ColumnInfo{
					{Name: "id", Ordinal: 1, ColumnType: "int(11)", IsPrimaryKey: true},
					{Name: "user_id", Ordinal: 2, ColumnType: "int(11)"},
					{Name: "amount", Ordinal: 3, ColumnType: "decimal(10,2)"},
				},
				Constraints: []db.ConstraintInfo{
					{Name: "pk_orders", Type: "PRIMARY_KEY", Columns: []string{"id"}},
				},
				Indexes: []db.IndexInfo{
					{Name: "orders_pkey", Type: "BTREE", IsUnique: true, Primary: true, Columns: []db.IndexColumn{{Name: "id", Order: "ASC", Sequence: 1}}},
				},
			},
		},
	}

	schemaMap, err := BuildSchemaMap(result)
	if err != nil {
		t.Fatalf("BuildSchemaMap failed: %v", err)
	}

	// Check users table entry
	usersEntry, ok := schemaMap["users"]
	if !ok {
		t.Fatal("expected 'users' entry in schema map")
	}

	if usersEntry.Table != "users" {
		t.Fatalf("expected table name 'users', got '%s'", usersEntry.Table)
	}

	// Check PK columns
	if len(usersEntry.PKColumns) != 1 || usersEntry.PKColumns[0] != "id" {
		t.Fatalf("expected PK columns ['id'], got %v", usersEntry.PKColumns)
	}

	// Check indexed columns includes email
	foundEmail := false
	for _, col := range usersEntry.IndexedColumns {
		if col == "email" {
			foundEmail = true
			break
		}
	}
	if !foundEmail {
		t.Fatalf("expected 'email' in indexed_columns, got %v", usersEntry.IndexedColumns)
	}

	// Check orders table entry
	ordersEntry, ok := schemaMap["orders"]
	if !ok {
		t.Fatal("expected 'orders' entry in schema map")
	}

	if ordersEntry.Table != "orders" {
		t.Fatalf("expected table name 'orders', got '%s'", ordersEntry.Table)
	}

	// CompositeIndexes should be empty for simple indexes
	if len(usersEntry.CompositeIndexes) != 0 {
		t.Fatalf("expected no composite indexes, got %v", usersEntry.CompositeIndexes)
	}
}

func TestBuildSchemaMap_Empty(t *testing.T) {
	result := &db.SchemaResult{}
	schemaMap, err := BuildSchemaMap(result)
	if err != nil {
		t.Fatalf("BuildSchemaMap failed: %v", err)
	}

	if len(schemaMap) != 0 {
		t.Fatalf("expected empty schema map, got %d entries", len(schemaMap))
	}
}
