package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/querylex/querylex/internal/credentials"
	"github.com/querylex/querylex/internal/index"
	"github.com/querylex/querylex/internal/state"
)

// setupWorkspaceAndDB creates a test workspace and database directory for RunStats tests.
// Returns the home directory path (a temp dir).
func setupStatsTestWorkspace(t *testing.T, entries []state.DatabaseEntry) string {
	t.Helper()
	home := t.TempDir()

	wsDir := filepath.Join(home, ".querylex")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir .querylex: %v", err)
	}

	ws := &state.Workspace{
		ConnectedDatabases: entries,
	}
	if len(entries) > 0 {
		ws.ActiveDatabaseID = &entries[0].ID
	}
	wsData, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("marshal workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	return home
}

// setupStatsTestDB creates a database directory within the test workspace with the given artifacts.
func setupStatsTestDB(t *testing.T, home, dbID string, dbType string, artifacts map[string]string) {
	t.Helper()
	dbDir := filepath.Join(home, ".querylex", dbID)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	// Create database.json
	dbJSON := DBConfigJSON{
		ID:       dbID,
		Name:     dbID,
		Type:     dbType,
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "testuser",
		SSLMode:  "require",
	}
	if dbType != "sqlite" {
		dbJSON.CredentialRef = &credentials.CredentialReference{
			Provider: "keychain",
			Service:  "test",
			Account:  dbID,
		}
	}
	dbData, err := json.Marshal(dbJSON)
	if err != nil {
		t.Fatalf("marshal database.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}

	// Create artifacts
	for relPath, content := range artifacts {
		fullPath := filepath.Join(dbDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir artifact dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}
}

// createStatsTestManifest writes an index_manifest.json with checksums for the given artifacts.
func createStatsTestManifest(t *testing.T, dbDir string, artifacts map[string]string) {
	t.Helper()
	checksums := make(map[string]string, len(artifacts))
	for relPath, content := range artifacts {
		fullPath := filepath.Join(dbDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir artifact dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		sum, err := index.ComputeChecksum(fullPath)
		if err != nil {
			t.Fatalf("compute checksum: %v", err)
		}
		checksums[relPath] = sum
	}

	manifest := &index.IndexManifest{
		SchemaVersionHash: "test-hash",
		DBVersion:         "mysql",
		TableCount:        5,
		ArtifactChecksums: checksums,
	}
	if err := index.WriteIndexManifest(dbDir, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// Test health section is present in response.
func TestRunStats_Health_Present(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupStatsTestDB(t, home, "db-1", "mysql", map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})
	createStatsTestManifest(t, filepath.Join(home, ".querylex", "db-1"), map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	resp := RunStats()
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}
	if resp.Data.Health == nil {
		t.Fatal("expected Health field to be non-nil")
	}
	if len(resp.Data.Health.Databases) != 1 {
		t.Fatalf("expected 1 database in health, got %d", len(resp.Data.Health.Databases))
	}
	if resp.Data.Health.Databases[0].DatabaseID != "db-1" {
		t.Fatalf("expected DatabaseID='db-1', got '%s'", resp.Data.Health.Databases[0].DatabaseID)
	}
}

// Test artifact state detection: present artifacts show as "present".
func TestRunStats_Health_ArtifactPresent(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupStatsTestDB(t, home, "db-1", "mysql", map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})
	createStatsTestManifest(t, filepath.Join(home, ".querylex", "db-1"), map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	artifacts := resp.Data.Health.Databases[0].Artifacts
	if artifacts["schema/schema.json"] != artifactStatePresent {
		t.Fatalf("expected schema/schema.json to be 'present', got '%s'", artifacts["schema/schema.json"])
	}

	// Missing artifacts (not created) should be "missing"
	if artifacts["schema/join_graph.json"] != artifactStateMissing {
		t.Fatalf("expected schema/join_graph.json to be 'missing', got '%s'", artifacts["schema/join_graph.json"])
	}
}

// Test stale artifact detection: checksum mismatch shows as "stale".
func TestRunStats_Health_ArtifactStale(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	dbDir := filepath.Join(home, ".querylex", "db-1")
	setupStatsTestDB(t, home, "db-1", "mysql", map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})
	// Build manifest with current content
	createStatsTestManifest(t, dbDir, map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	// Modify artifact after manifest was written — checksum will differ
	schemaPath := filepath.Join(dbDir, "schema", "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"tables":[{"name":"modified"}]}`), 0644); err != nil {
		t.Fatalf("modify artifact: %v", err)
	}

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	artifacts := resp.Data.Health.Databases[0].Artifacts
	if artifacts["schema/schema.json"] != artifactStateStale {
		t.Fatalf("expected schema/schema.json to be 'stale' after modification, got '%s'", artifacts["schema/schema.json"])
	}

	// Verify warning is emitted for stale artifacts
	foundStaleWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "DATABASE_ARTIFACTS_STALE" {
			foundStaleWarning = true
			break
		}
	}
	if !foundStaleWarning {
		t.Fatal("expected DATABASE_ARTIFACTS_STALE warning for stale artifacts")
	}
}

// Test credential status detection.
func TestRunStats_Health_CredentialStatus(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-mysql", Name: "MySQLDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
		{ID: "db-sqlite", Name: "SQLiteDB", Type: "sqlite", Status: state.StatusIndexed, IndexingProgress: 100},
		{ID: "db-nocred", Name: "NoCredDB", Type: "mysql", Status: state.StatusNotIndexed, IndexingProgress: 0},
	})
	t.Setenv("HOME", home)

	// MySQL with credential ref and env var set → "available"
	t.Setenv("QUERYLEX_DB_PASSWORD", "test-password")
	setupStatsTestDB(t, home, "db-mysql", "mysql", nil)

	// SQLite → "not_required"
	setupStatsTestDB(t, home, "db-sqlite", "sqlite", nil)

	// MySQL with no database.json → "missing"
	// Don't create the db dir for db-nocred

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	// Map by DatabaseID for easier assertion
	byID := make(map[string]DatabaseHealth)
	for _, h := range resp.Data.Health.Databases {
		byID[h.DatabaseID] = h
	}

	if h, ok := byID["db-mysql"]; !ok {
		t.Fatal("expected db-mysql in health")
	} else if h.CredentialStatus != "available" {
		t.Fatalf("expected db-mysql credential_status='available', got '%s'", h.CredentialStatus)
	}

	if h, ok := byID["db-sqlite"]; !ok {
		t.Fatal("expected db-sqlite in health")
	} else if h.CredentialStatus != "not_required" {
		t.Fatalf("expected db-sqlite credential_status='not_required', got '%s'", h.CredentialStatus)
	}

	if h, ok := byID["db-nocred"]; !ok {
		t.Fatal("expected db-nocred in health")
	} else if h.CredentialStatus != "missing" {
		t.Fatalf("expected db-nocred credential_status='missing', got '%s'", h.CredentialStatus)
	}
}

// Test placeholder fields (MemoryIndexState, ExplainCacheSummary).
func TestRunStats_Health_Placeholders(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupStatsTestDB(t, home, "db-1", "mysql", nil)

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	dbHealth := resp.Data.Health.Databases[0]
	if dbHealth.MemoryIndexState != "not_implemented" {
		t.Fatalf("expected MemoryIndexState='not_implemented', got '%s'", dbHealth.MemoryIndexState)
	}
	if dbHealth.ExplainCacheSummary != "not_implemented" {
		t.Fatalf("expected ExplainCacheSummary='not_implemented', got '%s'", dbHealth.ExplainCacheSummary)
	}
}

// Test empty workspace: health section present with empty databases array.
func TestRunStats_Health_EmptyWorkspace(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{})
	t.Setenv("HOME", home)

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}
	if resp.Data.Health == nil {
		t.Fatal("expected Health field to be non-nil for empty workspace")
	}
	if len(resp.Data.Health.Databases) != 0 {
		t.Fatalf("expected 0 databases in health for empty workspace, got %d", len(resp.Data.Health.Databases))
	}
}

