package cli

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// mockIndexesAdapter implements db.Adapter for testing Indexes().
type mockIndexesAdapter struct {
	db.Adapter
	indexesResult *db.IndexesResult
	err           error
}

func (m *mockIndexesAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	return m.indexesResult, m.err
}

func (m *mockIndexesAdapter) Close(ctx context.Context) error {
	return nil
}

func TestRunIndexesInternal_Live(t *testing.T) {
	adapter := &mockIndexesAdapter{
		indexesResult: &db.IndexesResult{
			Tables: []db.TableIndexInfo{
				{
					Table: "users",
					Indexes: []db.IndexInfo{
						{
							Name:    "PRIMARY",
							Type:    "BTREE",
							IsUnique: true,
							Primary: true,
							Visible: true,
							Columns: []db.IndexColumn{
								{Name: "id", Order: "ASC", Sequence: 1, Cardinality: 90321},
							},
						},
						{
							Name:     "users_email_uq",
							Type:     "BTREE",
							IsUnique: true,
							Visible:  true,
							Columns: []db.IndexColumn{
								{Name: "email", Order: "ASC", Sequence: 1, Cardinality: 90321},
							},
							Comment: "Unique customer email.",
						},
					},
				},
			},
		},
	}

	traceID := "test-trace-id"
	resp := runIndexesInternal(adapter, []string{"users"}, true, traceID, nil)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if len(resp.Data.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resp.Data.Tables))
	}

	table := resp.Data.Tables[0]
	if table.Table != "users" {
		t.Fatalf("expected table 'users', got '%s'", table.Table)
	}

	if len(table.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(table.Indexes))
	}

	// Check PRIMARY index
	primary := table.Indexes[0]
	if primary.Name != "PRIMARY" {
		t.Fatalf("expected first index 'PRIMARY', got '%s'", primary.Name)
	}
	if !primary.Unique {
		t.Fatal("expected PRIMARY to be unique")
	}
	if !primary.Primary {
		t.Fatal("expected PRIMARY.primary=true")
	}

	// Check email index
	emailIdx := table.Indexes[1]
	if emailIdx.Name != "users_email_uq" {
		t.Fatalf("expected second index 'users_email_uq', got '%s'", emailIdx.Name)
	}
	if !emailIdx.Unique {
		t.Fatal("expected email index to be unique")
	}
	if emailIdx.Comment != "Unique customer email." {
		t.Fatalf("expected comment 'Unique customer email.', got '%s'", emailIdx.Comment)
	}

	if resp.Meta.TraceID != traceID {
		t.Fatalf("expected trace_id '%s', got '%s'", traceID, resp.Meta.TraceID)
	}
	if resp.Meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version 1.0.0, got %s", resp.Meta.ProtocolVersion)
	}
}

func TestRunIndexesInternal_AdapterError(t *testing.T) {
	adapter := &mockIndexesAdapter{
		err: db.ErrConnectionFailed,
	}

	traceID := "test-trace-id"
	resp := runIndexesInternal(adapter, []string{"users"}, true, traceID, nil)

	if resp.Success {
		t.Fatal("expected failure on adapter error")
	}

	if resp.Error == nil {
		t.Fatal("expected error detail")
	}

	if resp.Error.Code != format.ErrCodeInternalError {
		t.Fatalf("expected INTERNAL_ERROR, got %s", resp.Error.Code)
	}
}
