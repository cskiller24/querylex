package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/db"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/memory"
	"github.com/cskiller24/querylex/internal/state"
)

// HealthReport contains health information for all connected databases.
type HealthReport struct {
	Databases []DatabaseHealth `json:"databases"`
}

// DatabaseHealth describes the health state of a single connected database.
type DatabaseHealth struct {
	DatabaseID          string            `json:"database_id"`
	DatabaseName        string            `json:"database_name"`
	Status              string            `json:"status"`
	ProgressPercent     int               `json:"indexing_progress"`
	Artifacts           map[string]string `json:"artifacts"`
	CredentialStatus    string            `json:"credential_status"`
	MemoryIndexState    string            `json:"memory_index_state"`
	ExplainCacheSummary string            `json:"explain_cache_summary"`
	Connectivity        string            `json:"connectivity"`
}

const (
	artifactStatePresent = "present"
	artifactStateStale   = "stale"
	artifactStateMissing = "missing"
)

// knownArtifactPaths lists all expected indexing artifacts per database.
var knownArtifactPaths = []string{
	"schema/schema.json",
	"schema/schema_slim.json",
	"schema/join_graph.json",
	"schema/schema_map.json",
	"domain_map.json",
	"schema/domain_map.json",
	"terminologies.md",
}

type StatsData struct {
	ActiveDatabaseID   *string               `json:"active_database_id"`
	ConnectedDatabases []state.DatabaseEntry  `json:"connected_databases"`
	Health             *HealthReport          `json:"health"`
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

	// Build health report for all connected databases
	health := buildHealthReport(ws, home)

	if len(ws.ConnectedDatabases) == 0 {
		warning := format.Warning{
			Code:    "NO_DATABASES_CONNECTED",
			Message: "No databases connected. Use 'querylex add-db' to add a database.",
		}
		data := StatsData{
			ActiveDatabaseID:   nil,
			ConnectedDatabases: []state.DatabaseEntry{},
			Health:             health,
		}
		resp := format.NewSuccessResponse(data, traceID, nil)
		resp.Warnings = []format.Warning{warning}
		return resp
	}

	data := StatsData{
		ActiveDatabaseID:   ws.ActiveDatabaseID,
		ConnectedDatabases: ws.ConnectedDatabases,
		Health:             health,
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

	// Add stale artifact warnings
	if health != nil {
		for _, dbHealth := range health.Databases {
			hasStale := false
			for _, artifactState := range dbHealth.Artifacts {
				if artifactState == artifactStateStale {
					hasStale = true
					break
				}
			}
			if hasStale {
				resp.Warnings = append(resp.Warnings, format.Warning{
					Code:    "DATABASE_ARTIFACTS_STALE",
					Message: fmt.Sprintf("Database '%s' has stale indexing artifacts. Re-run indexing to refresh.", dbHealth.DatabaseName),
				})
			}
		}
	}

	return resp
}

// buildHealthReport constructs a HealthReport for all connected databases in the workspace.
func buildHealthReport(ws *state.Workspace, home string) *HealthReport {
	health := &HealthReport{Databases: make([]DatabaseHealth, 0, len(ws.ConnectedDatabases))}

	for _, entry := range ws.ConnectedDatabases {
		dbDir := filepath.Join(home, ".querylex", entry.ID)

		connectivity := "not_checked"
		cfgPath := filepath.Join(dbDir, "database.json")
		if cfgData, err := os.ReadFile(cfgPath); err == nil {
			var cfg DBConfigJSON
			if json.Unmarshal(cfgData, &cfg) == nil {
				if cfg.Type == "sqlite" {
					connectivity = checkConnectivity(dbDir, cfg.Type, cfg.Database)
				} else {
					ref := cfg.CredentialRef
					password := ""
					if ref != nil {
						credStore, csErr := credentials.SelectCredentialStore()
						if csErr == nil {
							if pwd, getErr := credStore.Retrieve(ref); getErr == nil {
								password = pwd
							}
						}
					}
					connConfig := &DBConnectionConfig{
						ID:       cfg.ID,
						Name:     cfg.Name,
						Type:     cfg.Type,
						Host:     cfg.Host,
						Port:     cfg.Port,
						Database: cfg.Database,
						Username: cfg.Username,
						SSLMode:  cfg.SSLMode,
					}
					dsn := buildDSN(connConfig.Type, connConfig, password)
					if dsn != "" {
						connectivity = checkConnectivity(dbDir, connConfig.Type, dsn)
					}
				}
			}
		}

		dbHealth := DatabaseHealth{
			DatabaseID:          entry.ID,
			DatabaseName:        entry.Name,
			Status:              string(entry.Status),
			ProgressPercent:     entry.IndexingProgress,
			Artifacts:           scanArtifacts(dbDir, entry.Status),
			CredentialStatus:    checkCredentialStatus(dbDir),
			MemoryIndexState:    checkMemoryHealth(dbDir),
			ExplainCacheSummary: checkExplainCacheSummary(dbDir),
			Connectivity:        connectivity,
		}

		health.Databases = append(health.Databases, dbHealth)
	}

	return health
}

// scanArtifacts checks the state of all known indexing artifacts for a database.
func scanArtifacts(dbDir string, status state.DatabaseStatus) map[string]string {
	artifacts := make(map[string]string, len(knownArtifactPaths))

	var manifest *index.IndexManifest
	if status == state.StatusIndexed {
		manifest, _ = index.ReadIndexManifest(dbDir)
	}

	for _, artifactPath := range knownArtifactPaths {
		fullPath := filepath.Join(dbDir, artifactPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			artifacts[artifactPath] = artifactStateMissing
			continue
		}

		if status == state.StatusIndexed && manifest != nil {
			expectedChecksum, inManifest := manifest.ArtifactChecksums[artifactPath]
			if inManifest {
				actualChecksum, err := index.ComputeChecksum(fullPath)
				if err != nil {
					artifacts[artifactPath] = artifactStateMissing
				} else if actualChecksum == expectedChecksum {
					artifacts[artifactPath] = artifactStatePresent
				} else {
					artifacts[artifactPath] = artifactStateStale
				}
			} else {
				// Artifact not tracked in manifest (e.g., terminologies.md)
				artifacts[artifactPath] = artifactStatePresent
			}
		} else {
			// Non-indexed status or no manifest — simple file existence check
			artifacts[artifactPath] = artifactStatePresent
		}
	}

	return artifacts
}

// checkMemoryHealth determines the health state of the memory subsystem for a database.
// Returns "healthy" when memory.sqlite exists and revision matches index,
// "stale" when revision mismatch, "missing" when files absent.
func checkMemoryHealth(dbDir string) string {
	dbPath := filepath.Join(dbDir, "memory.sqlite")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "missing"
	}

	// Open memory store
	db, err := memory.OpenStore(dbDir)
	if err != nil {
		return "missing"
	}
	defer db.Close()

	// Get SQLite revision
	ctx := context.Background()
	sqliteRevision, err := memory.GetRevision(ctx, db)
	if err != nil {
		return "missing"
	}

	// Read index
	idx, err := memory.ReadIndex(dbDir)
	if err != nil || idx == nil {
		return "missing"
	}

	// Compare revisions
	if idx.Revision == sqliteRevision {
		return "healthy"
	}
	return "stale"
}

