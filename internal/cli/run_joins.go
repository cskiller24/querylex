package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/index"
)

// JoinsData is the response data for the joins command.
type JoinsData struct {
	Joins      []db.JoinEdge `json:"joins"`
	Path       []string      `json:"path,omitempty"`
	Tables     []string      `json:"tables"`
	GraphLoaded bool         `json:"graph_loaded"`
}

// RunJoins executes the joins command with a full preflight.
// STUB: replaced in GREEN phase.
func RunJoins(tables []string) *format.Response[JoinsData] {
	traceID := format.GenerateTraceID()
	return format.NewErrorResponse[JoinsData](
		format.ErrCodeJoinPathNotFound,
		"Joins not yet implemented",
		false,
		traceID,
	)
}

// runJoinsWithAdapter executes the joins command with a provided adapter.
// STUB: replaced in GREEN phase.
func runJoinsWithAdapter(adapter db.Adapter, tables []string, traceID string, activeDBID *string) *format.Response[JoinsData] {
	start := time.Now()
	resp := format.NewErrorResponse[JoinsData](
		format.ErrCodeJoinPathNotFound,
		"Joins not yet implemented",
		false,
		traceID,
	)
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
