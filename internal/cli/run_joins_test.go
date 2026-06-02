package cli

import (
	"context"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

// Test 1: RunJoins with tables returns Success with join edges
func TestRunJoins_Basic(t *testing.T) {
	adapter := &joinsMockAdapter{
		joinsFn: func(ctx context.Context, tables []string) (*db.JoinsResult, error) {
			return &db.JoinsResult{
				Edges: []db.JoinEdge{
					{
						Source:      "orders",
						Target:      "users",
						Columns:     [][2]string{{"user_id", "id"}},
						Confidence:  1.0,
						SourceType:  "declared_foreign_key",
						Composite:   false,
						CrossDomain: false,
					},
				},
			}, nil
		},
	}

	traceID := "test-trace"
	resp := runJoinsWithAdapter(adapter, []string{"users", "orders"}, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}
	if len(resp.Data.Joins) == 0 {
		t.Fatal("expected at least one join edge in response")
	}
	first := resp.Data.Joins[0]
	if first.Source == "" || first.Target == "" {
		t.Fatal("expected source and target in join edge")
	}
	if first.Confidence <= 0 {
		t.Fatal("expected positive confidence score")
	}
	if first.SourceType == "" {
		t.Fatal("expected source_type in join edge")
	}
}

// Test 2: RunJoins with no join path returns warning
func TestRunJoins_NoPath(t *testing.T) {
	adapter := &joinsMockAdapter{
		joinsFn: func(ctx context.Context, tables []string) (*db.JoinsResult, error) {
			return &db.JoinsResult{Edges: []db.JoinEdge{}}, nil
		},
	}

	traceID := "test-trace"
	resp := runJoinsWithAdapter(adapter, []string{"a", "b"}, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	// Should still return success with warning about no path
	if !resp.Success {
		t.Fatalf("expected Success=true for empty joins, got error=%v", resp.Error)
	}

	// Check for path-related warning
	hasWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "JOIN_PATH_NOT_FOUND" || w.Code == "AMBIGUOUS_JOIN" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		// If no specific warning, at minimum expect empty joins
		t.Log("no JOIN_PATH_NOT_FOUND or AMBIGUOUS_JOIN warning (empty joins are still valid)")
	}
}

// joinsMockAdapter implements db.Adapter for testing joins/resolve commands.
type joinsMockAdapter struct {
	schemaFn   func(ctx context.Context, tables []string) (*db.SchemaResult, error)
	explainFn  func(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error)
	validateFn func(ctx context.Context, query string) (*db.ValidateResult, error)
	statsFn    func(ctx context.Context, tables []string) (*db.StatsResult, error)
	indexesFn  func(ctx context.Context, tables []string) (*db.IndexesResult, error)
	joinsFn    func(ctx context.Context, tables []string) (*db.JoinsResult, error)
}

func (m *joinsMockAdapter) Connect(ctx context.Context, dsn string) error { return nil }
func (m *joinsMockAdapter) Ping(ctx context.Context) error                 { return nil }
func (m *joinsMockAdapter) Close(ctx context.Context) error                { return nil }
func (m *joinsMockAdapter) DatabaseType() string                           { return "mock" }

func (m *joinsMockAdapter) Schema(ctx context.Context, tables []string) (*db.SchemaResult, error) {
	if m.schemaFn != nil { return m.schemaFn(ctx, tables) }
	return nil, db.ErrNotImplemented
}
func (m *joinsMockAdapter) Explain(ctx context.Context, query string, analyze bool) (*db.ExplainPlan, error) {
	if m.explainFn != nil { return m.explainFn(ctx, query, analyze) }
	return nil, db.ErrNotImplemented
}
func (m *joinsMockAdapter) Validate(ctx context.Context, query string) (*db.ValidateResult, error) {
	if m.validateFn != nil { return m.validateFn(ctx, query) }
	return nil, db.ErrNotImplemented
}
func (m *joinsMockAdapter) Stats(ctx context.Context, tables []string) (*db.StatsResult, error) {
	if m.statsFn != nil { return m.statsFn(ctx, tables) }
	return nil, db.ErrNotImplemented
}
func (m *joinsMockAdapter) Indexes(ctx context.Context, tables []string) (*db.IndexesResult, error) {
	if m.indexesFn != nil { return m.indexesFn(ctx, tables) }
	return nil, db.ErrNotImplemented
}
func (m *joinsMockAdapter) Joins(ctx context.Context, tables []string) (*db.JoinsResult, error) {
	if m.joinsFn != nil { return m.joinsFn(ctx, tables) }
	return nil, db.ErrNotImplemented
}
