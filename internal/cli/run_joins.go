package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
)

// JoinsData is the response data for the joins command.
type JoinsData struct {
	Joins       []db.JoinEdge `json:"joins"`
	Path        []string      `json:"path,omitempty"`
	Tables      []string      `json:"tables"`
	GraphLoaded bool          `json:"graph_loaded"`
}

// RunJoins executes the joins command with a full preflight.
func RunJoins(tables []string) *format.Response[JoinsData] {
	preflight, errResp := PreflightForCommand()
	if errResp != nil {
		return convertJoinsError(errResp)
	}
	defer preflight.Adapter.Close(context.Background())

	traceID := format.GenerateTraceID()

	// Try to load pre-computed join_graph.json first (fast path)
	if graph, ok := loadJoinGraphFromDisk(preflight.ActiveDBID); ok {
		return runJoinsFromGraph(graph, tables, traceID, preflight.Workspace.ActiveDatabaseID)
	}

	return runJoinsWithAdapter(preflight.Adapter, tables, traceID, preflight.Workspace.ActiveDatabaseID)
}

// runJoinsWithAdapter executes the joins command with a provided adapter.
func runJoinsWithAdapter(adapter db.Adapter, tables []string, traceID string, activeDBID *string) *format.Response[JoinsData] {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := adapter.Joins(ctx, tables)
	if err != nil {
		resp := format.NewErrorResponse[JoinsData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Join extraction failed: %v", err),
			false,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	data := JoinsData{
		Joins:       result.Edges,
		Tables:      tables,
		GraphLoaded: false,
	}

	// If exactly 2 tables specified, try to find a specific path
	if len(tables) == 2 && len(result.Edges) > 0 {
		// Build a simple graph to find path
		graph := &index.JoinGraphResult{Edges: result.Edges}
		path, err := index.FindJoinPath(tables[0], tables[1], graph)
		if err == nil {
			data.Path = path
		}
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)

	// Add AMBIGUOUS_JOIN warnings for inferred edges
	for _, edge := range result.Edges {
		if edge.Confidence < 1.0 && edge.SourceType == "inferred_naming_match" {
			resp.Warnings = append(resp.Warnings, format.Warning{
				Code:    "AMBIGUOUS_JOIN",
				Message: fmt.Sprintf("Inferred join between '%s' and '%s' (confidence: %.2f)", edge.Source, edge.Target, edge.Confidence),
			})
		}
	}

	// If no joins found, add warning
	if len(result.Edges) == 0 {
		resp.Warnings = append(resp.Warnings, format.Warning{
			Code:    "JOIN_PATH_NOT_FOUND",
			Message: "No join paths found between the specified tables.",
		})
	}

	resp.Complete(start)
	return resp
}

// runJoinsFromGraph uses a pre-computed JoinGraphResult (from join_graph.json).
func runJoinsFromGraph(graph *index.JoinGraphResult, tables []string, traceID string, activeDBID *string) *format.Response[JoinsData] {
	start := time.Now()

	// Filter edges relevant to the requested tables
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t] = true
	}

	var relevantEdges []db.JoinEdge
	for _, edge := range graph.Edges {
		if len(tables) == 0 || tableSet[edge.Source] || tableSet[edge.Target] {
			relevantEdges = append(relevantEdges, edge)
		}
	}

	data := JoinsData{
		Joins:       relevantEdges,
		Tables:      tables,
		GraphLoaded: true,
	}

	// If exactly 2 tables specified, find path from graph
	if len(tables) == 2 {
		path, err := index.FindJoinPath(tables[0], tables[1], graph)
		if err == nil {
			data.Path = path
		}
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)

	// Add warnings for inferred edges
	for _, edge := range relevantEdges {
		if edge.Confidence < 1.0 && edge.SourceType == "inferred_naming_match" {
			resp.Warnings = append(resp.Warnings, format.Warning{
				Code:    "AMBIGUOUS_JOIN",
				Message: fmt.Sprintf("Inferred join between '%s' and '%s' (confidence: %.2f)", edge.Source, edge.Target, edge.Confidence),
			})
		}
	}

	if len(relevantEdges) == 0 {
		resp.Warnings = append(resp.Warnings, format.Warning{
			Code:    "JOIN_PATH_NOT_FOUND",
			Message: "No join paths found between the specified tables.",
		})
	}

	resp.Complete(start)
	return resp
}

// loadJoinGraphFromDisk attempts to load pre-computed join_graph.json.
func loadJoinGraphFromDisk(activeDBID string) (*index.JoinGraphResult, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false
	}
	joinGraphPath := filepath.Join(home, ".querylex", activeDBID, "schema", "join_graph.json")
	data, err := os.ReadFile(joinGraphPath)
	if err != nil {
		return nil, false
	}
	var graph index.JoinGraphResult
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, false
	}
	return &graph, true
}

// convertJoinsError converts an any-typed error response to a JoinsData-typed one.
func convertJoinsError(errResp *format.Response[any]) *format.Response[JoinsData] {
	if errResp.Error != nil {
		return format.NewErrorResponse[JoinsData](
			errResp.Error.Code,
			errResp.Error.Message,
			errResp.Error.Retryable,
			errResp.Meta.TraceID,
		)
	}
	return format.NewErrorResponse[JoinsData](
		format.ErrCodeInternalError,
		"Unknown error during preflight",
		false,
		format.GenerateTraceID(),
	)
}

// hasInferredEdge checks if any edge is an inferred naming match.
func hasInferredEdge(edges []db.JoinEdge) bool {
	for _, e := range edges {
		if strings.HasPrefix(e.SourceType, "inferred") {
			return true
		}
	}
	return false
}
