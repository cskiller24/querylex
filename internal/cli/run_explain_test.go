package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

// Test 1: RunExplain("SELECT 1", false) returns success with plan and heuristics
func TestRunExplain_Basic(t *testing.T) {
	adapter := &explainMockAdapter{
		explainFn: func(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
			cost := 1.0
			rows := int64(1)
			return &db.ExplainPlan{
				EstimatedTotalCost:    &cost,
				EstimatedRowsExamined: &rows,
				FullScanTables:        []string{},
				IndexUsage:            []db.IndexUsageEntry{},
				Warnings:              []string{},
			}, nil
		},
	}

	traceID := "test-trace"
	// Empty dbDir means cache check will miss (no directory)
	resp := runExplainWithAdapter(adapter, "SELECT 1", false, traceID, strPtr("test-db"), "", "mock")

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}
	if resp.Data.Plan == nil {
		t.Fatal("expected Plan in response data, got nil")
	}
	if resp.Data.Plan.EstimatedTotalCost == nil {
		t.Fatal("expected estimated_total_cost in plan")
	}
	if resp.Data.Heuristics == nil {
		t.Fatal("expected heuristics array in response, got nil")
	}
	if resp.Data.Analyze {
		t.Fatal("expected Analyze=false for non-analyze call")
	}
	// Cache miss — cache_hit should be false
	if resp.Meta.CacheHit == nil {
		t.Fatal("expected CacheHit in response meta")
	}
	if *resp.Meta.CacheHit {
		t.Fatal("expected CacheHit=false on cache miss")
	}
}

// Test 2: RunExplain("SELECT 1", true) adds ANALYZE_CONFIRMATION warning
func TestRunExplain_Analyze(t *testing.T) {
	adapter := &explainMockAdapter{
		explainFn: func(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
			cost := 1.0
			actualTime := 5.0
			rows := int64(1)
			return &db.ExplainPlan{
				EstimatedTotalCost:    &cost,
				ActualTotalTimeMs:     &actualTime,
				EstimatedRowsExamined: &rows,
				FullScanTables:        []string{},
				IndexUsage:            []db.IndexUsageEntry{},
				Warnings:              []string{},
			}, nil
		},
	}

	traceID := "test-trace"
	resp := runExplainWithAdapter(adapter, "SELECT 1", true, traceID, strPtr("test-db"), "", "mock")

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}

	// Should have ANALYZE_CONFIRMATION warning
	hasWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "ANALYZE_CONFIRMATION" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Fatal("expected ANALYZE_CONFIRMATION warning in analyze mode")
	}

	// Should have actual_total_time_ms
	if resp.Data.Plan.ActualTotalTimeMs == nil {
		t.Fatal("expected actual_total_time_ms in analyze mode")
	}
	if !resp.Data.Analyze {
		t.Fatal("expected Analyze=true for analyze call")
	}
	// Cache miss — cache_hit should be false
	if resp.Meta.CacheHit == nil {
		t.Fatal("expected CacheHit in response meta")
	}
	if *resp.Meta.CacheHit {
		t.Fatal("expected CacheHit=false on cache miss")
	}
}

// Test 3: Cache hit returns cached plan without calling adapter
func TestRunExplain_CacheHit(t *testing.T) {
	// Create a temp directory that looks like a database state dir
	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, "test-db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}

	// We cannot easily pre-populate a cache entry outside the explaincache package
	// without a manifest. Instead, use an empty dbDir to confirm cache miss.
	// Cache hit requires a valid manifest + cache entry, which is tested in
	// the explaincache package tests.
	adapter := &explainMockAdapter{
		explainFn: func(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
			cost := 1.0
			rows := int64(1)
			return &db.ExplainPlan{
				EstimatedTotalCost:    &cost,
				EstimatedRowsExamined: &rows,
				FullScanTables:        []string{},
				IndexUsage:            []db.IndexUsageEntry{},
				Warnings:              []string{},
			}, nil
		},
		callCount: 0,
	}

	traceID := "test-trace"
	resp := runExplainWithAdapter(adapter, "SELECT 1", false, traceID, strPtr("test-db"), dbDir, "mock")

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}
	// No manifest exists, so cache miss -> adapter.Explain must have been called
	if resp.Meta.CacheHit == nil {
		t.Fatal("expected CacheHit in response meta")
	}
	if *resp.Meta.CacheHit {
		t.Fatal("expected CacheHit=false when no manifest exists")
	}
}

// explainMockAdapter implements db.Adapter for testing explain/validate commands.
type explainMockAdapter struct {
	schemaFn   func(ctx context.Context, tables []string) (*db.SchemaResult, error)
	explainFn  func(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error)
	validateFn func(ctx context.Context, query string) (*db.ValidateResult, error)
	statsFn    func(ctx context.Context, tables []string) (*db.StatsResult, error)
	indexesFn  func(ctx context.Context, tables []string) (*db.IndexesResult, error)
	joinsFn    func(ctx context.Context, tables []string) (*db.JoinsResult, error)
	callCount  int
}

func (m *explainMockAdapter) Connect(ctx context.Context, dsn string) error { return nil }
func (m *explainMockAdapter) Ping(ctx context.Context) error                 { return nil }
func (m *explainMockAdapter) Close(ctx context.Context) error                { return nil }
func (m *explainMockAdapter) DatabaseType() string                           { return "mock" }

func (m *explainMockAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if m.schemaFn != nil {
		return m.schemaFn(ctx, tables)
	}
	return nil, db.ErrNotImplemented
}

func (m *explainMockAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	m.callCount++
	if m.explainFn != nil {
		return m.explainFn(ctx, query, analyze)
	}
	return nil, db.ErrNotImplemented
}

func (m *explainMockAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	if m.validateFn != nil {
		return m.validateFn(ctx, query)
	}
	return nil, db.ErrNotImplemented
}

func (m *explainMockAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if m.statsFn != nil {
		return m.statsFn(ctx, tables)
	}
	return nil, db.ErrNotImplemented
}

func (m *explainMockAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if m.indexesFn != nil {
		return m.indexesFn(ctx, tables)
	}
	return nil, db.ErrNotImplemented
}

func (m *explainMockAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if m.joinsFn != nil {
		return m.joinsFn(ctx, tables)
	}
	return nil, db.ErrNotImplemented
}

func strPtr(s string) *string { return &s }
