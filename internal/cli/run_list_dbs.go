package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/state"
)

// ListDBsData is the response data for the list-dbs command.
type ListDBsData struct {
	Databases []DBListItem `json:"databases"`
	Count     int          `json:"count"`
}

// DBListItem describes a single database connection in the list-dbs output.
type DBListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Database  string `json:"database,omitempty"`
	Username  string `json:"username,omitempty"`
	SSLMode   string `json:"ssl_mode,omitempty"`
	Status    string `json:"status"`
	IsActive  bool   `json:"is_active"`
}

// RunListDBs executes the querylex list-dbs command.
// It loads the workspace and reads database.json for each entry to build
// a comprehensive list of all connected databases with connection details.
func RunListDBs() *format.Response[ListDBsData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[ListDBsData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")
	workspaceFile := filepath.Join(querylexDir, "querylex.json")
	wsStore := state.NewFileWorkspaceStore(workspaceFile)

	ws, err := wsStore.Load()
	if err != nil {
		return format.NewErrorResponse[ListDBsData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	items := make([]DBListItem, 0, len(ws.ConnectedDatabases))

	for _, entry := range ws.ConnectedDatabases {
		item := DBListItem{
			ID:       entry.ID,
			Name:     entry.Name,
			Type:     entry.Type,
			Status:   string(entry.Status),
			IsActive: ws.ActiveDatabaseID != nil && *ws.ActiveDatabaseID == entry.ID,
		}

		// Try to read database.json for connection details
		dbDir := filepath.Join(querylexDir, entry.ID)
		databaseFile := filepath.Join(dbDir, "database.json")
		if cfg, loadErr := loadDBConfig(databaseFile); loadErr == nil {
			item.Host = cfg.Host
			item.Port = cfg.Port
			item.Database = cfg.Database
			item.Username = cfg.Username
			item.SSLMode = cfg.SSLMode
		}

		items = append(items, item)
	}

	// Sort by name for consistent output
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	data := ListDBsData{
		Databases: items,
		Count:     len(items),
	}

	resp := format.NewSuccessResponse(data, traceID, ws.ActiveDatabaseID)
	resp.Complete(time.Now())
	return resp
}
