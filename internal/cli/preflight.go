package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/db/mysql"
	"github.com/cskiller24/querylex/internal/db/postgresql"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
)

// PreflightResult holds the result of the standard command preflight.
type PreflightResult struct {
	Workspace    *state.Workspace
	ActiveDBID   string
	DBConfig     *DBConnectionConfig
	Adapter      db.Adapter
	CredStore    credentials.CredentialStore
}

// MemoryPreflight holds the result of the lightweight memory command preflight.
type MemoryPreflight struct {
	Workspace  *state.Workspace
	ActiveDBID string
	DBDir      string // $HOME/.querylex/<db-id>/
}

// DBConnectionConfig represents the connection metadata for a database.
type DBConnectionConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	SSLMode  string `json:"ssl_mode"`
}

// PreflightForCommand performs the standard preflight for all deterministic commands:
//  1. Load workspace
//  2. Resolve active database ID
//  3. Status gating (indexed/stale → proceed; not_indexed/index_failed → error)
//  4. Load database.json
//  5. Get credentials
//  6. Build DSN
//  7. Create adapter via factory
//  8. Connect
func PreflightForCommand() (*PreflightResult, *format.Response[any]) {
	traceID := format.GenerateTraceID()

	// 1. Load workspace
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, format.NewErrorResponse[any](
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
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	// 2. Resolve active database ID
	if ws.ActiveDatabaseID == nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeInvalidArgument,
			"No active database. Set one with 'querylex add-db'.",
			false,
			traceID,
		)
	}
	activeDBID := *ws.ActiveDatabaseID

	// 3. Status gating
	dbEntry := findDatabaseEntry(ws.ConnectedDatabases, activeDBID)
	if dbEntry == nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Active database '%s' not found in workspace.", activeDBID),
			false,
			traceID,
		)
	}

	switch dbEntry.Status {
	case state.StatusNotIndexed, state.StatusIndexFailed:
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Database '%s' is not indexed (status: %s). Run indexing first.", dbEntry.Name, dbEntry.Status),
			false,
			traceID,
		)
	case state.StatusIndexing:
		// Warn but proceed
	case state.StatusIndexed, state.StatusStale:
		// Proceed
	default:
		// Unknown status — proceed with warning
	}

	// 3.5. Stale detection: if indexed, verify manifest checksums against current artifacts (D-05)
	if dbEntry.Status == state.StatusIndexed {
		dbDir := filepath.Join(home, ".querylex", activeDBID)
		manifest, _ := index.ReadIndexManifest(dbDir)
		if manifest != nil {
			if valid, _ := index.VerifyManifest(dbDir, manifest); !valid {
				// Checksum mismatch or artifact missing — mark as stale
				if ws, loadErr := wsStore.Load(); loadErr == nil {
					for i := range ws.ConnectedDatabases {
						if ws.ConnectedDatabases[i].ID == activeDBID {
							ws.ConnectedDatabases[i].Status = state.StatusStale
							break
						}
					}
					_ = wsStore.Save(ws)
				}
			}
		} else {
			// No manifest for indexed DB — unexpected, mark as stale
			if ws, loadErr := wsStore.Load(); loadErr == nil {
				for i := range ws.ConnectedDatabases {
					if ws.ConnectedDatabases[i].ID == activeDBID {
						ws.ConnectedDatabases[i].Status = state.StatusStale
						break
					}
				}
				_ = wsStore.Save(ws)
			}
		}
	}

	// 4. Load database.json
	dbDir := filepath.Join(home, ".querylex", activeDBID)
	databaseFile := filepath.Join(dbDir, "database.json")
	dbConfig, err := loadDBConfig(databaseFile)
	if err != nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load database config: %v", err),
			false,
			traceID,
		)
	}

	// 5. Get credentials
	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		credStore = nil // preflight tolerates nil credStore — error if actually needed
	}
	// Auto-unlock EncryptedFileStore when QUERYLEX_KEYCHAIN_PASSPHRASE is set.
	// This mirrors the interactive promptEncryptedFilePassphrase pattern used
	// by run_adddb.go and run_ai_config.go but is non-interactive — intended
	// for CI/E2E environments where the passphrase is passed via env var.
	if encStore, ok := credStore.(*credentials.EncryptedFileStore); ok {
		if passphrase := os.Getenv("QUERYLEX_KEYCHAIN_PASSPHRASE"); passphrase != "" {
			if unlockErr := encStore.Unlock(passphrase); unlockErr != nil {
				// Auto-unlock failed — clear credStore so PreflightForCommand
				// falls through to the env-var or empty-password path. The
				// credential store interface already returns nil for
				// ErrPassphraseRequired; this handles wrong-passphrase too.
				credStore = nil
			}
		}
	}
	var password string
	var credRef *credentials.CredentialReference
	// Load credential reference from database.json alongside config
	credRef, err = loadCredentialRef(databaseFile)
	if err != nil {
		// Non-fatal — some database types (SQLite) don't need credentials
	}
	if credStore != nil && credRef != nil {
		password, err = credStore.Retrieve(credRef)
		if err != nil {
			return nil, format.NewErrorResponse[any](
				format.ErrCodeCredentialUnavailable,
				fmt.Sprintf("Failed to retrieve credentials: %v", err),
				true,
				traceID,
			)
		}
	}

	// 6. Build DSN
	dsn := buildDSN(dbConfig.Type, dbConfig, password)

	// 7. Create adapter via factory
	adapter, err := db.Open(dbConfig.Type, dsn)
	if err != nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeUnsupportedDatabase,
			fmt.Sprintf("Cannot open adapter for %s: %v", dbConfig.Type, err),
			false,
			traceID,
		)
	}

	// 8. Connect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := adapter.Connect(ctx, dsn); err != nil {
		adapter.Close(context.Background())
		return nil, format.NewErrorResponse[any](
			format.ErrCodeConnectionFailed,
			fmt.Sprintf("Connection to %s database failed: %v", dbConfig.Type, err),
			true,
			traceID,
		)
	}

	return &PreflightResult{
		Workspace:  ws,
		ActiveDBID: activeDBID,
		DBConfig:   dbConfig,
		Adapter:    adapter,
		CredStore:  credStore,
	}, nil
}

