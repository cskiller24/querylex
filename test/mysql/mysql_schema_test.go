//go:build e2e

package mysql

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMySQLSchema is a full vertical slice E2E test that:
//  1. Connects to live MySQL and creates a per-test e2e_* database
//  2. Loads the Employees DB schema (6 tables + 2 views)
//  3. Sets up a synthetic querylex workspace pointing to the per-test DB
//  4. Runs querylex schema --table employees as a subprocess
//  5. Verifies exit code 0 and JSON success response
//  6. Verifies the response contains the employees table with emp_no,
//     first_name, and last_name columns
func TestMySQLSchema(t *testing.T) {
	db := testhelper.ConnectMySQL(t)

	// Load Employees DB schema into the per-test database
	loadEmployeesSchema(t, db)

	// Extract connection info from the live MySQL connection
	host, port, dbName := extractConnectionInfo(t, db)

	// Set up workspace pointing to the per-test database
	setupE2EWorkspace(t, host, port, dbName)

	// Run querylex schema --table employees
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "schema", "--table", "employees")

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

	// Verify data contains employees table
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'data' field")
	}
	tablesRaw, ok := data["tables"].([]interface{})
	if !ok {
		t.Fatalf("data missing 'tables' array")
	}

	// Find the employees table in the response
	var employeesTable map[string]interface{}
	for _, tbl := range tablesRaw {
		tblMap, ok := tbl.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := tblMap["name"].(string)
		if name == "" {
			name, _ = tblMap["table"].(string)
		}
		if name == "employees" {
			employeesTable = tblMap
			break
		}
	}
	if employeesTable == nil {
		t.Fatalf("employees table not found in schema response: %+v", tablesRaw)
	}

	// Verify employees table has expected columns
	columnsRaw, ok := employeesTable["columns"].([]interface{})
	if !ok {
		t.Fatalf("employees table missing 'columns' array")
	}

	expectedColumns := map[string]bool{
		"emp_no":     false,
		"first_name": false,
		"last_name":  false,
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
			t.Errorf("expected column %q not found in employees table", col)
		}
	}
}

// extractConnectionInfo returns the connection parameters the querylex binary
// should use to reach this MySQL instance. Host and port are sourced from the
// TEST_MYSQL_DSN env var when set (E2E tests against Docker — container-internal
// @@hostname/@@port are not resolvable from the host).
func extractConnectionInfo(t *testing.T, db *sql.DB) (string, int, string) {
	t.Helper()

	host := "127.0.0.1"
	port := 3306

	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		_, p := testhelper.ExtractHostPort(dsn)
		if p > 0 {
			port = p
		}
	}

	// Get current database name
	var dbName string
	if err := db.QueryRow("SELECT DATABASE()").Scan(&dbName); err != nil {
		t.Fatalf("failed to query database name: %v", err)
	}

	return host, port, dbName
}
