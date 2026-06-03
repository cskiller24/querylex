package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
)

// setupPreflightTestWorkspace creates a test workspace and database directory for
// PreflightForCommand tests. Returns the home directory path (a temp dir).
func setupPreflightTestWorkspace(t *testing.T, entries []state.DatabaseEntry) string {
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

// setupPreflightTestDB creates a database directory for preflight tests with a
// valid database.json so step 4 does not fail.
func setupPreflightTestDB(t *testing.T, home, dbID, dbType string) {
	t.Helper()
	dbDir := filepath.Join(home, ".querylex", dbID)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

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
	dbData, err := json.Marshal(dbJSON)
	if err != nil {
		t.Fatalf("marshal database.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}
}

// setupPreflightTestManifest creates an index_manifest.json with checksums computed
// from the given artifact content map.
func setupPreflightTestManifest(t *testing.T, dbDir string, artifacts map[string]string) {
	t.Helper()
	checksums := make(map[string]string, len(artifacts))
	for relPath, content := range artifacts {
		fullPath := filepath.Join(dbDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		sum, err := index.ComputeChecksum(fullPath)
		if err != nil {
			t.Fatalf("checksum: %v", err)
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

// loadWorkspaceStatus is a test helper that reads the workspace file from disk
// and returns the status of the given database ID.
func loadWorkspaceStatus(t *testing.T, home, dbID string) state.DatabaseStatus {
	t.Helper()
	wsPath := filepath.Join(home, ".querylex", "querylex.json")
	data, err := os.ReadFile(wsPath)
	if err != nil {
		t.Fatalf("read workspace: %v", err)
	}
	var ws state.Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		t.Fatalf("unmarshal workspace: %v", err)
	}
	for _, entry := range ws.ConnectedDatabases {
		if entry.ID == dbID {
			return entry.Status
		}
	}
	t.Fatalf("database %s not found in workspace", dbID)
	return ""
}

// Test stale detection when manifest checksums match — status stays "indexed",
// preflight proceeds to connection step (expected error: CONNECTION_FAILED).
func TestStaleDetection_NoMismatch(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	dbDir := filepath.Join(home, ".querylex", "db-1")
	setupPreflightTestDB(t, home, "db-1", "mysql")
	setupPreflightTestManifest(t, dbDir, map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response (connection will fail), got nil")
	}
	if errResp.Success {
		t.Fatal("expected preflight to fail (no real database to connect to)")
	}
	if errResp.Error.Code != format.ErrCodeConnectionFailed {
		t.Fatalf("expected CONNECTION_FAILED error (preflight should proceed past stale detection and fail at connect), got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}

	// Status should remain "indexed" (no mismatch detected)
	status := loadWorkspaceStatus(t, home, "db-1")
	if status != state.StatusIndexed {
		t.Fatalf("expected status to remain 'indexed', got '%s'", status)
	}
}

// Test stale detection when a checksum mismatch is found — status updated to "stale".
func TestStaleDetection_Mismatch(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	dbDir := filepath.Join(home, ".querylex", "db-1")
	setupPreflightTestDB(t, home, "db-1", "mysql")
	// Create manifest with original checksums
	setupPreflightTestManifest(t, dbDir, map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	// Modify artifact after manifest was written — checksum will differ
	schemaPath := filepath.Join(dbDir, "schema", "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"tables":[{"name":"modified"}]}`), 0644); err != nil {
		t.Fatalf("modify artifact: %v", err)
	}

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response (connection will fail), got nil")
	}
	if errResp.Success {
		t.Fatal("expected preflight to fail (no real database)")
	}

	// Status should be updated to "stale"
	status := loadWorkspaceStatus(t, home, "db-1")
	if status != state.StatusStale {
		t.Fatalf("expected status to be updated to 'stale', got '%s'", status)
	}

	// The error should be CONNECTION_FAILED (not WORKSPACE_STATE_INVALID), because
	// stale detection is non-blocking — it updates status but lets preflight continue.
	if errResp.Error.Code != format.ErrCodeConnectionFailed {
		t.Fatalf("expected CONNECTION_FAILED (stale detection is non-blocking), got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}
}

// Test stale detection is skipped for non-indexed status.
func TestStaleDetection_SkippedForNotIndexed(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusNotIndexed, IndexingProgress: 0},
	})
	t.Setenv("HOME", home)

	setupPreflightTestDB(t, home, "db-1", "mysql")

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response, got nil")
	}

	// For not_indexed, preflight should fail at step 3 with WORKSPACE_STATE_INVALID
	if errResp.Error.Code != format.ErrCodeWorkspaceStateInvalid {
		t.Fatalf("expected WORKSPACE_STATE_INVALID for not_indexed database, got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}

	// Status should remain "not_indexed" (stale detection was skipped)
	status := loadWorkspaceStatus(t, home, "db-1")
	if status != state.StatusNotIndexed {
		t.Fatalf("expected status to remain 'not_indexed', got '%s'", status)
	}
}

// Test stale detection when indexed database has no manifest file.
func TestStaleDetection_NoManifest(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupPreflightTestDB(t, home, "db-1", "mysql")
	// Create an artifact but no manifest
	schemaDir := filepath.Join(home, ".querylex", "db-1", "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("mkdir schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte(`{"tables":[]}`), 0644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response, got nil")
	}

	// Status should be updated to "stale" (missing manifest = stale)
	status := loadWorkspaceStatus(t, home, "db-1")
	if status != state.StatusStale {
		t.Fatalf("expected status to be updated to 'stale' (missing manifest), got '%s'", status)
	}

	// The error should be CONNECTION_FAILED (not WORKSPACE_STATE_INVALID) because
	// stale detection is non-blocking even for missing manifest.
	if errResp.Error.Code != format.ErrCodeConnectionFailed {
		t.Fatalf("expected CONNECTION_FAILED (stale detection is non-blocking), got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}
}

// Test stale detection error format follows the Response envelope.
func TestStaleDetection_ErrorFormat(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	setupPreflightTestDB(t, home, "db-1", "mysql")
	// Create manifest with a matching artifact
	dbDir := filepath.Join(home, ".querylex", "db-1")
	setupPreflightTestManifest(t, dbDir, map[string]string{
		"schema/schema.json": `{"tables":[]}`,
	})

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response, got nil")
	}
	if errResp.Success {
		t.Fatal("expected success=false")
	}
	if errResp.Meta.TraceID == "" {
		t.Fatal("expected non-empty trace_id")
	}
	if errResp.Meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version='1.0.0', got '%s'", errResp.Meta.ProtocolVersion)
	}
}

// Test that stale detection crashes gracefully when the artifact directory doesn't exist.
func TestStaleDetection_MissingArtifactDir(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)

	// Create database.json but no artifact directory at all
	dbDir := filepath.Join(home, ".querylex", "db-1")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbJSON := DBConfigJSON{
		ID:       "db-1",
		Name:     "TestDB",
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "testuser",
		SSLMode:  "require",
	}
	dbData, _ := json.Marshal(dbJSON)
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response, got nil")
	}

	// Missing artifact dir means manifest is nil → status should be updated to "stale"
	status := loadWorkspaceStatus(t, home, "db-1")
	if status != state.StatusStale {
		t.Fatalf("expected status to be 'stale' (missing artifact dir), got '%s'", status)
	}
}

// Test that PreflightForCommand auto-unlocks the EncryptedFileStore when
// QUERYLEX_KEYCHAIN_PASSPHRASE is set. Without the auto-unlock, credential
// retrieval would fail with CREDENTIAL_UNAVAILABLE because the EncryptedFileStore
// instance created by SelectCredentialStore() has no passphrase set.
func TestPreflight_AutoUnlockEncryptedStore(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)
	t.Setenv("QUERYLEX_KEYCHAIN_PASSPHRASE", "test-passphrase")

	// Pre-populate the encrypted credential store with the test password.
	encPath := filepath.Join(home, ".querylex", "credentials.json.enc")
	credStore := credentials.NewEncryptedFileStore(encPath)
	if err := credStore.Unlock("test-passphrase"); err != nil {
		t.Fatalf("setup: unlock failed: %v", err)
	}
	ref, err := credStore.Store("db-1", "testpass")
	if err != nil {
		t.Fatalf("setup: store failed: %v", err)
	}

	// Create database.json WITH credential reference so preflight tries to
	// Retrieve() from the encrypted store.
	dbDir := filepath.Join(home, ".querylex", "db-1")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	dbJSON := DBConfigJSON{
		ID:            "db-1",
		Name:          "TestDB",
		Type:          "mysql",
		Host:          "localhost",
		Port:          3306,
		Database:      "testdb",
		Username:      "testuser",
		SSLMode:       "require",
		CredentialRef: ref,
	}
	dbData, err := json.Marshal(dbJSON)
	if err != nil {
		t.Fatalf("marshal database.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response (connection will fail), got nil")
	}
	// With auto-unlock, credential retrieval succeeds and preflight reaches
	// the connection step, which fails with CONNECTION_FAILED (no real MySQL).
	// Without auto-unlock, preflight fails earlier with CREDENTIAL_UNAVAILABLE.
	if errResp.Error.Code != format.ErrCodeConnectionFailed {
		t.Fatalf("expected CONNECTION_FAILED (credential should be auto-unlocked), got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}
}

// Test that when QUERYLEX_KEYCHAIN_PASSPHRASE is NOT set, preflight behavior
// is unchanged — credential retrieval still works via existing env store path
// or fails with the original error.
func TestPreflight_AutoUnlock_NoPassphraseEnv(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
	})
	t.Setenv("HOME", home)
	// Intentionally NOT setting QUERYLEX_KEYCHAIN_PASSPHRASE

	setupPreflightTestDB(t, home, "db-1", "mysql")

	_, errResp := PreflightForCommand()
	if errResp == nil {
		t.Fatal("expected error response (connection will fail), got nil")
	}
	// Without QUERYLEX_KEYCHAIN_PASSPHRASE, the auto-unlock is skipped.
	// Preflight should still fail with CONNECTION_FAILED (the existing behavior:
	// no credential ref means empty password, DSN built, connection fails).
	if errResp.Error.Code != format.ErrCodeConnectionFailed {
		t.Fatalf("expected CONNECTION_FAILED (existing behavior preserved), got %s: %s",
			errResp.Error.Code, errResp.Error.Message)
	}
}
