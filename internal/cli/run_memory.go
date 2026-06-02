package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/memory"
)

// MemoryData is the response data for the memory command.
type MemoryData struct {
	MatchFound bool            `json:"match_found"`
	Similarity *float64        `json:"similarity"`
	MatchType  *string         `json:"match_type"`
	Entry      *MemoryEntryData `json:"entry"`
}

// MemoryEntryData contains the entry details returned by the memory command.
type MemoryEntryData struct {
	ID                  string `json:"id"`
	Input               string `json:"input"`
	SQL                 string `json:"sql,omitempty"`
	OptimizationSummary string `json:"optimization_summary,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	DatabaseID          string `json:"database_id,omitempty"`
}

// RunMemory executes the querylex memory command.
// It searches memory for strong matches (similarity >= 0.86) and returns the
// top result, or match_found: false if no strong match exists.
func RunMemory(input string) *format.Response[MemoryData] {
	start := time.Now()
	traceID := format.GenerateTraceID()

	preflight, errResp := PreflightForMemoryCommand()
	if errResp != nil {
		resp := convertMemoryError(errResp)
		resp.Complete(start)
		return resp
	}

	// Search memory
	results, warning, err := memory.Search(preflight.DBDir, input, 20)
	if err != nil {
		errMsg := err.Error()
		code := format.ErrCodeMemoryStoreUnavailable
		if strings.Contains(errMsg, "MEMORY_STORE_UNAVAILABLE") {
			code = format.ErrCodeMemoryStoreUnavailable
		}
		resp := format.NewErrorResponse[MemoryData](
			code,
			fmt.Sprintf("Memory subsystem is unavailable: %v", err),
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Determine result
	var data MemoryData

	if len(results) > 0 && results[0].Similarity >= 0.86 {
		top := results[0]
		sim := top.Similarity
		matchType := top.Entry.MatchType
		data = MemoryData{
			MatchFound: true,
			Similarity: &sim,
			MatchType:  &matchType,
			Entry: &MemoryEntryData{
				ID:                  top.Entry.ID,
				Input:               top.Entry.Input,
				SQL:                 top.Entry.SQL,
				OptimizationSummary: top.Entry.OptimizationSummary,
				CreatedAt:           top.Entry.CreatedAt,
				UpdatedAt:           top.Entry.UpdatedAt,
				DatabaseID:          top.Entry.DatabaseID,
			},
		}
	} else {
		data = MemoryData{
			MatchFound: false,
			Similarity: nil,
			MatchType:  nil,
			Entry:      nil,
		}
	}

	resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
	resp.Warnings = addEmbeddingsWarning()
	if warning != nil {
		resp.Warnings = append(resp.Warnings, *warning)
	}
	resp.Complete(start)
	return resp
}

// convertMemoryError converts an any-typed error response to a MemoryData-typed one.
func convertMemoryError(errResp *format.Response[any]) *format.Response[MemoryData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[MemoryData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[MemoryData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