// PreflightForMemoryCommand performs a lightweight preflight for memory commands.
// It validates workspace state and active database without connecting to the database.
// Returns a MemoryPreflight on success, or an error response on failure.
func PreflightForMemoryCommand() (*MemoryPreflight, *format.Response[any]) {
	traceID := format.GenerateTraceID()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, format.NewErrorResponse[any](
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
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Failed to load workspace state: %v", err),
			false,
			traceID,
		)
	}

	// Validate active database ID is set
	if ws.ActiveDatabaseID == nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeInvalidArgument,
			"No active database. Set one with 'querylex add-db'.",
			false,
			traceID,
		)
	}
	activeDBID := *ws.ActiveDatabaseID

	// Validate active database exists in ConnectedDatabases
	dbEntry := findDatabaseEntry(ws.ConnectedDatabases, activeDBID)
	if dbEntry == nil {
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Active database '%s' not found in workspace.", activeDBID),
			false,
			traceID,
		)
	}

	// Status gate: reject not_indexed and index_failed
	switch dbEntry.Status {
	case state.StatusNotIndexed, state.StatusIndexFailed:
		return nil, format.NewErrorResponse[any](
			format.ErrCodeWorkspaceStateInvalid,
			fmt.Sprintf("Database '%s' is not indexed (status: %s). Run indexing first.", dbEntry.Name, dbEntry.Status),
			false,
			traceID,
		)
	}

	dbDir := filepath.Join(home, ".querylex", activeDBID)

	return &MemoryPreflight{
		Workspace:  ws,
		ActiveDBID: activeDBID,
		DBDir:      dbDir,
	}, nil
}

// findDatabaseEntry looks up a DatabaseEntry by ID in the workspace.
func findDatabaseEntry(entries []state.DatabaseEntry, id string) *state.DatabaseEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

// DBConfigJSON is the on-disk format of database.json.
type DBConfigJSON struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Type          string                       `json:"type"`
	Host          string                       `json:"host"`
	Port          int                          `json:"port"`
	Database      string                       `json:"database"`
	Username      string                       `json:"username"`
	SSLMode       string                       `json:"ssl_mode"`
	CredentialRef *credentials.CredentialReference `json:"credential_reference"`
}

// loadDBConfig reads and parses the database.json file.
func loadDBConfig(path string) (*DBConnectionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read database config: %w", err)
	}

	var cfg DBConfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	return &DBConnectionConfig{
		ID:       cfg.ID,
		Name:     cfg.Name,
		Type:     cfg.Type,
		Host:     cfg.Host,
		Port:     cfg.Port,
		Database: cfg.Database,
		Username: cfg.Username,
		SSLMode:  cfg.SSLMode,
	}, nil
}

// loadCredentialRef reads the credential reference from database.json.
func loadCredentialRef(path string) (*credentials.CredentialReference, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read database config: %w", err)
	}

	var cfg DBConfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	return cfg.CredentialRef, nil
}

// buildDSN constructs a DSN string based on database type.
func buildDSN(dbType string, cfg *DBConnectionConfig, password string) string {
	switch dbType {
	case "mysql":
		return mysql.BuildDSN(cfg.Host, cfg.Port, cfg.Database, cfg.Username, password, cfg.SSLMode)
	case "postgres", "postgresql":
		return postgresql.BuildDSN(cfg.Host, cfg.Port, cfg.Database, cfg.Username, password, cfg.SSLMode)
	case "sqlite":
		return cfg.Database // file path
	default:
		return ""
	}
}
