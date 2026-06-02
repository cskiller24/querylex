package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/querylex/querylex/internal/analysis"
	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// ExplainData is the response data for the explain command.
type ExplainData struct {
	Plan       *db.ExplainPlan             `json:"execution_plan"`
	Heuristics []analysis.HeuristicSignal  `json:"heuristics"`
	Analyze    bool                        `json:"analyze"`
}

// RunExplain executes the explain command with a full preflight.
func RunExplain(query string, analyze bool) *format.Response[ExplainData] {
	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertExplainError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()
	return runExplainWithAdapter(preflight.Adapter, query, analyze, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runExplainWithAdapter executes the explain command with a provided adapter.
func runExplainWithAdapter(adapter db.Adapter, query string, analyze bool, traceID string, activeDBID *string) *format.Response[ExplainData] {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Explain(ctx, query, analyze)
	if err != nil {
		resp := format.NewErrorResponse[ExplainData](
			format.ErrCodeExplainFailed,
			fmt.Sprintf("Explain plan extraction failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Run heuristic analysis on the normalized plan
	signals := analysis.AnalyzeExplainPlan(result)
	if signals == nil {
		signals = []analysis.HeuristicSignal{}
	}

	// Build response data
	data := ExplainData{
		Plan:       result,
		Heuristics: signals,
		Analyze:    analyze,
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)

	// Add ANALYZE_CONFIRMATION warning if analyze mode
	if analyze {
		resp.Warnings = append(resp.Warnings, format.Warning{
			Code:    "ANALYZE_CONFIRMATION",
			Message: "The query will be executed for runtime plan analysis. This may impact database performance.",
		})
	}

	resp.Complete(start)
	return resp
}

// convertExplainError converts an any-typed error response to an ExplainData-typed one.
func convertExplainError(errResp *format.Response[any]) *format.Response[ExplainData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[ExplainData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[ExplainData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
