package cli

import (
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// ValidateData is the response data for the validate command.
type ValidateData struct {
	Valid         bool     `json:"valid"`
	NormalizedSQL string   `json:"normalized_sql,omitempty"`
	StatementType string   `json:"statement_type,omitempty"`
	ReadOnly      bool     `json:"read_only"`
	Tables        []string `json:"tables,omitempty"`
	Columns       []string `json:"columns,omitempty"`
}

// RunValidate executes the validate command with a full preflight.
// STUB: replaced with real implementation in GREEN phase.
func RunValidate(query string) *format.Response[ValidateData] {
	traceID := format.GenerateTraceID()
	return format.NewErrorResponse[ValidateData](
		format.ErrCodeInvalidSQL,
		"Validate not yet implemented",
		false,
		traceID,
	)
}

// runValidateWithAdapter executes the validate command with a provided adapter.
// STUB: replaced with real implementation in GREEN phase.
func runValidateWithAdapter(adapter db.Adapter, query string, traceID string, activeDBID *string) *format.Response[ValidateData] {
	start := time.Now()
	resp := format.NewErrorResponse[ValidateData](
		format.ErrCodeInvalidSQL,
		"Validate not yet implemented",
		false,
		traceID,
	)
	resp.Complete(start)
	return resp
}

// convertValidateError converts an any-typed error response to a ValidateData-typed one.
func convertValidateError(errResp *format.Response[any]) *format.Response[ValidateData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[ValidateData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[ValidateData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
