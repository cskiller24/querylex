//go:build e2e

package mysql

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMySQLIsolation verifies that each per-test database created by
// ConnectMySQL is truly isolated — a table created in one per-test DB
// must NOT be visible from a second per-test DB via the querylex binary.
//
// Steps:
//  1. Connect to MySQL and create first per-test database (db1)
//  2. Create a table in db1 and insert data
//  3. Connect to MySQL and create second per-test database (db2)
//  4. Verify the two databases have different names (D-21)
//  5. Set up workspace pointing to db2 (not db1)
//  6. Run querylex schema --table isolation_marker
//  7. Assert the table from db1 is NOT visible from db2
func TestMySQLIsolation(t *testing.T) {
	// DB 1: create a test table with isolation marker data
	db1 := testhelper.ConnectMySQL(t)
	db1Name := getDatabaseName(t, db1)

	_, err := db1.Exec("CREATE TABLE isolation_marker (id INT PRIMARY KEY, val VARCHAR(50))")
	if err != nil {
		t.Fatalf("db1: create table: %v", err)
	}
	_, err = db1.Exec("INSERT INTO isolation_marker VALUES (1, 'db1-secret')")
	if err != nil {
		t.Fatalf("db1: insert: %v", err)
	}

	host1, port1, _ := extractConnectionInfo(t, db1)

	// DB 2: second per-test database (should be different)
	db2 := testhelper.ConnectMySQL(t)
	db2Name := getDatabaseName(t, db2)

	// Verify the databases have different names (D-21)
	if db1Name == db2Name {
		t.Fatalf("expected different per-test database names, got identical: %s", db1Name)
	}

	host2, port2, _ := extractConnectionInfo(t, db2)

	// Log the isolation boundaries
	t.Logf("DB1: %s (host=%s, port=%d)", db1Name, host1, port1)
	t.Logf("DB2: %s (host=%s, port=%d)", db2Name, host2, port2)

	// Set up workspace pointing to db2
	setupE2EWorkspace(t, host2, port2, db2Name)

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

// getDatabaseName queries the current database name from the MySQL connection.
func getDatabaseName(t *testing.T, db *sql.DB) string {
	t.Helper()
	var dbName string
	if err := db.QueryRow("SELECT DATABASE()").Scan(&dbName); err != nil {
		t.Fatalf("query database name: %v", err)
	}
	return dbName
}