// checkExplainCacheSummary returns a summary of the explain cache for a database.
// Returns "{N} entries" with the count of cached JSON files, or "unavailable"
// if the cache directory does not exist.
func checkExplainCacheSummary(dbDir string) string {
	cacheDir := filepath.Join(dbDir, "explain_cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "unavailable"
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}

	return fmt.Sprintf("%d entries", count)
}

// connectivityCache holds ping results keyed by database ID with expiry.
var connectivityCache sync.Map

type connCacheEntry struct {
	status    string
	expiresAt time.Time
}

// checkConnectivity attempts to ping the database and returns a status string.
// Results are cached for 30 seconds to avoid hammering unreachable databases.
func checkConnectivity(dbDir, dbType, dsn string) string {
	if dbType == "sqlite" {
		return "online"
	}

	if cached, ok := connectivityCache.Load(dbDir); ok {
		entry := cached.(connCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.status
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	adapter, err := db.Open(dbType, dsn)
	if err != nil {
		entry := connCacheEntry{status: "unreachable", expiresAt: time.Now().Add(30 * time.Second)}
		connectivityCache.Store(dbDir, entry)
		return "unreachable"
	}

	err = adapter.TestConnect(ctx, dsn)

	var status string
	switch {
	case err == nil:
		status = "online"
	case ctx.Err() != nil:
		status = "timeout"
	case strings.Contains(err.Error(), "access denied") || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "login"):
		status = "auth_failed"
	default:
		status = "unreachable"
	}

	entry := connCacheEntry{status: status, expiresAt: time.Now().Add(30 * time.Second)}
	connectivityCache.Store(dbDir, entry)
	return status
}

// RenderStatsHuman renders workspace stats as a human-readable terminal output.
func RenderStatsHuman(w io.Writer, data StatsData) {
	fmt.Fprintf(w, "Querylex Workspace Health\n=========================\n\n")

	if data.ActiveDatabaseID != nil {
		fmt.Fprintf(w, "Active database: %s\n\n", *data.ActiveDatabaseID)
	}

	if len(data.ConnectedDatabases) == 0 {
		fmt.Fprintln(w, "No databases connected. Run 'querylex add-db' to add one.")
		return
	}

	if data.Health == nil {
		return
	}

	for _, db := range data.Health.Databases {
		fmt.Fprintf(w, "  %s (%s)\n", db.DatabaseName, db.DatabaseID)
		fmt.Fprintf(w, "    Status:         %s\n", db.Status)
		fmt.Fprintf(w, "    Indexing:       %d%%\n", db.ProgressPercent)
		fmt.Fprintf(w, "    Credentials:    %s\n", db.CredentialStatus)
		fmt.Fprintf(w, "    Connectivity:   %s\n", db.Connectivity)
		fmt.Fprintf(w, "    Memory Index:   %s\n", db.MemoryIndexState)
		fmt.Fprintf(w, "    Explain Cache:  %s\n", db.ExplainCacheSummary)

		if len(db.Artifacts) > 0 {
			fmt.Fprintf(w, "    Artifacts:\n")
			for path, state := range db.Artifacts {
				fmt.Fprintf(w, "      %s: %s\n", path, state)
			}
		}

		fmt.Fprintln(w)
	}
}

// checkCredentialStatus determines credential availability for a database.
func checkCredentialStatus(dbDir string) string {
	data, err := os.ReadFile(filepath.Join(dbDir, "database.json"))
	if err != nil {
		return "missing"
	}

	var cfg DBConfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "missing"
	}

	if cfg.Type == "sqlite" {
		return "not_required"
	}

	if cfg.CredentialRef == nil {
		return "missing"
	}

	// Check if a credential store is available
	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		credStore = nil
	}
	if credStore != nil {
		return "available"
	}

	return "missing"
}
