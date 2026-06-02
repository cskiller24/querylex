package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/queryutil"
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
func RunValidate(query string) *format.Response[ValidateData] {
	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertValidateError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()
	return runValidateWithAdapter(preflight.Adapter, query, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runValidateWithAdapter executes the validate command with a provided adapter.
func runValidateWithAdapter(adapter db.Adapter, query string, traceID string, activeDBID *string) *format.Response[ValidateData] {
	start := time.Now()

	// Layer 1: DML/DCL keyword scan (client-side, no DB connection needed)
	if err := queryutil.ValidateSQLSafety(query); err != nil {
		resp := format.NewErrorResponse[ValidateData](
			format.ErrCodeUnsafeSQL,
			fmt.Sprintf("DML/DCL statements are not permitted. Only read-only queries allowed."),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Layer 2: Database-backed validation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := adapter.Validate(ctx, query)
	if err != nil {
		resp := format.NewErrorResponse[ValidateData](
			format.ErrCodeInvalidSQL,
			fmt.Sprintf("SQL validation failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	if !result.Valid {
		errMsg := "SQL validation failed"
		if len(result.Errors) > 0 {
			errMsg = result.Errors[0]
		}
		resp := format.NewErrorResponse[ValidateData](
			format.ErrCodeInvalidSQL,
			errMsg,
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Build success response
	data := ValidateData{
		Valid:         result.Valid,
		NormalizedSQL: result.NormalizedSQL,
		StatementType: result.StatementType,
		ReadOnly:      result.ReadOnly,
		Tables:        result.Tables,
		Columns:       result.Columns,
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)
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
