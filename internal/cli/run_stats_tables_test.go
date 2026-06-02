package cli

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// mockStatsAdapter implements db.Adapter for testing Stats().
type mockStatsAdapter struct {
	db.Adapter
	statsResult *db.StatsResult
	err         error
}

func (m *mockStatsAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	return m.statsResult, m.err
}

func (m *mockStatsAdapter) Close(ctx context.Context) error {
	return nil
}

func TestRunStatsTablesInternal_Success(t *testing.T) {
	adapter := &mockStatsAdapter{
		statsResult: &db.StatsResult{
			Tables: []db.TableStats{
				{
					Name:               "users",
					RowCount:           90321,
					CardinalityEstimate: 90321,
					DataSizeBytes:      83886080,
					IndexSizeBytes:     25165824,
					Freshness:          "fresh",
					UpdatedAt:          "2026-05-29T00:00:00Z",
				},
			},
		},
	}

	traceID := "test-trace-id"
	activeDB := "test-db"
	resp := runStatsTablesInternal(adapter, []string{"users"}, traceID, &activeDB)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if len(resp.Data.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resp.Data.Tables))
	}

	table := resp.Data.Tables[0]
	if table.Name != "users" {
		t.Fatalf("expected table name 'users', got '%s'", table.Name)
	}
	if table.RowCount != 90321 {
		t.Fatalf("expected RowCount 90321, got %d", table.RowCount)
	}

	if resp.Meta.TraceID != traceID {
		t.Fatalf("expected trace_id '%s', got '%s'", traceID, resp.Meta.TraceID)
	}
	if resp.Meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version 1.0.0, got %s", resp.Meta.ProtocolVersion)
	}
}

func TestRunStatsTablesInternal_Empty(t *testing.T) {
	adapter := &mockStatsAdapter{
		statsResult: &db.StatsResult{},
	}

	traceID := "test-trace-id"
	resp := runStatsTablesInternal(adapter, nil, traceID, nil)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if len(resp.Data.Tables) != 0 {
		t.Fatalf("expected 0 tables, got %d", len(resp.Data.Tables))
	}
}

func TestRunStatsTablesInternal_AdapterError(t *testing.T) {
	adapter := &mockStatsAdapter{
		err: db.ErrConnectionFailed,
	}

	traceID := "test-trace-id"
	resp := runStatsTablesInternal(adapter, []string{"users"}, traceID, nil)

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

func TestRunStatsTablesInternal_ProtocolVersion(t *testing.T) {
	adapter := &mockStatsAdapter{
		statsResult: &db.StatsResult{
			Tables: []db.TableStats{
				{Name: "orders", RowCount: 1000},
			},
		},
	}

	traceID := "550e8400-e29b-41d4-a716-446655440000"
	resp := runStatsTablesInternal(adapter, []string{"orders"}, traceID, nil)

	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if resp.Meta.TraceID != traceID {
		t.Fatalf("expected trace_id '%s', got '%s'", traceID, resp.Meta.TraceID)
	}
	if resp.Meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version 1.0.0, got %s", resp.Meta.ProtocolVersion)
	}
}
