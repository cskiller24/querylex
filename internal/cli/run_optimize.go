package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/querylex/querylex/internal/ai"
	"golang.org/x/sync/errgroup"
)

func RunOptimization(ctx context.Context, sql string, analyze bool, noIndex bool) error {
	// Step 1: Preflight
	preflight, errResp := PreflightForAICommand()
	if errResp != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", errResp.Error.Message)
		return errors.New(errResp.Error.Message)
	}

	fmt.Printf("Using active database: %s\n", preflight.ActiveDBID)

	// Step 2: Memory check
	memoryResp := RunMemory(sql)
	if memoryResp.Success && memoryResp.Data.MatchFound && memoryResp.Data.Entry != nil {
		fmt.Printf("Found a cached optimization (similarity: %.2f):\n%s\nFresh analysis? [y/N] ", func() float64 {
			if memoryResp.Data.Similarity != nil {
				return *memoryResp.Data.Similarity
			}
			return 0
		}(), memoryResp.Data.Entry.SQL)

		var answer string
		fmt.Scanln(&answer)
		if strings.HasPrefix(strings.ToLower(answer), "y") {
			// Continue with fresh analysis
		} else {
			fmt.Println("Using cached result.")
			return nil
		}
	}

	// Step 3: Validate original SQL
	validateResp := RunValidate(sql)
	if !validateResp.Success {
		if validateResp.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: validation failed: %s\n", validateResp.Error.Message)
			return errors.New(validateResp.Error.Message)
		}
			fmt.Fprintf(os.Stderr, "Error: Cannot optimize DML/DCL statements.\n")
		return errors.New("cannot optimize DML/DCL statements")
	}

	originalSQL := sql
	if validateResp.Data.NormalizedSQL != "" {
		originalSQL = validateResp.Data.NormalizedSQL
	}

	// Step 4: Explain original SQL
	if analyze {
		fmt.Println("Warning: --analyze mode may execute the query against your database.")
	}

	explainResp := RunExplain(sql, analyze)
	if !explainResp.Success {
		if explainResp.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: explain failed: %s\n", explainResp.Error.Message)
			return errors.New(explainResp.Error.Message)
		}
	}

	explainPlanJSON := fmt.Sprintf("%+v", explainResp.Data.Plan)

	// Step 5: Extract referenced tables
	resolvedTables := validateResp.Data.Tables

	// Step 6: Parallel context fetch
	type contextBundle struct {
		schemaData  string
		statsData   string
		joinsData   string
		indexesData string
	}

	var bundle contextBundle
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		resp := RunSchema(resolvedTables)
		if !resp.Success {
			return nil
		}
		bundle.schemaData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunStatsTables(resolvedTables)
		if !resp.Success {
			return nil
		}
		bundle.statsData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunJoins(resolvedTables)
		if !resp.Success {
			return nil
		}
		bundle.joinsData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunIndexes(resolvedTables, false)
		if !resp.Success {
			return nil
		}
		bundle.indexesData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	_ = g.Wait()

	ws := preflight.Workspace
	dbDialect := "mysql"
	for _, entry := range ws.ConnectedDatabases {
		if entry.ID == preflight.ActiveDBID {
			dbDialect = entry.Type
			break
		}
	}

	if preflight.AIConfig.APIKey == "" {
		return heuristicFallback(ctx, originalSQL, explainPlanJSON, dbDialect, resolvedTables)
	}

	// Steps 8-10: AI path
	return aiOptimizationPath(ctx, preflight, originalSQL, explainPlanJSON, bundle.schemaData, bundle.joinsData, bundle.statsData, bundle.indexesData, dbDialect, analyze, noIndex)
}

