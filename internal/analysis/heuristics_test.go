package analysis

import (
	"testing"

	"github.com/querylex/querylex/internal/db"
)

// Helper to create a float64 pointer.
func f64(v float64) *float64 { return &v }

// Helper to create an int64 pointer.
func i64(v int64) *int64 { return &v }

// Test 1: FULL_TABLE_SCAN detection
func TestAnalyzeExplainPlan_FullTableScan(t *testing.T) {
	plan := &db.ExplainPlan{
		FullScanTables: []string{"users"},
	}

	signals := AnalyzeExplainPlan(plan)
	if len(signals) == 0 {
		t.Fatal("expected at least one heuristic signal")
	}

	found := false
	for _, s := range signals {
		if s.Code == "FULL_TABLE_SCAN" {
			found = true
			if s.Severity != "high" {
				t.Errorf("expected severity 'high', got '%s'", s.Severity)
			}
			if s.Detail == "" {
				t.Error("expected non-empty detail message")
			}
		}
	}
	if !found {
		t.Error("expected FULL_TABLE_SCAN signal")
	}
}

// Test 2: EXCESSIVE_SORTING detection
func TestAnalyzeExplainPlan_ExcessiveSorting(t *testing.T) {
	plan := &db.ExplainPlan{
		SortOperations: 3,
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "EXCESSIVE_SORTING" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected EXCESSIVE_SORTING signal")
	}
}

// Test 3: TEMPORARY_TABLE_USAGE detection
func TestAnalyzeExplainPlan_TempTableUsage(t *testing.T) {
	plan := &db.ExplainPlan{
		TempOperations: 1,
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "TEMPORARY_TABLE_USAGE" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected TEMPORARY_TABLE_USAGE signal")
	}
}

// Test 4: Clean plan returns empty slice
func TestAnalyzeExplainPlan_CleanPlan(t *testing.T) {
	plan := &db.ExplainPlan{
		FullScanTables: []string{},
		IndexUsage:     []db.IndexUsageEntry{},
		SortOperations: 0,
		TempOperations: 0,
		JoinOperations: []db.JoinOperationEntry{},
		Warnings:       []string{},
	}

	signals := AnalyzeExplainPlan(plan)
	if len(signals) != 0 {
		t.Errorf("expected empty signals for clean plan, got %d: %v", len(signals), signals)
	}
}