// Test database with not_indexed status: all artifacts show as "missing" (no checksum logic).
func TestRunStats_Health_NotIndexedArtifacts(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusNotIndexed, IndexingProgress: 0},
	})
	t.Setenv("HOME", home)

	// Create artifacts but no manifest
	setupStatsTestDB(t, home, "db-1", "mysql", map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	// For not_indexed, artifacts that exist show as "present"
	artifacts := resp.Data.Health.Databases[0].Artifacts
	if artifacts["schema/schema.json"] != artifactStatePresent {
		t.Fatalf("expected schema/schema.json to be 'present' (not_indexed only checks existence), got '%s'", artifacts["schema/schema.json"])
	}
	// Missing artifacts show as "missing"
	if artifacts["domain_map.json"] != artifactStateMissing {
		t.Fatalf("expected domain_map.json to be 'missing', got '%s'", artifacts["domain_map.json"])
	}
}

// Test response format: Response[StatsData] envelope.
func TestRunStats_Health_ResponseFormat(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupStatsTestDB(t, home, "db-1", "mysql", map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})
	createStatsTestManifest(t, filepath.Join(home, ".querylex", "db-1"), map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	resp := RunStats()
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	// Verify response envelope fields
	if resp.Meta.TraceID == "" {
		t.Fatal("expected non-empty trace_id")
	}
	if resp.Meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version='1.0.0', got '%s'", resp.Meta.ProtocolVersion)
	}

	// Verify StatsData structure
	data := resp.Data
	if data.Health == nil {
		t.Fatal("expected Health field in StatsData")
	}
	if len(data.ConnectedDatabases) != 1 {
		t.Fatalf("expected 1 connected database, got %d", len(data.ConnectedDatabases))
	}
}

// Test multiple databases each get their own health entry.
func TestRunStats_Health_MultipleDatabases(t *testing.T) {
	home := setupStatsTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-a", Name: "Database A", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
		{ID: "db-b", Name: "Database B", Type: "postgres", Status: state.StatusNotIndexed, IndexingProgress: 0},
	})
	t.Setenv("HOME", home)

	setupStatsTestDB(t, home, "db-a", "mysql", nil)
	setupStatsTestDB(t, home, "db-b", "postgres", nil)

	resp := RunStats()
	if !resp.Success {
		t.Fatalf("expected success, got error: %+v", resp.Error)
	}

	if len(resp.Data.Health.Databases) != 2 {
		t.Fatalf("expected 2 databases in health, got %d", len(resp.Data.Health.Databases))
	}

	byID := make(map[string]DatabaseHealth)
	for _, h := range resp.Data.Health.Databases {
		byID[h.DatabaseID] = h
	}

	if _, ok := byID["db-a"]; !ok {
		t.Fatal("expected db-a in health")
	}
	if _, ok := byID["db-b"]; !ok {
		t.Fatal("expected db-b in health")
	}

	if byID["db-a"].Status != string(state.StatusIndexed) {
		t.Fatalf("expected db-a status='indexed', got '%s'", byID["db-a"].Status)
	}
	if byID["db-b"].Status != string(state.StatusNotIndexed) {
		t.Fatalf("expected db-b status='not_indexed', got '%s'", byID["db-b"].Status)
	}
}
