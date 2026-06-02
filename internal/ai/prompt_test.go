package ai

import (
	"context"
	"strings"
	"testing"
)

func TestBuildSQLGenerationPromptIncludesDialect(t *testing.T) {
	budget := NewTokenBudget(100000)
	ctx := context.Background()
	sqlCtx := &SQLGenerationContext{
		Question: "show me orders",
		Dialect:  "mysql",
		SchemaSlimJSON: `{"tables": [{"name": "orders", "columns": ["id", "total"]}]}`,
	}

	system, _, _ := BuildSQLGenerationPrompt(ctx, sqlCtx, budget)
	if !strings.Contains(system, "mysql") {
		t.Errorf("expected system prompt to contain dialect 'mysql', got: %s", system)
	}
}

func TestBuildSQLGenerationPromptBudgetTruncation(t *testing.T) {
	budget := NewTokenBudget(20)
	ctx := context.Background()
	sqlCtx := &SQLGenerationContext{
		Question: "test",
		Dialect:  "mysql",
		SchemaSlimJSON: strings.Repeat("x", 500),
		JoinGraphJSON:  strings.Repeat("y", 500),
	}

	_, user, warnings := BuildSQLGenerationPrompt(ctx, sqlCtx, budget)
	if len(warnings) == 0 {
		t.Error("expected warnings from budget truncation")
	}
	if user == "" {
		t.Error("expected non-empty user prompt")
	}
}

func TestBuildSQLGenerationPromptWarningsPropagated(t *testing.T) {
	budget := NewTokenBudget(30)
	ctx := context.Background()
	sqlCtx := &SQLGenerationContext{
		Question: "test query",
		Dialect:  "postgresql",
		SchemaSlimJSON: strings.Repeat("x", 300),
		IndexInfo:      strings.Repeat("y", 300),
	}

	_, _, warnings := BuildSQLGenerationPrompt(ctx, sqlCtx, budget)
	if len(warnings) < 1 {
		t.Error("expected at least one warning from budget pressure")
	}
}

func TestBuildOptimizationPromptIncludesAllSections(t *testing.T) {
	budget := NewTokenBudget(100000)
	ctx := context.Background()

	system, user := BuildOptimizationPrompt(ctx, "SELECT * FROM orders",
		"Seq Scan on orders",
		"orders table schema",
		"no joins",
		"100 rows",
		"index on id",
		"postgresql",
		nil,
		budget)

	if !strings.Contains(system, "postgresql") {
		t.Errorf("expected system to contain dialect, got: %s", system)
	}
	if !strings.Contains(user, "SELECT") {
		t.Errorf("expected user prompt to contain original SQL")
	}
}
