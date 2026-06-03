//go:build e2e

package mysql

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
			{ID: dbID, Name: dbName, Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
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
		"type":                "mysql",
		"host":                host,
		"port":                port,
		"database":            dbName,
		"username":            "root",
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
		DBVersion:         "mysql",
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

// loadEmployeesDB loads the employees schema and small data tables (departments,
// dept_manager) from cached dump files. It reuses loadEmployeesSchema for DDL
// creation, then loads dump files for small data tables that validation tests
// depend on. Larger tables (employees, dept_emp, titles, salaries) are not
// loaded to keep test runtime reasonable.
//
// If dump files are missing (cache not populated), the function loads schema
// only and issues a warning — tests degrade gracefully to schema-only mode.
func loadEmployeesDB(t *testing.T, db *sql.DB) {
	t.Helper()

	// First load schema (6 tables + 2 views) — reuses existing DDL logic
	loadEmployeesSchema(t, db)

	// Then load small data tables from dump files
	cacheDir := filepath.Join("test", "testdata", "cache", "test_db-extracted", "test_db-master")

	// Load departments (9 rows)
	loadDumpFile(t, db, filepath.Join(cacheDir, "load_departments.dump"), "departments")

	// Load dept_manager (24 rows)
	loadDumpFile(t, db, filepath.Join(cacheDir, "load_dept_manager.dump"), "dept_manager")
}

// loadDumpFile reads a dump file containing batch INSERT statements, splits on
// semicolon+newline, and executes each non-empty statement against the DB.
// If the file does not exist, it logs a warning and returns without error —
// the test continues with schema-only data.
func loadDumpFile(t *testing.T, db *sql.DB, path, tableName string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Logf("warning: dump file %s not found — %s data skipped (schema-only mode)", path, tableName)
		return
	}

	t.Logf("loading %s data from %s (%d bytes)", tableName, path, len(content))

	// The dump files use batch INSERT syntax with semicolon-terminated statements.
	// Splitting on ";\n" yields complete statements; trailing empty element after
	// final newline is filtered out in the loop.
	statements := strings.Split(string(content), ";\n")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("load %s data: %v\nSQL: %.1000s", tableName, err, stmt)
		}
	}

	t.Logf("loaded %s data successfully", tableName)
}

// loadEmployeesSchema reads the Employees DB SQL from the cached download at
// test/testdata/cache/test_db-extracted/test_db-master/employees.sql, extracts
// DDL statements (CREATE TABLE, CREATE VIEW), and executes them against the
// given *sql.DB. This loads only the schema (6 tables + 2 views), not the
// 3.9M rows of data — sufficient for schema extraction and validation tests.
//
// The employees.sql file contains MySQL source commands and version-specific
// comments that are not valid for database/sql. This function filters them out:
//   - Skips lines starting with "source" (data loading commands)
//   - Skips /*!...*/ version-specific comments
//   - Skips "flush binary logs" and info SELECT statements
//   - Skips DROP DATABASE / CREATE DATABASE / USE (operates on test's DB)
func loadEmployeesSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	path := filepath.Join("test", "testdata", "cache", "test_db-extracted", "test_db-master", "employees.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read employees.sql: %v", err)
	}

	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Skip comment-only lines
		if strings.HasPrefix(stmt, "--") || strings.HasPrefix(stmt, "#") {
			continue
		}

		// Skip MySQL version-specific comments
		if strings.HasPrefix(stmt, "/*!") {
			continue
		}

		// Skip source commands (data loading from dump files)
		if strings.HasPrefix(stmt, "source") || strings.HasPrefix(stmt, "SOURCE") {
			continue
		}

		// Skip DROP DATABASE / CREATE DATABASE / USE (operates on the whole server)
		upper := strings.ToUpper(stmt)
		if strings.HasPrefix(upper, "DROP DATABASE") ||
			strings.HasPrefix(upper, "CREATE DATABASE") ||
			strings.HasPrefix(upper, "USE ") {
			continue
		}

		// Skip flush binary logs and info SELECT statements
		if strings.HasPrefix(upper, "FLUSH") {
			continue
		}
		if strings.HasPrefix(upper, "SELECT") && strings.Contains(upper, "INFO") {
			continue
		}

		// Execute remaining DDL (CREATE TABLE, CREATE VIEW, DROP TABLE)
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("execute DDL: %v\nSQL: %.1000s", err, stmt)
		}
	}
}
