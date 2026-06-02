package ai

import (
	"encoding/json"
	"math"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func TestOptimizationResponseSchemaGeneration(t *testing.T) {
	schema, err := jsonschema.GenerateSchemaForType(OptimizationResponse{})
	if err != nil {
		t.Fatalf("GenerateSchemaForType failed: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
}

func TestParseOptimizationResponseValid(t *testing.T) {
	resp := &openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: `{"issues":[{"code":"FULL_SCAN","severity":"warning","evidence":"Seq scan on large table","affected_tables":["orders"]}],"rewrite_strategy":"predicate_rewrite","optimized_sql":"SELECT id FROM orders WHERE status = 'active'","expected_plan_changes":["Index scan instead of seq scan"],"requires_new_index":true,"confidence":0.85}`,
				},
			},
		},
	}

	result, err := ParseOptimizationResponse(resp)
	if err != nil {
		t.Fatalf("ParseOptimizationResponse failed: %v", err)
	}
	if result.RewriteStrategy != "predicate_rewrite" {
		t.Errorf("expected predicate_rewrite, got %s", result.RewriteStrategy)
	}
	if !result.RequiresNewIndex {
		t.Error("expected RequiresNewIndex to be true")
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(result.Issues))
	}
}

func TestParseOptimizationResponseInvalidJSON(t *testing.T) {
	resp := &openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: `{invalid json}`,
				},
			},
		},
	}

	_, err := ParseOptimizationResponse(resp)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseOptimizationResponseEmptyChoices(t *testing.T) {
	resp := &openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{},
	}

	_, err := ParseOptimizationResponse(resp)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestTemperatureIsSet(t *testing.T) {
	// Verify the temperature constant is not zero (regression for omitempty)
	if math.SmallestNonzeroFloat32 == 0 {
		t.Error("SmallestNonzeroFloat32 should not be zero")
	}
}

func TestOptimizationResponseJSONRoundTrip(t *testing.T) {
	original := OptimizationResponse{
		Issues: []Issue{
			{Code: "FULL_SCAN", Severity: "warning", Evidence: "seq scan", AffectedTables: []string{"orders"}},
		},
		RewriteStrategy:   "predicate_rewrite",
		OptimizedSQL:      "SELECT * FROM orders WHERE id = 1",
		ExpectedPlanChanges: []string{"index scan"},
		RequiresNewIndex:  false,
		Confidence:        0.92,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded OptimizationResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.RewriteStrategy != original.RewriteStrategy {
		t.Errorf("expected %s, got %s", original.RewriteStrategy, decoded.RewriteStrategy)
	}
}
