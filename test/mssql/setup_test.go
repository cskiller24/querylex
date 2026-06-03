//go:build e2e

package mssql

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
)

// setupE2EWorkspace creates a synthetic querylex workspace in a temp directory
// and sets environment variables so the querylex binary resolves credentials
// via EncryptedFileStore auto-unlocked by QUERYLEX_KEYCHAIN_PASSPHRASE.
//
// Steps:
//  1. Creates t.TempDir() with .querylex/ directory
//  2. Writes workspace.json: one DatabaseEntry with status "indexed"
//  3. Writes database.json with a CredentialReference to encrypted file
//  4. Pre-populates the encrypted credential store with "testpass"
//  5. Writes minimal indexing artifacts (schema.json + index_manifest.json)
//  6. Sets HOME, QUERYLEX_DB_PASSWORD, QUERYLEX_KEYCHAIN_PASSPHRASE env vars
//
// Returns the home directory path.
func setupE2EWorkspace(t *testing.T, host string, port int, dbName string) string {
	t.Helper()

	dbID := "e2e-test-db"
	home := t.TempDir()
	wsDir := filepath.Join(home, ".querylex")

	// 1. Create .querylex directory
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir .querylex: %v", err)
	}

	// 2. Write workspace.json
	activeDBID := dbID
	ws := &state.Workspace{
		ConnectedDatabases: []state.DatabaseEntry{
			{ID: dbID, Name: dbName, Type: "mssql", Status: state.StatusIndexed, IndexingProgress: 100},
		},
		ActiveDatabaseID: &activeDBID,
	}
	wsData, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("marshal workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write workspace.json: %v", err)
	}

	// 3. Pre-populate the encrypted credential store
	encPath := filepath.Join(wsDir, "credentials.json.enc")
	encStore := credentials.NewEncryptedFileStore(encPath)
	if err := encStore.Unlock("e2e-test-passphrase"); err != nil {
		t.Fatalf("encrypted store unlock: %v", err)
	}
	credRef, err := encStore.Store(dbID, "testpass")
	if err != nil {
		t.Fatalf("encrypted store store: %v", err)
	}

	// 4. Write database.json with credential reference
	dbDir := filepath.Join(wsDir, dbID)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	dbCfg := map[string]interface{}{
		"id":                  dbID,
		"name":                dbName,
		"type":                "mssql",
		"host":                host,
		"port":                port,
		"database":            dbName,
		"username":            "sa",
		"ssl_mode":            "disable",
		"credential_reference": credRef,
	}
	dbData, err := json.Marshal(dbCfg)
	if err != nil {
		t.Fatalf("marshal database.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}

	// 5. Create minimal indexing artifacts for preflight gating
	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	schemaData := map[string]interface{}{
		"tables": []interface{}{},
	}
	schemaJSON, err := json.Marshal(schemaData)
	if err != nil {
		t.Fatalf("marshal schema.json: %v", err)
	}
	schemaPath := filepath.Join(schemaDir, "schema.json")
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		t.Fatalf("write schema.json: %v", err)
	}

	// Compute checksum for the artifact and write index manifest
	schemaChecksum, err := index.ComputeChecksum(schemaPath)
	if err != nil {
		t.Fatalf("compute schema checksum: %v", err)
	}
	manifest := &index.IndexManifest{
		SchemaVersionHash: "e2e-test-hash",
		DBVersion:         "mssql",
		TableCount:        0,
		ArtifactChecksums: map[string]string{
			"schema/schema.json": schemaChecksum,
		},
	}
	if err := index.WriteIndexManifest(dbDir, manifest); err != nil {
		t.Fatalf("write index manifest: %v", err)
	}

	// 6. Set environment variables
	t.Setenv("HOME", home)
	t.Setenv("QUERYLEX_DB_PASSWORD", "testpass")
	t.Setenv("QUERYLEX_KEYCHAIN_PASSPHRASE", "e2e-test-passphrase")

	return home
}

// loadNorthwindSchema reads Northwind DDL from the cached extraction directory,
// extracts DDL statements (CREATE TABLE, INSERT INTO), and executes them against
// the given *sql.DB. Northwind DDL files may contain GO batch separators that
// are not valid SQL for database/sql — this function splits on GO first, then
// on semicolons, filtering out USE, CREATE DATABASE, and DROP DATABASE statements.
func loadNorthwindSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	cacheDir := filepath.Join("test", "testdata", "cache", "northwind-extracted")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read northwind-extracted dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		// Split on GO batch separators first (MSSQL-specific, not valid SQL for database/sql)
		goBatches := strings.Split(string(content), "\nGO\n")
		for _, batch := range goBatches {
			batch = strings.TrimSpace(batch)
			if batch == "" {
				continue
			}

			// Then split on semicolons
			statements := strings.Split(batch, ";")
			for _, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}

				// Skip comment-only lines
				if strings.HasPrefix(stmt, "--") {
					continue
				}

				// Skip DROP DATABASE / CREATE DATABASE / USE (operates on the whole server)
				upper := strings.ToUpper(stmt)
				if strings.HasPrefix(upper, "DROP DATABASE") ||
					strings.HasPrefix(upper, "CREATE DATABASE") ||
					strings.HasPrefix(upper, "USE ") {
					continue
				}

				// Execute remaining DDL / DML
				if _, err := db.Exec(stmt); err != nil {
					t.Fatalf("execute DDL: %v\nSQL: %.1000s", err, stmt)
				}
			}
		}
	}
}