// Test 5: NON_SARGABLE_PREDICATE detection
func TestAnalyzeExplainPlan_NonSargable(t *testing.T) {
	plan := &db.ExplainPlan{
		IndexUsage: []db.IndexUsageEntry{
			{Table: "orders", Index: "idx_created_at", Covering: false, AccessType: "index_scan"},
			{Table: "orders", Index: "idx_created_at", Covering: true, AccessType: "index_seek"},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "NON_SARGABLE_PREDICATE" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected NON_SARGABLE_PREDICATE signal")
	}
}

// Test 6: MULTI_TABLE_JOIN detection
func TestAnalyzeExplainPlan_MultiTableJoin(t *testing.T) {
	plan := &db.ExplainPlan{
		JoinOperations: []db.JoinOperationEntry{
			{Type: "hash_join", Tables: []string{"orders", "customers"}},
			{Type: "nested_loop", Tables: []string{"payments", "orders"}},
			{Type: "hash_join", Tables: []string{"shipments", "orders"}},
			{Type: "nested_loop", Tables: []string{"products", "order_items"}},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "MULTI_TABLE_JOIN" {
			found = true
			if s.Severity != "low" {
				t.Errorf("expected severity 'low', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected MULTI_TABLE_JOIN signal")
	}
}

// Test 7: All 12 heuristic codes are testable
func TestAnalyzeExplainPlan_AllTwelveCodes(t *testing.T) {
	// Build a plan that should trigger all 12 codes
	plan := &db.ExplainPlan{
		EstimatedTotalCost:   f64(5000),
		FullScanTables:       []string{"users", "logs"},
		SortOperations:       3,
		TempOperations:       2,
		JoinOperations: []db.JoinOperationEntry{
			{Type: "hash_join", Tables: []string{"a", "b"}},
			{Type: "nested_loop", Tables: []string{"c", "d"}},
			{Type: "hash_join", Tables: []string{"e", "f"}},
			{Type: "CROSS JOIN", Tables: []string{"g", "h"}},
		},
		IndexUsage: []db.IndexUsageEntry{
			{Table: "users", Index: "idx_email", Covering: false, AccessType: "index_scan"},
			{Table: "users", Index: "idx_name", Covering: false, AccessType: "ALL"},
		},
		Warnings: []string{
			"type conversion on column created_at",
			"stale statistics for table orders",
		},
	}

	signals := AnalyzeExplainPlan(plan)

	codeSet := make(map[string]bool)
	for _, s := range signals {
		codeSet[s.Code] = true
	}

	expectedCodes := []string{
		"FULL_TABLE_SCAN",
		"NON_SARGABLE_PREDICATE",
		"MISSING_INDEX",
		"EXCESSIVE_SORTING",
		"TEMPORARY_TABLE_USAGE",
		"INDEX_NOT_USED",
		"IMPLICIT_TYPE_CONVERSION",
		"MULTI_TABLE_JOIN",
		"CARTESIAN_JOIN",
		"HIGH_COST_ESTIMATE",
		"STALE_STATISTICS",
		"SUBOPTIMAL_JOIN_ORDER",
	}

	for _, code := range expectedCodes {
		if !codeSet[code] {
			t.Errorf("expected heuristic code '%s' not produced", code)
		}
	}
}

// Test: MISSING_INDEX detection (table is full-scanned but no index usage for it)
func TestAnalyzeExplainPlan_MissingIndex(t *testing.T) {
	plan := &db.ExplainPlan{
		FullScanTables: []string{"orders"},
		IndexUsage:     []db.IndexUsageEntry{
			// Index usage is for a different table
			{Table: "users", Index: "idx_email", Covering: true, AccessType: "index_seek"},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "MISSING_INDEX" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected MISSING_INDEX signal")
	}
}

// Test: INDEX_NOT_USED detection
func TestAnalyzeExplainPlan_IndexNotUsed(t *testing.T) {
	plan := &db.ExplainPlan{
		IndexUsage: []db.IndexUsageEntry{
			{Table: "orders", Index: "idx_status", Covering: false, AccessType: "ALL"},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "INDEX_NOT_USED" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected INDEX_NOT_USED signal")
	}
}

// Test: IMPLICIT_TYPE_CONVERSION detection
func TestAnalyzeExplainPlan_ImplicitTypeConversion(t *testing.T) {
	plan := &db.ExplainPlan{
		Warnings: []string{"type conversion on column id"},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "IMPLICIT_TYPE_CONVERSION" {
			found = true
			if s.Severity != "low" {
				t.Errorf("expected severity 'low', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected IMPLICIT_TYPE_CONVERSION signal")
	}
}

// Test: CARTESIAN_JOIN detection
func TestAnalyzeExplainPlan_CartesianJoin(t *testing.T) {
	plan := &db.ExplainPlan{
		JoinOperations: []db.JoinOperationEntry{
			{Type: "CROSS JOIN", Tables: []string{"orders", "customers"}},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "CARTESIAN_JOIN" {
			found = true
			if s.Severity != "high" {
				t.Errorf("expected severity 'high', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected CARTESIAN_JOIN signal")
	}
}

// Test: HIGH_COST_ESTIMATE detection
func TestAnalyzeExplainPlan_HighCostEstimate(t *testing.T) {
	plan := &db.ExplainPlan{
		EstimatedTotalCost: f64(5000),
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "HIGH_COST_ESTIMATE" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected HIGH_COST_ESTIMATE signal")
	}
}

// Test: STALE_STATISTICS detection
func TestAnalyzeExplainPlan_StaleStatistics(t *testing.T) {
	plan := &db.ExplainPlan{
		Warnings: []string{"stale statistics for table orders"},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "STALE_STATISTICS" {
			found = true
			if s.Severity != "low" {
				t.Errorf("expected severity 'low', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected STALE_STATISTICS signal")
	}
}

// Test: SUBOPTIMAL_JOIN_ORDER detection
func TestAnalyzeExplainPlan_SuboptimalJoinOrder(t *testing.T) {
	plan := &db.ExplainPlan{
		EstimatedTotalCost: f64(5000),
		JoinOperations: []db.JoinOperationEntry{
			{Type: "nested_loop", Tables: []string{"orders", "customers"}},
			{Type: "nested_loop", Tables: []string{"payments", "orders"}},
		},
	}

	signals := AnalyzeExplainPlan(plan)

	found := false
	for _, s := range signals {
		if s.Code == "SUBOPTIMAL_JOIN_ORDER" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected severity 'medium', got '%s'", s.Severity)
			}
		}
	}
	if !found {
		t.Error("expected SUBOPTIMAL_JOIN_ORDER signal")
	}
}
