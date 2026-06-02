package analysis

import (
	"fmt"
	"strings"

	"github.com/cskiller24/querylex/internal/db"
)

// HeuristicSignal describes a single optimization signal detected in an ExplainPlan.
type HeuristicSignal struct {
	Code     string `json:"code"`     // e.g., "FULL_TABLE_SCAN"
	Severity string `json:"severity"` // "high", "medium", "low"
	Detail   string `json:"detail"`   // human-readable explanation
}

// AnalyzeExplainPlan examines a normalized ExplainPlan and returns all
// heuristic signals detected. Applies the ANALYSIS.md heuristic rules.
// Returns empty slice if no signals detected.
func AnalyzeExplainPlan(plan *db.ExplainPlan) []HeuristicSignal {
	var signals []HeuristicSignal

	// 1. FULL_TABLE_SCAN (high): Any table in FullScanTables
	for _, table := range plan.FullScanTables {
		signals = append(signals, HeuristicSignal{
			Code:     "FULL_TABLE_SCAN",
			Severity: "high",
			Detail:   fmt.Sprintf("Table '%s' is full-scanned. Consider adding an index.", table),
		})
	}

	// 2. NON_SARGABLE_PREDICATE (medium): IndexUsage entries where AccessType
	//    is not "index_seek" or "index_only" — index touched but can't seek
	for _, usage := range plan.IndexUsage {
		if usage.AccessType != "index_seek" && usage.AccessType != "index_only" {
			signals = append(signals, HeuristicSignal{
				Code:     "NON_SARGABLE_PREDICATE",
				Severity: "medium",
				Detail:   fmt.Sprintf("Non-sargable predicate on table '%s'. Index '%s' touched but cannot seek.", usage.Table, usage.Index),
			})
		}
	}

	// 3. MISSING_INDEX (medium): FullScanTables non-empty + no corresponding index usage
	if len(plan.FullScanTables) > 0 {
		indexedTables := make(map[string]bool)
		for _, usage := range plan.IndexUsage {
			indexedTables[usage.Table] = true
		}
		for _, table := range plan.FullScanTables {
			if !indexedTables[table] {
				signals = append(signals, HeuristicSignal{
					Code:     "MISSING_INDEX",
					Severity: "medium",
					Detail:   fmt.Sprintf("Table '%s' is full-scanned with no index usage. Consider adding an index.", table),
				})
			}
		}
	}

	// 4. EXCESSIVE_SORTING (medium): SortOperations > 2
	if plan.SortOperations > 2 {
		signals = append(signals, HeuristicSignal{
			Code:     "EXCESSIVE_SORTING",
			Severity: "medium",
			Detail:   fmt.Sprintf("Query uses %d sort operations. Consider indexing ORDER BY columns.", plan.SortOperations),
		})
	}

	// 5. TEMPORARY_TABLE_USAGE (medium): TempOperations > 0
	if plan.TempOperations > 0 {
		signals = append(signals, HeuristicSignal{
			Code:     "TEMPORARY_TABLE_USAGE",
			Severity: "medium",
			Detail:   fmt.Sprintf("Query creates %d temporary tables. Check GROUP BY / DISTINCT on indexed columns.", plan.TempOperations),
		})
	}

	// 6. INDEX_NOT_USED (medium): IndexUsage where Covering=false and access suggests non-use
	for _, usage := range plan.IndexUsage {
		if !usage.Covering && (usage.AccessType == "ALL" || usage.AccessType == "index" || usage.AccessType == "index_scan") {
			signals = append(signals, HeuristicSignal{
				Code:     "INDEX_NOT_USED",
				Severity: "medium",
				Detail:   fmt.Sprintf("Index '%s' on table '%s' exists but is not used (access type: %s).", usage.Index, usage.Table, usage.AccessType),
			})
		}
	}

	// 7. IMPLICIT_TYPE_CONVERSION (low): Warnings matching type conversion patterns
	for _, warn := range plan.Warnings {
		warnLower := strings.ToLower(warn)
		if strings.Contains(warnLower, "type conversion") || strings.Contains(warnLower, "implicit cast") {
			signals = append(signals, HeuristicSignal{
				Code:     "IMPLICIT_TYPE_CONVERSION",
				Severity: "low",
				Detail:   fmt.Sprintf("Implicit type conversion detected: %s", warn),
			})
		}
	}

	// 8. MULTI_TABLE_JOIN (low): len(JoinOperations) > 3
	if len(plan.JoinOperations) > 3 {
		signals = append(signals, HeuristicSignal{
			Code:     "MULTI_TABLE_JOIN",
			Severity: "low",
			Detail:   fmt.Sprintf("Query joins %d tables. Consider limiting join count for better performance.", len(plan.JoinOperations)+1),
		})
	}

	// 9. CARTESIAN_JOIN (high): JoinOperations where Type contains CROSS JOIN or NESTED LOOP
	for _, join := range plan.JoinOperations {
		joinType := strings.ToUpper(join.Type)
		if strings.Contains(joinType, "CROSS JOIN") || strings.Contains(joinType, "CROSS") {
			tables := strings.Join(join.Tables, ", ")
			signals = append(signals, HeuristicSignal{
				Code:     "CARTESIAN_JOIN",
				Severity: "high",
				Detail:   fmt.Sprintf("Cartesian join detected between tables: %s. Add join predicates.", tables),
			})
		}
		// Nested loop without index condition also considered high-risk
		if strings.Contains(joinType, "NESTED LOOP") || strings.Contains(joinType, "NESTED_LOOP") {
			if plan.EstimatedTotalCost != nil && *plan.EstimatedTotalCost > 1000 {
				signals = append(signals, HeuristicSignal{
					Code:     "CARTESIAN_JOIN",
					Severity: "high",
					Detail:   fmt.Sprintf("Nested loop join with high estimated cost between tables: %s.", strings.Join(join.Tables, ", ")),
				})
			}
		}
	}

	// 10. HIGH_COST_ESTIMATE (medium): EstimatedTotalCost > 1000
	if plan.EstimatedTotalCost != nil && *plan.EstimatedTotalCost > 1000 {
		signals = append(signals, HeuristicSignal{
			Code:     "HIGH_COST_ESTIMATE",
			Severity: "medium",
			Detail:   fmt.Sprintf("Query has high estimated cost (%.2f). Consider optimization.", *plan.EstimatedTotalCost),
		})
	}

	// 11. STALE_STATISTICS (low): Warnings containing staleness indicators
	for _, warn := range plan.Warnings {
		warnLower := strings.ToLower(warn)
		if strings.Contains(warnLower, "stale") || strings.Contains(warnLower, "outdated") {
			signals = append(signals, HeuristicSignal{
				Code:     "STALE_STATISTICS",
				Severity: "low",
				Detail:   fmt.Sprintf("Stale statistics detected: %s", warn),
			})
		}
	}

	// 12. SUBOPTIMAL_JOIN_ORDER (medium): Nested loop joins with high cost
	if len(plan.JoinOperations) > 0 && plan.EstimatedTotalCost != nil && *plan.EstimatedTotalCost > 1000 {
		for _, join := range plan.JoinOperations {
			joinType := strings.ToUpper(join.Type)
			if strings.Contains(joinType, "NESTED LOOP") || strings.Contains(joinType, "NESTED_LOOP") {
				signals = append(signals, HeuristicSignal{
					Code:     "SUBOPTIMAL_JOIN_ORDER",
					Severity: "medium",
					Detail:   fmt.Sprintf("Suboptimal join order detected. Nested loop join on tables %s with high cost.", strings.Join(join.Tables, ", ")),
				})
				break // One signal is enough
			}
		}
	}

	return signals
}
