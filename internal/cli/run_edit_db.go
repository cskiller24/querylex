package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/state"
)

// EditDBData is the response data for the edit-db command.
type EditDBData struct {
	DatabaseID string `json:"database_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Updated    bool   `json:"updated"`
}

// RunEditDB executes the querylex edit-db command.
// It loads an existing database configuration, prompts the user for updated
// values, and saves the changes. If the password is changed, the old credential
// is deleted and the new one is stored.
func RunEditDB(id string) *format.Response[EditDBData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[EditDBData](
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
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	dbEntry := findDatabaseEntry(ws.ConnectedDatabases, id)
	if dbEntry == nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Database with ID '%s' not found in workspace.", id),
			false,
			traceID,
		)
	}

	// 2. Load database config
	dbDir := filepath.Join(querylexDir, id)
	databaseFile := filepath.Join(dbDir, "database.json")
	currentConfig, err := loadDBConfig(databaseFile)
	if err != nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load database config: %v", err),
			false,
			traceID,
		)
	}

	// 3. Prompt for updated values
	answers, err := PromptDatabaseEdit(currentConfig)
	if err != nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeInvalidArgument,
			fmt.Sprintf("Failed to get updated database information: %v", err),
			false,
			traceID,
		)
	}

	// 4. Handle password change
	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		credStore = nil
	}

	// Load existing credential reference
	oldCredRef, _ := loadCredentialRef(databaseFile)

	var credRef *credentials.CredentialReference
	if answers.Password != "" {
		// Password changed — store new credential
		if credStore == nil {
			return format.NewErrorResponse[EditDBData](
				format.ErrCodeCredentialUnavailable,
				"No credential store available. Set up OS keychain or configure QUERYLEX_DB_PASSWORD environment variable.",
				false,
				traceID,
			)
		}

		credRef, err = credStore.Store(id, answers.Password)
		if err != nil {
			return format.NewErrorResponse[EditDBData](
				format.ErrCodeCredentialUnavailable,
				fmt.Sprintf("Failed to store new credential: %v", err),
				false,
				traceID,
			)
		}
		credRef.SecretKind = "database-password"

		// Delete old credential
		if oldCredRef != nil && credStore != nil {
			_ = credStore.Delete(oldCredRef.Account)
		}
	} else {
		// Password not changed — keep existing credential reference
		credRef = oldCredRef
	}

	// 5. Update database.json
	type dbConnectionMeta struct {
		ID            string                        `json:"id"`
		Name          string                        `json:"name"`
		Type          string                        `json:"type"`
		Host          string                        `json:"host"`
		Port          int                           `json:"port"`
		Database      string                        `json:"database"`
		Username      string                        `json:"username"`
		SSLMode       string                        `json:"ssl_mode"`
		CredentialRef *credentials.CredentialReference `json:"credential_reference"`
	}

	meta := dbConnectionMeta{
		ID:            id,
		Name:          answers.Name,
		Type:          currentConfig.Type,
		Host:          answers.Host,
		Port:          answers.Port,
		Database:      answers.Database,
		Username:      answers.Username,
		SSLMode:       answers.SSLMode,
		CredentialRef: credRef,
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to serialize database metadata: %v", err),
			false,
			traceID,
		)
	}

	if err := os.WriteFile(databaseFile, metaData, 0600); err != nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to write database file: %v", err),
			false,
			traceID,
		)
	}

	// 6. Update workspace entry (preserve status and progress)
	updatedEntry := state.DatabaseEntry{
		ID:               id,
		Name:             answers.Name,
		Type:             currentConfig.Type,
		Status:           dbEntry.Status,
		IndexingProgress: dbEntry.IndexingProgress,
	}

	if err := wsStore.UpdateDatabase(id, updatedEntry); err != nil {
		return format.NewErrorResponse[EditDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to update workspace: %v", err),
			false,
			traceID,
		)
	}

	data := EditDBData{
		DatabaseID: id,
		Name:       answers.Name,
		Type:       currentConfig.Type,
		Updated:    true,
	}

	resp := format.NewSuccessResponse(data, traceID, &id)
	resp.Complete(time.Now())
	return resp
}
