package ai

import (
	"context"
	"fmt"
	"strings"
)

type SQLGenerationContext struct {
	Question       string
	Dialect        string
	ResolveOutput  string
	SchemaSlimJSON string
	Terminology    string
	JoinGraphJSON  string
	IndexInfo      string
	StatsInfo      string
	HistoryRefs    string
}

type PriorAttempt struct {
	Strategy     string
	SQL          string
	ValidationResult string
}

func BuildSQLGenerationPrompt(ctx context.Context, context *SQLGenerationContext, budget *TokenBudget) (system string, user string, warnings []string) {
	system = fmt.Sprintf("You are a SQL query generator for a %s database. Generate only the SQL query, no explanation. Use %s-specific syntax.", context.Dialect, context.Dialect)

	var parts []string

	tiers := []struct {
		name    string
		content string
	}{
		{"Schema", context.SchemaSlimJSON},
		{"Resolve Output", context.ResolveOutput},
		{"Terminology", context.Terminology},
		{"Join Graph", context.JoinGraphJSON},
		{"Index Info", context.IndexInfo},
		{"Table Stats", context.StatsInfo},
		{"History Refs", context.HistoryRefs},
	}

	for _, tier := range tiers {
		if tier.content == "" {
			continue
		}
		added := budget.AddTier(tier.name, tier.content)
		if added != "" {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", tier.name, added))
		}
	}

	user = fmt.Sprintf("%s\n\nUser question: %s", strings.Join(parts, "\n\n"), context.Question)
	return system, user, budget.Warnings
}

func BuildOptimizationPrompt(ctx context.Context, originalSQL string, explainPlan string, schemaContext string, joinsContext string, statsContext string, indexesContext string, dialect string, priorAttempts []PriorAttempt, budget *TokenBudget) (system string, user string) {
	system = fmt.Sprintf("You are an SQL query optimizer for %s. Analyze the query plan and suggest optimized SQL.", dialect)

	var parts []string

	sections := []struct {
		name    string
		content string
	}{
		{"Original SQL", fmt.Sprintf("```sql\n%s\n```", originalSQL)},
		{"Explain Plan", explainPlan},
		{"Schema Context", schemaContext},
		{"Joins Context", joinsContext},
		{"Statistics", statsContext},
		{"Indexes", indexesContext},
	}

	for _, s := range sections {
		if s.content == "" {
			continue
		}
		added := budget.AddTier(s.name, s.content)
		if added != "" {
			parts = append(parts, fmt.Sprintf("### %s\n\n%s", s.name, added))
		}
	}

	if len(priorAttempts) > 0 {
		var summaries []string
		for _, pa := range priorAttempts {
			summaries = append(summaries, fmt.Sprintf("- Strategy: %s\n  SQL: ```sql\n  %s\n  ```\n  Result: %s", pa.Strategy, pa.SQL, pa.ValidationResult))
		}
		added := budget.AddTier("Prior Attempts", strings.Join(summaries, "\n"))
		if added != "" {
			parts = append(parts, fmt.Sprintf("### Prior Attempts\n\n%s", added))
		}
	}

	user = strings.Join(parts, "\n\n")
	return system, user
}
