package cli

import (
	"time"

	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/queryutil"
)

// ResolveData is the response data for the resolve command.
type ResolveData struct {
	Tables     []queryutil.TableCandidate  `json:"tables"`
	Columns    []queryutil.ColumnCandidate `json:"columns"`
	Confidence float64                     `json:"confidence"`
}

// RunResolve executes the resolve command, loading schema_slim.json from disk.
// No database connection needed. Pure computation from local artifacts.
// STUB: replaced in GREEN phase.
func RunResolve(question string) *format.Response[ResolveData] {
	traceID := format.GenerateTraceID()
	return format.NewErrorResponse[ResolveData](
		format.ErrCodeSchemaParseError,
		"Resolve not yet implemented",
		false,
		traceID,
	)
}

// runResolveWithSlim executes resolve with provided slim schema data (for testing).
// STUB: replaced in GREEN phase.
func runResolveWithSlim(question string, slimData []byte, traceID string, activeDBID *string) *format.Response[ResolveData] {
	start := time.Now()
	_ = question
	_ = slimData
	resp := format.NewErrorResponse[ResolveData](
		format.ErrCodeSchemaParseError,
		"Resolve not yet implemented",
		false,
		traceID,
	)
	resp.Complete(start)
	return resp
}
