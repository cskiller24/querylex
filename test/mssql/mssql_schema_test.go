//go:build e2e

package mssql

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMSSQLSchema is a full vertical slice E2E test that:
//  1. Connects to live MSSQL and creates a per-test e2e_* database
//  2. Loads the Northwind schema
//  3. Sets up a synthetic querylex workspace pointing to the per-test DB
//  4. Runs querylex schema --table Customers as a subprocess
//  5. Verifies exit code 0 and JSON success response
//  6. Verifies the response contains the Customers table with CustomerID,
//     CompanyName, and ContactName columns
func TestMSSQLSchema(t *testing.T) {
	db := testhelper.ConnectMSSQL(t)

	// Load Northwind schema into the per-test database
	loadNorthwindSchema(t, db)

	// Extract connection info from the live MSSQL connection
	host, port, dbName := extractConnectionInfo(t, db)

	// Set up workspace pointing to the per-test database
	setupE2EWorkspace(t, host, port, dbName)

	// Run querylex schema --table Customers
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "schema", "--table", "Customers")

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

	// Verify data contains Customers table
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'data' field")
	}
	tablesRaw, ok := data["tables"].([]interface{})
	if !ok {
		t.Fatalf("data missing 'tables' array")
	}

	// Find the Customers table in the response
	var customersTable map[string]interface{}
	for _, tbl := range tablesRaw {
		tblMap, ok := tbl.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := tblMap["name"].(string); ok && name == "Customers" {
			customersTable = tblMap
			break
		}
	}
	if customersTable == nil {
		t.Fatalf("Customers table not found in schema response: %+v", tablesRaw)
	}

	// Verify Customers table has expected columns
	columnsRaw, ok := customersTable["columns"].([]interface{})
	if !ok {
		t.Fatalf("Customers table missing 'columns' array")
	}

	expectedColumns := map[string]bool{
		"CustomerID":   false,
		"CompanyName":  false,
		"ContactName":  false,
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
			t.Errorf("expected column %q not found in Customers table", col)
		}
	}
}

// extractConnectionInfo queries the MSSQL connection for hostname, port,
// and current database name. Host is hardcoded to "localhost" for workspace
// config since MSSQL connections use DSN-based addressing.
func extractConnectionInfo(t *testing.T, db *sql.DB) (string, int, string) {
	t.Helper()

	// Get hostname — hardcoded to localhost for workspace config
	hostname := "localhost"

	// Get port — hardcoded to 0 (resolved at runtime via connect.go)
	port := 0

	// Get current database name
	var dbName string
	if err := db.QueryRow("SELECT DB_NAME()").Scan(&dbName); err != nil {
		t.Fatalf("failed to query database name: %v", err)
	}

	return hostname, port, dbName
}
