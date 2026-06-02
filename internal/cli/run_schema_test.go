package cli

import (
	"context"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

// mockSchemaAdapter implements db.Adapter for testing Schema().
type mockSchemaAdapter struct {
	db.Adapter
	schemaResult *db.SchemaResult
	err          error
}

func (m *mockSchemaAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	return m.schemaResult, m.err
}

func (m *mockSchemaAdapter) Close(ctx context.Context) error {
	return nil
}

func TestRunSchemaInternal_Success(t *testing.T) {
	adapter := &mockSchemaAdapter{
		schemaResult: &db.SchemaResult{
			Tables: []db.TableInfo{
				{
					Schema: "testdb",
					Name:   "users",
					Type:   "BASE TABLE",
					Columns: []db.ColumnInfo{
						{Name: "id", Ordinal: 1, ColumnType: "int(11)", IsPrimaryKey: true},
						{Name: "name", Ordinal: 2, ColumnType: "varchar(255)"},
					},
				},
			},
		},
	}

	traceID := "test-trace-id"
	activeDB := "test-db"
	resp := runSchemaInternal(adapter, []string{"users"}, traceID, &activeDB)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if resp.Data.Schema == nil {
		t.Fatal("expected Schema in data")
	}

	if len(resp.Data.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resp.Data.Tables))
	}

	if resp.Data.Tables[0].Name != "users" {
		t.Fatalf("expected table name 'users', got '%s'", resp.Data.Tables[0].Name)
	}

	if resp.Meta.TraceID != traceID {
		t.Fatalf("expected trace_id '%s', got '%s'", traceID, resp.Meta.TraceID)
	}
}

func TestRunSchemaInternal_NoTables(t *testing.T) {
	adapter := &mockSchemaAdapter{
		schemaResult: &db.SchemaResult{},
	}

	traceID := "test-trace-id"
	resp := runSchemaInternal(adapter, nil, traceID, nil)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if len(resp.Data.Tables) != 0 {
		t.Fatalf("expected 0 tables, got %d", len(resp.Data.Tables))
	}
}

func TestRunSchemaInternal_AdapterError(t *testing.T) {
	adapter := &mockSchemaAdapter{
		err: db.ErrConnectionFailed,
	}

	traceID := "test-trace-id"
	resp := runSchemaInternal(adapter, []string{"users"}, traceID, nil)

	if resp.Success {
		t.Fatal("expected failure on adapter error")
	}

	if resp.Error == nil {
		t.Fatal("expected error detail")
	}

	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("expected INTERNAL_ERROR, got %s", resp.Error.Code)
	}
}