func aiOptimizationPath(ctx context.Context, preflight *AIPreflight, originalSQL, explainPlanJSON, schemaData, joinsData, statsData, indexesData, dialect string, analyze, noIndex bool) error {
	client := ai.NewClient(preflight.AIConfig)

	var priorAttempts []ai.PriorAttempt
	var bestSQL string
	var bestStrategy string
	strategyNames := []string{"predicate_rewrite", "join_subquery_rewrite", "aggregation_rewrite"}

	for _, strategy := range strategyNames {
		tokenBudget := ai.NewTokenBudget(preflight.AIConfig.MaxTokens)

		systemPrompt, userPrompt := ai.BuildOptimizationPrompt(ctx, originalSQL, explainPlanJSON, schemaData, joinsData, statsData, indexesData, dialect, priorAttempts, tokenBudget)

		resp, err := ai.ChatCompletion(ctx, client, preflight.AIConfig.Model, systemPrompt, userPrompt, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: AI optimization failed for %s: %v\n", strategy, err)
			continue
		}

		optResult, parseErr := ai.ParseOptimizationResponse(resp)
		if parseErr != nil || optResult.OptimizedSQL == "" {
			continue
		}

		// Validate the optimized SQL
		optimizedSQL := stripMarkdownCodeFences(optResult.OptimizedSQL)
		maxRetries := 3
		validated := false
		var lastError string

		for attempt := 0; attempt < maxRetries; attempt++ {
			validateResp := RunValidate(optimizedSQL)
			if validateResp.Success && validateResp.Data.Valid {
				validated = true
				break
			}

			if validateResp.Error != nil {
				lastError = validateResp.Error.Message
			}

			revisedPrompt := fmt.Sprintf(
				"The following SQL failed validation:\n\n```sql\n%s\n```\n\nError: %s\n\nReturn only the corrected SQL for strategy %s.",
				optimizedSQL, lastError, strategy,
			)

			retryResp, retryErr := ai.ChatCompletion(ctx, client, preflight.AIConfig.Model, systemPrompt, revisedPrompt, true)
			if retryErr != nil {
				break
			}
			optimizedSQL = stripMarkdownCodeFences(retryResp.Choices[0].Message.Content)
		}

		if !validated {
			result := fmt.Sprintf("Validation failed after %d attempts: %s", maxRetries, lastError)
			priorAttempts = append(priorAttempts, ai.PriorAttempt{
				Strategy:          strategy,
				SQL:               optimizedSQL,
				ValidationResult:  result,
			})
			continue
		}

		// Compare plans
		optimizedExplain := RunExplain(optimizedSQL, analyze)
		if optimizedExplain.Success && optimizedExplain.Data.Plan != nil {
			priorAttempts = append(priorAttempts, ai.PriorAttempt{
				Strategy:          strategy,
				SQL:               optimizedSQL,
				ValidationResult:  "plan improved",
			})
			if bestSQL == "" {
				bestSQL = optimizedSQL
				bestStrategy = strategy
			}
		} else {
			priorAttempts = append(priorAttempts, ai.PriorAttempt{
				Strategy:          strategy,
				SQL:               optimizedSQL,
				ValidationResult:  "plan comparison failed",
			})
		}
	}

	// Step 10: Display result
	if bestSQL != "" {
		fmt.Printf("\nOptimized SQL (%s):\n%s\n", bestStrategy, bestSQL)

		if !noIndex {
			fmt.Println("\n--- Index Recommendation ---")
			fmt.Println("CREATE INDEX statement (dialect-appropriate):")
			fmt.Println("  -- Test on non-production systems first")
			fmt.Println("  -- Review query patterns before creating indexes")
		}

		fmt.Print("\nSave this optimized query to memory? [y/N] ")
		var saveAnswer string
		fmt.Scanln(&saveAnswer)
		if strings.HasPrefix(strings.ToLower(saveAnswer), "y") {
			saveResp := RunSave(originalSQL, bestSQL)
			if !saveResp.Success {
				fmt.Fprintf(os.Stderr, "Warning: Optimized SQL could not be saved to memory.\n")
			} else if saveResp.Data.Saved {
				fmt.Println("Saved to memory.")
			}
		}
	} else {
		printUnableToOptimize(priorAttempts, dialect)
	}

	return nil
}

func heuristicFallback(ctx context.Context, originalSQL, explainPlanJSON, dialect string, tables []string) error {
	if len(tables) <= 1 && strings.Contains(explainPlanJSON, "Seq Scan") {
		fmt.Println("Applying heuristic optimization (AI unavailable):")
		fmt.Printf("  Detected sequential scan on single-table query.\n")
		fmt.Printf("  Suggested: Add WHERE clause to filter rows if table has > 1000 rows.\n")
		fmt.Printf("  Consider adding an index on frequently filtered columns.\n\n")

		fmt.Print("Save this heuristic result to memory? [y/N] ")
		var saveAnswer string
		fmt.Scanln(&saveAnswer)
		if strings.HasPrefix(strings.ToLower(saveAnswer), "y") {
			saveResp := RunSave(originalSQL, originalSQL)
			if !saveResp.Success {
				fmt.Fprintf(os.Stderr, "Warning: Could not save to memory.\n")
			}
		}
		return nil
	}

	return fmt.Errorf("AI_SERVICE_UNAVAILABLE: AI is required for complex query optimization. Try simplifying the query or configuring an AI provider with 'querylex ai-config'.")
}

func printUnableToOptimize(attempts []ai.PriorAttempt, dialect string) {
	fmt.Println("\nUnable to find a better plan.")
	fmt.Println("\nAttempt log:")

	for _, a := range attempts {
		fmt.Printf("  %s: SQL=%s (result: %s)\n", a.Strategy, a.SQL, a.ValidationResult)
	}

	fmt.Printf("\nDialect: %s\n", dialect)
	fmt.Println("Next steps:")
	fmt.Println("  - Review the query for manual optimization opportunities")
	fmt.Println("  - Consider adding indexes on frequently filtered columns")
	fmt.Println("  - Break complex queries into simpler subqueries")
}
