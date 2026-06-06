package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/db/mysql"
	"github.com/cskiller24/querylex/internal/db/postgresql"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
)

type AddDBData struct {
	DatabaseID         string                        `json:"database_id"`
	Name               string                        `json:"name"`
	Type               string                        `json:"type"`
	CredentialRef      *credentials.CredentialReference `json:"credential_reference"`
	DatabaseFile       string                        `json:"database_file"`
	WorkspaceFile      string                        `json:"workspace_file"`
	IndexingStatus     string                        `json:"indexing_status"`
	IndexingProgress   int                           `json:"indexing_progress"`
}

func RunAddDB() *format.Response[AddDBData] {
	traceID := uuid.New().String()

	answers, err := PromptDatabaseSetup()
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInvalidArgument,
			fmt.Sprintf("Failed to get database setup information: %v", err),
			false,
			traceID,
		)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	dbID := generateDBID(answers.Name)

	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeCredentialUnavailable,
			"No credential store available. Set up OS keychain or configure QUERYLEX_DB_PASSWORD environment variable.",
			false,
			traceID,
		)
	}

	// If the encrypted file store was selected, prompt for passphrase
	if encStore, ok := credStore.(*credentials.EncryptedFileStore); ok {
		if ppErr := promptEncryptedFilePassphrase(encStore); ppErr != nil {
			return format.NewErrorResponse[AddDBData](
				format.ErrCodeCredentialUnavailable,
				ppErr.Error(),
				false,
				traceID,
			)
		}
	}

	credRef, err := credStore.Store(dbID, answers.Password)
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeCredentialUnavailable,
			fmt.Sprintf("Failed to store credential: %v", err),
			false,
			traceID,
		)
	}
	credRef.SecretKind = "database-password"

	var dsn string
	switch answers.DBType {
	case "mysql":
		dsn = mysql.BuildDSN(answers.Host, answers.Port, answers.Database, answers.Username, answers.Password, answers.SSLMode)
	case "postgres":
		dsn = postgresql.BuildDSN(answers.Host, answers.Port, answers.Database, answers.Username, answers.Password, answers.SSLMode)
	default:
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeUnsupportedDatabase,
			fmt.Sprintf("Unsupported database type: %s", answers.DBType),
			false,
			traceID,
		)
	}

	adapter, err := db.Open(answers.DBType, dsn)
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeUnsupportedDatabase,
			fmt.Sprintf("Cannot open adapter for %s: %v", answers.DBType, err),
			false,
			traceID,
		)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := adapter.Connect(pingCtx, dsn); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeConnectionFailed,
			fmt.Sprintf("Connection to %s database failed: %v", answers.DBType, err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")
	if err := os.MkdirAll(querylexDir, 0700); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot create .querylex directory: %v", err),
			false,
			traceID,
		)
	}

	dbDir := filepath.Join(querylexDir, dbID)
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot create database directory: %v", err),
			false,
			traceID,
		)
	}

	databaseFile := filepath.Join(dbDir, "database.json")
	workspaceFile := filepath.Join(querylexDir, "querylex.json")

	type dbConnectionMeta struct {
		ID                 string                        `json:"id"`
		Name               string                        `json:"name"`
		Type               string                        `json:"type"`
		Host               string                        `json:"host"`
		Port               int                           `json:"port"`
		Database           string                        `json:"database"`
		Username           string                        `json:"username"`
		SSLMode            string                        `json:"ssl_mode"`
		CredentialRef      *credentials.CredentialReference `json:"credential_reference"`
	}

	meta := dbConnectionMeta{
		ID:            dbID,
		Name:          answers.Name,
		Type:          answers.DBType,
		Host:          answers.Host,
		Port:          answers.Port,
		Database:      answers.Database,
		Username:      answers.Username,
		SSLMode:       answers.SSLMode,
		CredentialRef: credRef,
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to serialize database metadata: %v", err),
			false,
			traceID,
		)
	}

	if err := os.WriteFile(databaseFile, metaData, 0600); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to write database file: %v", err),
			false,
			traceID,
		)
	}

	wsStore := state.NewFileWorkspaceStore(workspaceFile)
	entry := state.DatabaseEntry{
		ID:               dbID,
		Name:             answers.Name,
		Type:             answers.DBType,
		Status:           state.StatusNotIndexed,
		IndexingProgress: 0,
	}

	if err := wsStore.AddDatabase(entry); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to update workspace: %v", err),
			false,
			traceID,
		)
	}

	if err := wsStore.SetActiveDatabase(dbID); err != nil {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to set active database: %v", err),
			false,
			traceID,
		)
	}

	// Run indexing pipeline synchronously
	indexingStatus := string(state.StatusIndexed)
	indexingProgress := 100
	pipeline := index.NewPipeline(adapter, dbDir, answers.Name, answers.DBType)
	pipeCtx, pipeCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if pipeErr := index.RunPipeline(pipeCtx, pipeline); pipeErr != nil {
		// Non-fatal — log and report failure
		indexingStatus = string(state.StatusIndexFailed)
		indexingProgress = 0

		// Try to read partial progress from status file
		if st, readErr := index.ReadIndexStatus(dbDir); readErr == nil && st != nil {
			indexingProgress = st.ProgressPercent
		}
	}
	pipeCancel()

	// Update workspace entry status
	if ws, loadErr := wsStore.Load(); loadErr == nil {
		for i := range ws.ConnectedDatabases {
			if ws.ConnectedDatabases[i].ID == dbID {
				ws.ConnectedDatabases[i].Status = state.DatabaseStatus(indexingStatus)
				ws.ConnectedDatabases[i].IndexingProgress = indexingProgress
				_ = wsStore.Save(ws)
				break
			}
		}
	}

	// Close adapter after indexing completes
	adapter.Close(context.Background())

	data := AddDBData{
		DatabaseID:       dbID,
		Name:             answers.Name,
		Type:             answers.DBType,
		CredentialRef:    credRef,
		DatabaseFile:     databaseFile,
		WorkspaceFile:    workspaceFile,
		IndexingStatus:   indexingStatus,
		IndexingProgress: indexingProgress,
	}

	resp := format.NewSuccessResponse(data, traceID, &dbID)
	return resp
}

// Deprecated: use credentials.SelectCredentialStore() from factory.go instead.
// This local implementation has incorrect priority order (EnvVar before EncryptedFile).
func selectCredentialStore() credentials.CredentialStore {
	keychain := credentials.NewKeychainStore()
	if keychain.Available() {
		return keychain
	}

	envStore := credentials.NewEnvStore()
	if envStore.Available() {
		return envStore
	}

	home, err := os.UserHomeDir()
	if err == nil {
		encFile := filepath.Join(home, ".querylex", "credentials.json.enc")
		encStore := credentials.NewEncryptedFileStore(encFile)
		return encStore
	}

	return nil
}

func generateDBID(name string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	suffix := uuid.New().String()[:8]
	return fmt.Sprintf("%s-%s", sanitized, suffix)
}
