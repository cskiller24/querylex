package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/state"
)

type StatsData struct {
	ActiveDatabaseID   *string               `json:"active_database_id"`
	ConnectedDatabases []state.DatabaseEntry  `json:"connected_databases"`
}

func RunStats() *format.Response[StatsData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[StatsData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	workspaceFile := filepath.Join(home, ".querylex", "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)

	ws, err := wsStore.Load()
	if err != nil {
		return format.NewErrorResponse[StatsData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	if len(ws.ConnectedDatabases) == 0 {
		warning := format.Warning{
			Code:    "NO_DATABASES_CONNECTED",
			Message: "No databases connected. Use 'querylex-add-db' to add a database.",
		}
		data := StatsData{
			ActiveDatabaseID:   nil,
			ConnectedDatabases: []state.DatabaseEntry{},
		}
		resp := format.NewSuccessResponse(data, traceID, nil)
		resp.Warnings = []format.Warning{warning}
		return resp
	}

	data := StatsData{
		ActiveDatabaseID:   ws.ActiveDatabaseID,
		ConnectedDatabases: ws.ConnectedDatabases,
	}

	resp := format.NewSuccessResponse(data, traceID, ws.ActiveDatabaseID)

	for _, entry := range ws.ConnectedDatabases {
		if entry.Status != state.StatusIndexed {
			resp.Warnings = append(resp.Warnings, format.Warning{
				Code:    "DATABASE_NOT_INDEXED",
				Message: fmt.Sprintf("Database '%s' has status '%s'. Run indexing to enable query features.", entry.Name, entry.Status),
			})
		}
	}

	return resp
}
