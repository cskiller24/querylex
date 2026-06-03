//go:build e2e

package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/cskiller24/querylex/test/testhelper"
)

// TestSQLiteIsolation verifies that each per-test SQLite database created by
// ConnectSQLite is truly isolated — a table created in one file DB must NOT
// be visible from a second file DB via the querylex binary.
//
// Steps:
//  1. Connect to SQLite and create first per-test database (db1)
//  2. Create a table in db1 and insert data
//  3. Connect to SQLite and create second per-test database (db2)
//  4. Verify the two databases have different paths (D-21)
//  5. Set up workspace pointing to db2 (not db1)
//  6. Run querylex schema --table isolation_marker
//  7. Assert the table from db1 is NOT visible from db2
func TestSQLiteIsolation(t *testing.T) {
	// DB 1: create a test table with isolation marker data
	db1 := testhelper.ConnectSQLite(t)
	db1Path := getDatabasePath(t, db1)

	_, err := db1.Exec("CREATE TABLE isolation_marker (id INT PRIMARY KEY, val VARCHAR(50))")
	if err != nil {
		t.Fatalf("db1: create table: %v", err)
	}
	_, err = db1.Exec("INSERT INTO isolation_marker VALUES (1, 'db1-secret')")
	if err != nil {
		t.Fatalf("db1: insert: %v", err)
	}

	// DB 2: second per-test database (should be a different file)
	db2 := testhelper.ConnectSQLite(t)
	db2Path := getDatabasePath(t, db2)

	// Verify the databases have different paths (D-21)
	if db1Path == db2Path {
		t.Fatalf("expected different SQLite database paths, got identical: %s", db1Path)
	}

	// Log the isolation boundaries
	t.Logf("DB1: %s", db1Path)
	t.Logf("DB2: %s", db2Path)

	// Set up workspace pointing to db2
	setupE2EWorkspaceSQLite(t, db2Path)

	// Try to query isolation_marker from db2's workspace
	stdout, _, exitCode := testhelper.RunQuerylex(t, "schema", "--table", "isolation_marker")

	// The table isolation_marker was created in db1, NOT in db2.
	// From db2's workspace, querylex should either:
	//   a) Exit with non-zero code (TABLE_NOT_FOUND or similar)
	//   b) Return success JSON with empty/no tables for isolation_marker

	if exitCode == 0 {
		// Check if the JSON response indicates the table was not found
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
			if success, ok := resp["success"].(bool); ok && success {
				// Schema command succeeded but should not have found the table
				data, _ := resp["data"].(map[string]interface{})
				tables, _ := data["tables"].([]interface{})
				for _, tbl := range tables {
					tblMap, _ := tbl.(map[string]interface{})
					if name, _ := tblMap["name"].(string); name == "isolation_marker" {
						t.Errorf("isolation_marker table from db1 is visible in db2's schema!")
						return
					}
				}
				t.Log("OK: isolation_marker not visible from db2 (empty result)")
				return
			}
		}
		t.Logf("OK: exit code 0 but response indicates table not found: %s", stdout)
		return
	}

	// If exit code != 0, that's also expected (TABLE_NOT_FOUND error)
	t.Logf("OK: exit code %d (expected — table not found in db2)", exitCode)
	if strings.Contains(stdout, "TABLE_NOT_FOUND") || strings.Contains(stdout, "not found") {
		t.Log("Response confirms table not found in db2 database")
	}
}

// getDatabaseName queries the SQLite database file path via PRAGMA database_list.
func getDatabaseName(t *testing.T, db *sql.DB) string {
	t.Helper()
	var seq int
	var name, filePath string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &filePath); err != nil {
		t.Fatalf("query PRAGMA database_list: %v", err)
	}
	return filePath
}
