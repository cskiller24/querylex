package cli

import (
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
// STUB: replaced with real implementation in GREEN phase.
func RunExplain(query string, analyze bool) *format.Response[ExplainData] {
	traceID := format.GenerateTraceID()
	return format.NewErrorResponse[ExplainData](
		format.ErrCodeExplainFailed,
		"Explain not yet implemented",
		false,
		traceID,
	)
}

// runExplainWithAdapter executes the explain command with a provided adapter.
// STUB: replaced with real implementation in GREEN phase.
func runExplainWithAdapter(adapter db.Adapter, query string, analyze bool, traceID string, activeDBID *string) *format.Response[ExplainData] {
	start := time.Now()
	resp := format.NewErrorResponse[ExplainData](
		format.ErrCodeExplainFailed,
		"Explain not yet implemented",
		false,
		traceID,
	)
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
