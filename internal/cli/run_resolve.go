package cli

import (
	"os"
	"path/filepath"
	"time"

	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/queryutil"
	"github.com/querylex/querylex/internal/state"
)

// ResolveData is the response data for the resolve command.
type ResolveData struct {
	Tables     []queryutil.TableCandidate  `json:"tables"`
	Columns    []queryutil.ColumnCandidate `json:"columns"`
	Confidence float64                     `json:"confidence"`
}

// RunResolve executes the resolve command, loading schema_slim.json from disk.
// No database connection needed. Pure computation from local artifacts.
func RunResolve(question string) *format.Response[ResolveData] {
	traceID := format.GenerateTraceID()

	// Load workspace
	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[ResolveData](
			format.ErrCodeInternalError,
			"Cannot determine home directory",
			false,
			traceID,
		)
	}

	workspaceFile := filepath.Join(home, ".querylex", "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)

	ws, err := wsStore.Load()
	if err != nil {
		return format.NewErrorResponse[ResolveData](
			format.ErrCodeWorkspaceStateInvalid,
			"Failed to load workspace state",
			false,
			traceID,
		)
	}

	if ws.ActiveDatabaseID == nil {
		return format.NewErrorResponse[ResolveData](
			format.ErrCodeInvalidArgument,
			"No active database. Set one with 'querylex-add-db'.",
			false,
			traceID,
		)
	}

	// Load schema_slim.json from disk
	dbDir := filepath.Join(home, ".querylex", *ws.ActiveDatabaseID)
	slimPath := filepath.Join(dbDir, "schema", "schema_slim.json")
	slimData, err := os.ReadFile(slimPath)
	if err != nil {
		return format.NewErrorResponse[ResolveData](
			format.ErrCodeSchemaParseError,
			"Unable to load schema metadata for active database",
			true,
			traceID,
		)
	}

	return runResolveWithSlim(question, slimData, traceID, ws.ActiveDatabaseID)
}

// runResolveWithSlim executes resolve with provided slim schema data (for testing).
func runResolveWithSlim(question string, slimData []byte, traceID string, activeDBID *string) *format.Response[ResolveData] {
	start := time.Now()

	// Call multi-pass deterministic resolution
	result, err := queryutil.ResolveTokens(question, slimData)
	if err != nil {
		resp := format.NewErrorResponse[ResolveData](
			format.ErrCodeSchemaParseError,
			"Unable to parse schema metadata",
			true,
			traceID,
		)
		resp.Complete(start)
		return resp
	}

	data := ResolveData{
		Tables:     result.Tables,
		Columns:    result.Columns,
		Confidence: result.Confidence,
	}

	resp := format.NewSuccessResponse(data, traceID, activeDBID)

	// Empty results are valid per D-07 — add NO_MATCHING_TABLES warning
	if len(result.Tables) == 0 {
		resp.Warnings = append(resp.Warnings, format.Warning{
			Code:    "NO_MATCHING_TABLES",
			Message: "No matching tables found for the given input.",
		})
	}

	resp.Complete(start)
	return resp
}
