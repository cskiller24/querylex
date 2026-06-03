//go:build e2e

package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestSQLiteSchema is a full vertical slice E2E test that:
//  1. Connects to SQLite via ConnectSQLite (temp file, no Docker)
//  2. Loads the Chinook schema
//  3. Sets up a synthetic querylex workspace pointing to the SQLite file DB
//  4. Runs querylex schema --table Album as a subprocess
//  5. Verifies exit code 0 and JSON success response
//  6. Verifies the response contains the Album table with AlbumId,
//     Title, and ArtistId columns
func TestSQLiteSchema(t *testing.T) {
	db := testhelper.ConnectSQLite(t)

	// Load Chinook schema into the SQLite database
	loadChinookSchema(t, db)

	// Get the database file path for workspace config
	dbPath := getDatabasePath(t, db)

	// Set up workspace pointing to the SQLite file
	setupE2EWorkspaceSQLite(t, dbPath)

	// Run querylex schema --table Album
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "schema", "--table", "Album")

	// Assert exit code 0
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s",
			exitCode, stdout, stderr)
	}

	// Stderr should be empty or contain only warnings
	if stderr != "" && !strings.Contains(stderr, "Warning:") {
		t.Logf("stderr contains non-warning output: %s", stderr)
	}

	// Parse JSON response
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON in stdout: %v\nstdout: %s", err, stdout)
	}

	// Assert success field
	success, ok := resp["success"].(bool)
	if !ok {
		t.Fatalf("response missing 'success' field: %s", stdout)
	}
	if !success {
		errDetail, _ := resp["error"].(map[string]interface{})
		errCode, _ := errDetail["code"].(string)
		errMsg, _ := errDetail["message"].(string)
		t.Fatalf("expected success=true, got false (code=%s, message=%s)",
			errCode, errMsg)
	}

	// Verify response metadata
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'meta' field")
	}
	if meta["protocol_version"] != "1.0.0" {
		t.Errorf("expected protocol_version=1.0.0, got %v", meta["protocol_version"])
	}
	if _, ok := meta["trace_id"]; !ok {
		t.Errorf("response missing trace_id")
	}

	// Verify data contains Album table
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'data' field")
	}
	tablesRaw, ok := data["tables"].([]interface{})
	if !ok {
		t.Fatalf("data missing 'tables' array")
	}

	// Find the Album table in the response
	var albumTable map[string]interface{}
	for _, tbl := range tablesRaw {
		tblMap, ok := tbl.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := tblMap["name"].(string); ok && name == "Album" {
			albumTable = tblMap
			break
		}
	}
	if albumTable == nil {
		t.Fatalf("Album table not found in schema response: %+v", tablesRaw)
	}

	// Verify Album table has expected columns
	columnsRaw, ok := albumTable["columns"].([]interface{})
	if !ok {
		t.Fatalf("Album table missing 'columns' array")
	}

	expectedColumns := map[string]bool{
		"AlbumId":  false,
		"Title":    false,
		"ArtistId": false,
	}
	for _, col := range columnsRaw {
		colMap, ok := col.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := colMap["name"].(string); ok {
			if _, found := expectedColumns[name]; found {
				expectedColumns[name] = true
			}
		}
	}

	for col, found := range expectedColumns {
		if !found {
			t.Errorf("expected column %q not found in Album table", col)
		}
	}
}

// getDatabasePath extracts the SQLite database file path from a connection
// by querying PRAGMA database_list, which returns rows (seq, name, file).
// The "main" database entry contains the file path in the third column.
func getDatabasePath(t *testing.T, db *sql.DB) string {
	t.Helper()
	var seq int
	var name, filePath string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &filePath); err != nil {
		t.Fatalf("query PRAGMA database_list: %v", err)
	}
	return filePath
}
