package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cskiller24/querylex/internal/ai"
	"github.com/cskiller24/querylex/internal/index"
	"golang.org/x/sync/errgroup"
)

func RunSQLGeneration(ctx context.Context, question string) error {
	// Step 1: Preflight
	preflight, errResp := PreflightForAICommand()
	if errResp != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", errResp.Error.Message)
		return errors.New(errResp.Error.Message)
	}

	fmt.Printf("Using active database: %s\n", preflight.ActiveDBID)

	// Step 2: Resolve
	resolveResp := RunResolve(question)
	if !resolveResp.Success {
		fmt.Fprintf(os.Stderr, "Error: resolve failed: %s\n", resolveResp.Error.Message)
		return errors.New(resolveResp.Error.Message)
	}
	if len(resolveResp.Data.Tables) == 0 {
		fmt.Println("No relevant tables found for your question. Try providing more context, such as specific table names, column names, or business terms.")
		return nil
	}

	resolvedTables := make([]string, len(resolveResp.Data.Tables))
	for i, t := range resolveResp.Data.Tables {
		resolvedTables[i] = t.Name
	}

	// Step 3: Memory check
	memoryResp := RunMemory(question)
	if memoryResp.Success && memoryResp.Data.MatchFound && memoryResp.Data.Entry != nil {
		fmt.Printf("Found a saved query matching your question (similarity: %.2f):\n%s\nUse this cached result? [y/N] ", func() float64 {
			if memoryResp.Data.Similarity != nil {
				return *memoryResp.Data.Similarity
			}
			return 0
		}(), memoryResp.Data.Entry.SQL)

		var answer string
		fmt.Scanln(&answer)
		if strings.HasPrefix(strings.ToLower(answer), "y") {
			fmt.Println("Using cached result.")
			return nil
		}
	}

	// Step 4: Terminology
	var terminology string
	termContent, err := os.ReadFile(filepath.Join(preflight.DBDir, "terminologies.md"))
	if err == nil {
		terms, parseErr := index.ParseTerminology(termContent)
		if parseErr == nil && terms != nil {
			terminology = formatTerms(terms)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: continuing without business terms.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "Warning: continuing without business terms.\n")
	}

	// Steps 5-8: Parallel context fetching
	type contextBundle struct {
		schemaData string
		statsData  string
		joinsData  string
		indexesData string
		err        error
	}

	var bundle contextBundle
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		resp := RunSchema(resolvedTables)
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Warning: schema fetch: %s\n", resp.Error.Message)
			return nil
		}
		bundle.schemaData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunStatsTables(resolvedTables)
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Warning: stats fetch: %s\n", resp.Error.Message)
			return nil
		}
		bundle.statsData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunJoins(resolvedTables)
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Warning: joins fetch: %s\n", resp.Error.Message)
			return nil
		}
		if len(resp.Data.Joins) == 0 && len(resolvedTables) > 1 {
			_ = gCtx // suppress unused
			return nil
		}
		bundle.joinsData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	g.Go(func() error {
		resp := RunIndexes(resolvedTables, false)
		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Warning: indexes fetch: %s\n", resp.Error.Message)
			return nil
		}
		bundle.indexesData = fmt.Sprintf("%+v", resp.Data)
		return nil
	})

	_ = g.Wait()

	// Step 9: Assemble context
	ws := preflight.Workspace
	dbDialect := "mysql"
	for _, entry := range ws.ConnectedDatabases {
		if entry.ID == preflight.ActiveDBID {
			dbDialect = entry.Type
			break
		}
	}

	sqlCtx := &ai.SQLGenerationContext{
		Question:       question,
		Dialect:        dbDialect,
		ResolveOutput:  formatResolveOutput(resolveResp.Data),
		SchemaSlimJSON: bundle.schemaData,
		Terminology:    terminology,
		JoinGraphJSON:  bundle.joinsData,
		IndexInfo:      bundle.indexesData,
		StatsInfo:      bundle.statsData,
	}

	// Step 10: AI SQL construction
	tokenBudget := ai.NewTokenBudget(preflight.AIConfig.MaxTokens)
	systemPrompt, userPrompt, warnings := ai.BuildSQLGenerationPrompt(ctx, sqlCtx, tokenBudget)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	client := ai.NewClient(preflight.AIConfig)
	resp, err := ai.ChatCompletion(ctx, client, preflight.AIConfig.Model, systemPrompt, userPrompt, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: AI service unavailable: %v\n", err)
		return fmt.Errorf("AI_SERVICE_UNAVAILABLE: %w", err)
	}

	generatedSQL := strings.TrimSpace(resp.Choices[0].Message.Content)
	generatedSQL = stripMarkdownCodeFences(generatedSQL)

	// Step 11: Validate loop
	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		validateResp := RunValidate(generatedSQL)
		if validateResp.Success && validateResp.Data.Valid {
			break
		}

		if attempt >= maxAttempts {
			msg := fmt.Sprintf("Validation failed after %d attempts.", maxAttempts)
			if validateResp.Error != nil {
				msg = fmt.Sprintf("%s Last error: %s", msg, validateResp.Error.Message)
			}
			fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
			return errors.New(msg)
		}

		errorMsg := "validation failed"
		if validateResp.Error != nil {
			errorMsg = validateResp.Error.Message
		}

		revisedPrompt := fmt.Sprintf(
			"The following SQL failed validation:\n\n```sql\n%s\n```\n\nValidation error: %s\n\nPlease fix the SQL and return only the corrected SQL.",
			generatedSQL, errorMsg,
		)

		newResp, err := ai.ChatCompletion(ctx, client, preflight.AIConfig.Model, systemPrompt, revisedPrompt, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: AI retry failed: %v\n", err)
			return fmt.Errorf("AI_SERVICE_UNAVAILABLE: %w", err)
		}
		generatedSQL = strings.TrimSpace(newResp.Choices[0].Message.Content)
		generatedSQL = stripMarkdownCodeFences(generatedSQL)
	}

	// Step 12: Display and save
	fmt.Printf("\nGenerated SQL:\n%s\n", generatedSQL)

	fmt.Print("Save this query to memory? [y/N] ")
	var saveAnswer string
	fmt.Scanln(&saveAnswer)
	if strings.HasPrefix(strings.ToLower(saveAnswer), "y") {
		saveResp := RunSave(question, generatedSQL)
		if !saveResp.Success {
			fmt.Fprintf(os.Stderr, "Warning: SQL generated but could not be saved to memory.\n")
		} else if saveResp.Data.Saved {
			if saveResp.Data.Entry != nil {
				fmt.Printf("Saved to memory (ID: %s)\n", saveResp.Data.Entry.ID)
			} else {
				fmt.Println("Saved to memory.")
			}
		}
	}

	return nil
}

func stripMarkdownCodeFences(s string) string {
	s = strings.TrimPrefix(s, "```sql")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func formatResolveOutput(data interface{}) string {
	return fmt.Sprintf("%+v", data)
}

func formatTerms(terms *index.TerminologyDoc) string {
	if terms == nil {
		return ""
	}
	return fmt.Sprintf("%+v", terms.Terms)
}
