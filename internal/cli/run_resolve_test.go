package cli

import (
	"encoding/json"
	"testing"

	"github.com/querylex/querylex/internal/queryutil"
)

// makeTestSlim creates a schema_slim.json-like byte slice for testing.
func makeTestSlim(tables []string) []byte {
	type col struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type tbl struct {
		Name    string `json:"name"`
		Columns []col  `json:"columns"`
	}
	type slim struct {
		Tables []tbl `json:"tables"`
	}

	s := slim{}
	for _, t := range tables {
		s.Tables = append(s.Tables, tbl{
			Name: t,
			Columns: []col{
				{Name: "id", Type: "int"},
				{Name: "name", Type: "varchar"},
			},
		})
	}
	data, _ := json.Marshal(s)
	return data
}

// Test 4: RunResolve("find orders") returns ranked candidates with confidence
func TestRunResolve_Basic(t *testing.T) {
	slimData := makeTestSlim([]string{"orders", "customers", "products"})

	traceID := "test-trace"
	resp := runResolveWithSlim("find orders", slimData, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}
	if len(resp.Data.Tables) == 0 {
		t.Fatal("expected at least one table candidate, got none")
	}

	// Should find "orders" table with a confidence score
	foundOrders := false
	for _, tc := range resp.Data.Tables {
		if tc.Name == "orders" && tc.Score > 0 {
			foundOrders = true
			break
		}
	}
	if !foundOrders {
		t.Error("expected 'orders' as a table candidate with positive score")
	}

	if resp.Data.Confidence <= 0 {
		t.Error("expected positive overall confidence")
	}
}

// Test 5: RunResolve with no matches returns Success=true and NO_MATCHING_TABLES warning
func TestRunResolve_NoMatch(t *testing.T) {
	slimData := makeTestSlim([]string{"orders", "customers"})

	traceID := "test-trace"
	resp := runResolveWithSlim("xyzzy_nonexistent_table", slimData, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true for empty results, got error=%v", resp.Error)
	}

	// Empty tables are valid per D-07 — not an error
	if len(resp.Data.Tables) != 0 {
		t.Fatalf("expected 0 table candidates, got %d", len(resp.Data.Tables))
	}

	// Should have NO_MATCHING_TABLES warning
	hasWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "NO_MATCHING_TABLES" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected NO_MATCHING_TABLES warning for no matches")
	}
}

// Test 6: RunResolve returns error with invalid schema data
func TestRunResolve_InvalidSchema(t *testing.T) {
	// Invalid JSON data
	invalidData := []byte(`{invalid json`)

	traceID := "test-trace"
	resp := runResolveWithSlim("find orders", invalidData, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if resp.Success {
		t.Fatal("expected Success=false for invalid schema data")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
}

// Test: RunResolve validates data types in response
func TestRunResolve_Types(t *testing.T) {
	slimData := makeTestSlim([]string{"users"})

	traceID := "test-trace"
	resp := runResolveWithSlim("users", slimData, traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got error=%v", resp.Error)
	}

	// Verify response contains TableCandidate structs with required fields
	for _, tc := range resp.Data.Tables {
		if tc.Name == "" {
			t.Error("expected non-empty table name")
		}
		if tc.Score <= 0 || tc.Score > 1.0 {
			t.Errorf("expected score in (0,1], got %f", tc.Score)
		}
		if tc.MatchType == "" {
			t.Error("expected non-empty match_type")
		}
	}

	// Verify columns are queryutil.ColumnCandidate types
	if len(resp.Data.Columns) > 0 {
		cc := resp.Data.Columns[0]
		if cc.Name == "" {
			t.Error("expected non-empty column name")
		}
		if cc.Confidence <= 0 || cc.Confidence > 1.0 {
			t.Errorf("expected column confidence in (0,1], got %f", cc.Confidence)
		}
		_ = queryutil.ColumnCandidate{} // verify import works
	}
}
