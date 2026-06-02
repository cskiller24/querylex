package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
)

// StatsTablesData is the response data for the stats --table command.
type StatsTablesData struct {
	Tables []StatsTableEntry `json:"tables"`
}

// StatsTableEntry describes statistics for a single table.
type StatsTableEntry struct {
	Name          string           `json:"table"`
	RowCount      int64            `json:"row_count"`
	DataSizeBytes int64            `json:"data_length_bytes"`
	IndexSizeBytes int64           `json:"index_length_bytes"`
	Cardinality   map[string]int64 `json:"cardinality"`
	UpdatedAt     string           `json:"last_analyzed_at"`
}

// RunStatsTables executes the stats command with a full preflight.
func RunStatsTables(tables []string) *format.Response[StatsTablesData] {
	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertStatsTablesError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()
	return runStatsTablesInternal(preflight.Adapter, tables, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runStatsTablesInternal executes the stats command with a provided adapter.
func runStatsTablesInternal(adapter db.Adapter, tables []string, traceID string, activeDBID *string) *format.Response[StatsTablesData] {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Stats(ctx, tables)
	if err != nil {
		resp := format.NewErrorResponse[StatsTablesData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Stats extraction failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	entries := make([]StatsTableEntry, 0, len(result.Tables))
	for _, t := range result.Tables {
		cardinality := map[string]int64{}
		cardinality[t.Name] = t.CardinalityEstimate

		entries = append(entries, StatsTableEntry{
			Name:           t.Name,
			RowCount:       t.RowCount,
			DataSizeBytes:  t.DataSizeBytes,
			IndexSizeBytes: t.IndexSizeBytes,
			Cardinality:    cardinality,
			UpdatedAt:      t.UpdatedAt,
		})
	}

	data := StatsTablesData{
		Tables: entries,
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)
	resp.Complete(start)
	return resp
}

// convertStatsTablesError converts an any-typed error response to a StatsTablesData-typed one.
func convertStatsTablesError(errResp *format.Response[any]) *format.Response[StatsTablesData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[StatsTablesData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[StatsTablesData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
