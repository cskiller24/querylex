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

// AddDBFlags holds the non-interactive flags for the add-db command.
// When all required fields are populated, the interactive prompts are skipped.
type AddDBFlags struct {
	Type     string
	Name     string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

// HasAllRequired returns true when all required non-interactive flags are populated.
func (f *AddDBFlags) HasAllRequired() bool {
	return f.Type != "" && f.Name != "" && f.Host != "" && f.Database != "" && f.Username != "" && f.Password != ""
}

// ToDBSetupAnswers converts AddDBFlags to the prompt answer struct.
func (f *AddDBFlags) ToDBSetupAnswers() *DBSetupAnswers {
	port := f.Port
	if port == 0 {
		port = DefaultPort(f.Type)
	}
	sslMode := f.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	return &DBSetupAnswers{
		DBType:   f.Type,
		Name:     f.Name,
		Host:     f.Host,
		Port:     port,
		Database: f.Database,
		Username: f.Username,
		Password: f.Password,
		SSLMode:  sslMode,
	}
}

func RunAddDB(flags *AddDBFlags) *format.Response[AddDBData] {
	traceID := uuid.New().String()

	var answers *DBSetupAnswers
	var err error

	if flags != nil && flags.HasAllRequired() {
		answers = flags.ToDBSetupAnswers()
	} else if flags != nil && !flags.HasAllRequired() {
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeInvalidArgument,
			"Incomplete flags for non-interactive add-db. Required: --type, --name, --host, --database, --username, --password",
			false,
			traceID,
		)
	} else {
		answers, err = PromptDatabaseSetup()
		if err != nil {
			return format.NewErrorResponse[AddDBData](
				format.ErrCodeInvalidArgument,
				fmt.Sprintf("Failed to get database setup information: %v", err),
				false,
				traceID,
			)
		}
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

	credRef, err := credStore.Store(dbID, answers.Password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] Storing credentials: %v\n", err)
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeCredentialUnavailable,
			fmt.Sprintf("Failed to store credential: %v", err),
			false,
			traceID,
		)
	}
	credRef.SecretKind = "database-password"
	progressStep(2, 5, "Storing credentials... Credentials stored.")

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
		fmt.Fprintf(os.Stderr, "[FAIL] Connection: %v\n", err)
		return format.NewErrorResponse[AddDBData](
			format.ErrCodeConnectionFailed,
			fmt.Sprintf("Connection to %s database failed: %v", answers.DBType, err),
			false,
			traceID,
		)
	}
	progressStep(1, 5, fmt.Sprintf("Connecting to %s at %s:%d... Connected.", answers.DBType, answers.Host, answers.Port))

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

	// Step 3: Fetching schema
	progressStep(3, 5, "Fetching schema...")

	pipeline := index.NewPipeline(adapter, dbDir, answers.Name, answers.DBType)
	pipeCtx, pipeCancel := context.WithTimeout(context.Background(), 5*time.Minute)

	// Step 4: Periodically read indexing progress from status file
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pipeCtx.Done():
				return
			case <-ticker.C:
				if st, readErr := index.ReadIndexStatus(dbDir); readErr == nil && st != nil {
					fmt.Fprintf(os.Stderr, "[4/5] Indexing schema... %d%%\n", st.ProgressPercent)
				}
			}
		}
	}()

	if pipeErr := index.RunPipeline(pipeCtx, pipeline); pipeErr != nil {
		// Non-fatal — log and report failure
		fmt.Fprintf(os.Stderr, "[FAIL] Indexing: %v\n", pipeErr)
		indexingStatus = string(state.StatusIndexFailed)
		indexingProgress = 0

		// Try to read partial progress from status file
		if st, readErr := index.ReadIndexStatus(dbDir); readErr == nil && st != nil {
			indexingProgress = st.ProgressPercent
		}
	}
	pipeCancel()
	<-progressDone

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

	// Step 5: Workspace saved
	progressStep(5, 5, "Workspace saved.")

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

func progressStep(step, total int, message string) {
	fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", step, total, message)
}

func generateDBID(name string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	suffix := uuid.New().String()[:8]
	return fmt.Sprintf("%s-%s", sanitized, suffix)
}
