package cli

import (
	"math"
	"sort"
	"time"

	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/memory"
)

// HistoryData is the response data for the history command.
type HistoryData struct {
	Topic   string          `json:"topic"`
	Results []HistoryResult `json:"results"`
}

// HistoryResult represents a single ranked result in the history response.
type HistoryResult struct {
	ID         string  `json:"id"`
	Input      string  `json:"input"`
	Similarity float64 `json:"similarity"`
	SQL        string  `json:"sql"`
	LastUsedAt string  `json:"last_used_at"`
}

// RunHistory executes the querylex history command.
// It searches memory for all related entries and returns them ranked by
// composite score: (similarity * 0.8) + (recency * 0.2).
func RunHistory(topic string) *format.Response[HistoryData] {
	start := time.Now()
	traceID := format.GenerateTraceID()

	preflight, errResp := PreflightForMemoryCommand()
	if errResp != nil {
		resp := convertHistoryError(errResp)
		resp.Complete(start)
		return resp
	}

	// Search memory with maxResults=0 to get ALL scored entries
	results, warning, err := memory.Search(preflight.DBDir, topic, 0)
	if err != nil {
		resp := format.NewErrorResponse[HistoryData](
			format.ErrCodeMemoryStoreUnavailable,
			"Memory subsystem is unavailable: "+err.Error(),
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	// Compute composite score and sort
	now := time.Now().UTC()
	type scored struct {
		result         HistoryResult
		compositeScore float64
	}
	scoredResults := make([]scored, 0, len(results))

	for _, entry := range results {
		// Compute recency score
		timeStr := entry.Entry.LastUsedAt
		if timeStr == "" {
			timeStr = entry.Entry.UpdatedAt
		}

		var recencyScore float64
		if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
			daysSince := now.Sub(parsedTime).Hours() / 24.0
			recencyScore = math.Exp(-daysSince / 43.3)
			if recencyScore < 0 {
				recencyScore = 0
			}
			if recencyScore > 1 {
				recencyScore = 1
			}
		}

		compositeScore := entry.Similarity*0.8 + recencyScore*0.2

		// Only include results with composite score >= 0.01
		if compositeScore < 0.01 {
			continue
		}

		scoredResults = append(scoredResults, scored{
			result: HistoryResult{
				ID:         entry.Entry.ID,
				Input:      entry.Entry.Input,
				Similarity: entry.Similarity,
				SQL:        entry.Entry.SQL,
				LastUsedAt: entry.Entry.LastUsedAt,
			},
			compositeScore: compositeScore,
		})
	}

	// Sort by composite score descending
	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].compositeScore > scoredResults[j].compositeScore
	})

	// Build results slice
	historyResults := make([]HistoryResult, len(scoredResults))
	for i, s := range scoredResults {
		historyResults[i] = s.result
	}

	data := HistoryData{
		Topic:   topic,
		Results: historyResults,
	}

	resp := format.NewSuccessResponse(data, traceID, &preflight.ActiveDBID)
	resp.Warnings = addEmbeddingsWarning()
	if warning != nil {
		resp.Warnings = append(resp.Warnings, *warning)
	}
	resp.Complete(start)
	return resp
}

// convertHistoryError converts an any-typed error response to a HistoryData-typed one.
func convertHistoryError(errResp *format.Response[any]) *format.Response[HistoryData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[HistoryData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[HistoryData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}
