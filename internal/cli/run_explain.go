package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/analysis"
	explaincache "github.com/cskiller24/querylex/internal/cache"
	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/format"
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

	// Compute dbDir for cache operations
	home, err := os.UserHomeDir()
	if err != nil {
		resp := format.NewErrorResponse[ExplainData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			format.GenerateTraceID(),
		)
		resp.Complete(time.Now())
		return resp
	}
	dbDir := filepath.Join(home, ".querylex", preflight.ActiveDBID)
	dbType := preflight.DBConfig.Type

	traceID := format.GenerateTraceID()
	return runExplainWithAdapter(preflight.Adapter, query, analyze, traceID, preflight.Workspace.ActiveDatabaseID, dbDir, dbType)
}

// runExplainWithAdapter executes the explain command with a provided adapter.
func runExplainWithAdapter(adapter db.Adapter, query string, analyze bool, traceID string, activeDBID *string, dbDir string, dbType string) *format.Response[ExplainData] {
	start := time.Now()

	// D-06: Check explain cache before calling adapter
	cachedPlan, cacheHit := explaincache.Check(dbDir, query, analyze, dbType)
	if cacheHit {
		signals := analysis.AnalyzeExplainPlan(cachedPlan)
		if signals == nil {
			signals = []analysis.HeuristicSignal{}
		}
		data := ExplainData{Plan: cachedPlan, Heuristics: signals, Analyze: analyze}
		resp := format.NewSuccessResponse(data, traceID, activeDBID)
		cacheHitTrue := true
		resp.Meta.CacheHit = &cacheHitTrue
		resp.Complete(start)
		return resp
	}

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

	// D-06: Write explain cache after successful explain (best-effort)
	normalizedSQL := normalizeSQL(query)
	_ = explaincache.Write(dbDir, result, normalizedSQL, analyze, dbType)

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
	cacheHitFalse := false
	resp.Meta.CacheHit = &cacheHitFalse

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

// normalizeSQL trims whitespace and collapses multiple spaces to a single space.
func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
