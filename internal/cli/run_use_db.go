package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/state"
)

// UseDBData is the response data for the use-db command.
type UseDBData struct {
	DatabaseID string `json:"database_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	Database   string `json:"database,omitempty"`
	Username   string `json:"username,omitempty"`
	Switched   bool   `json:"switched"`
}

// RunUseDB executes the querylex use-db command.
// It sets the specified database as the active database in the workspace.
func RunUseDB(id string) *format.Response[UseDBData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[UseDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")
	workspaceFile := filepath.Join(querylexDir, "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)

	// 1. Load workspace and find entry
	ws, err := wsStore.Load()
	if err != nil {
		return format.NewErrorResponse[UseDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	dbEntry := findDatabaseEntry(ws.ConnectedDatabases, id)
	if dbEntry == nil {
		return format.NewErrorResponse[UseDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Database with ID '%s' not found in workspace.", id),
			false,
			traceID,
		)
	}

	// 2. Read database config for connection details
	var host string
	var port int
	var databaseName string
	var username string
	dbDir := filepath.Join(querylexDir, id)
	databaseFile := filepath.Join(dbDir, "database.json")
	if cfg, loadErr := loadDBConfig(databaseFile); loadErr == nil {
		host = cfg.Host
		port = cfg.Port
		databaseName = cfg.Database
		username = cfg.Username
	}

	// 3. Set active database
	if err := wsStore.SetActiveDatabase(id); err != nil {
		return format.NewErrorResponse[UseDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to set active database: %v", err),
			false,
			traceID,
		)
	}

	data := UseDBData{
		DatabaseID: id,
		Name:       dbEntry.Name,
		Type:       dbEntry.Type,
		Host:       host,
		Port:       port,
		Database:   databaseName,
		Username:   username,
		Switched:   true,
	}

	resp := format.NewSuccessResponse(data, traceID, &id)
	resp.Complete(time.Now())
	return resp
}
