package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/state"
)

// DeleteDBData is the response data for the delete-db command.
type DeleteDBData struct {
	DatabaseID string `json:"database_id"`
	Name       string `json:"name"`
	Deleted    bool   `json:"deleted"`
}

// RunDeleteDB executes the querylex delete-db command.
// It removes a database connection from the workspace, deletes its credential,
// and cleans up its artifacts directory (~/.querylex/<id>/).
// If force is false, the user is prompted for confirmation.
func RunDeleteDB(id string, force bool) *format.Response[DeleteDBData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[DeleteDBData](
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
		return format.NewErrorResponse[DeleteDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	dbEntry := findDatabaseEntry(ws.ConnectedDatabases, id)
	if dbEntry == nil {
		return format.NewErrorResponse[DeleteDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Database with ID '%s' not found in workspace.", id),
			false,
			traceID,
		)
	}

	// 2. Confirmation prompt (unless --force)
	if !force {
		confirmed, err := PromptConfirm(
			fmt.Sprintf("Are you sure you want to delete database '%s' (%s)? This will remove all indexed data.", dbEntry.Name, id),
			false,
		)
		if err != nil {
			return format.NewErrorResponse[DeleteDBData](
				format.ErrCodeInvalidArgument,
				fmt.Sprintf("Failed to get confirmation: %v", err),
				false,
				traceID,
			)
		}
		if !confirmed {
			data := DeleteDBData{
				DatabaseID: id,
				Name:       dbEntry.Name,
				Deleted:    false,
			}
			resp := format.NewSuccessResponse(data, traceID, nil)
			resp.Complete(time.Now())
			return resp
		}
	}

	// 3. Delete credential
	dbDir := filepath.Join(querylexDir, id)
	databaseFile := filepath.Join(dbDir, "database.json")

	credStore, err := credentials.SelectCredentialStore()
	if err == nil {
		credRef, loadErr := loadCredentialRef(databaseFile)
		if loadErr == nil && credRef != nil {
			_ = credStore.Delete(credRef.Account)
		}
	}

	// 4. Remove from workspace
	if err := wsStore.DeleteDatabase(id); err != nil {
		return format.NewErrorResponse[DeleteDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to remove database from workspace: %v", err),
			false,
			traceID,
		)
	}

	// 5. Clean up artifacts directory
	_ = os.RemoveAll(dbDir)

	data := DeleteDBData{
		DatabaseID: id,
		Name:       dbEntry.Name,
		Deleted:    true,
	}

	resp := format.NewSuccessResponse(data, traceID, nil)
	resp.Complete(time.Now())
	return resp
}
