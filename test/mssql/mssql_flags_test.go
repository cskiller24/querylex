//go:build e2e

package mssql

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMSSQLFlags validates all flag combinations for the 7 deterministic
// subcommands (schema, stats, indexes, explain, validate, joins, resolve)
// and 2 AI subcommands (sql, optimize). All sub-tests share a single
// workspace setup with Northwind schema loaded into an isolated per-test DB.
//
// Table-driven with sub-tests per flag combination:
//   - Valid flag combinations → exit code 0 + success JSON
//   - Invalid flags (--nonexistent-flag-xyz) → exit code 1 + cobra error on stderr
//   - AI subcommands with QUERYLEX_AI_API_KEY=fake → exit code 1 + AI error code
func TestMSSQLFlags(t *testing.T) {
	// Set up workspace once for all sub-tests
	db := testhelper.ConnectMSSQL(t)
	loadNorthwindSchema(t, db)
	host, port, dbName := extractConnectionInfo(t, db)
	home := setupE2EWorkspace(t, host, port, dbName)

	// Write schema_slim.json for resolve command (reads this file directly)
	writeSchemaSlim(t, home)

	tests := []struct {
		name         string
		args         []string
		wantOK       bool     // expect exit 0 + success JSON
		wantStderr   string   // expected stderr substring (cobra flag errors)
		wantErrCodes []string // expected error.code(s) in JSON (any match passes)
		setupAIKey   bool     // set QUERYLEX_AI_API_KEY=fake before RunQuerylex
	}{
		// ── Schema (flags: --table [stringArray], --tables-json [string]) ──
		{
			name:   "schema_all_tables",
			args:   []string{"schema"},
			wantOK: true,
		},
		{
			name:   "schema_single_table",
			args:   []string{"schema", "--table", "Customers"},
			wantOK: true,
		},
		{
			name:   "schema_multi_table",
			args:   []string{"schema", "--table", "Customers", "--table", "Orders"},
			wantOK: true,
		},
		{
			name:   "schema_tables_json",
			args:   []string{"schema", "--tables-json", `["Customers","Orders"]`},
			wantOK: true,
		},
		{
			name:       "schema_invalid_flag",
			args:       []string{"schema", "--nonexistent-flag-xyz"},
			wantOK:     false,
			wantStderr: "unknown flag",
		},

		// ── Stats (flags: --table [stringArray], --tables-json [string]) ──
		{
			name:   "stats_all_tables",
			args:   []string{"stats"},
			wantOK: true,
		},
		{
			name:   "stats_single_table",
			args:   []string{"stats", "--table", "Customers"},
			wantOK: true,
		},
		{
			name:   "stats_multi_table",
			args:   []string{"stats", "--table", "Customers", "--table", "Orders"},
			wantOK: true,
		},
		{
			name:   "stats_tables_json",
			args:   []string{"stats", "--tables-json", `["Customers","Orders"]`},
			wantOK: true,
		},

		// ── Indexes (flags: --table [stringArray], --tables-json [string], --live [bool]) ──
		{
			name:   "indexes_single_table",
			args:   []string{"indexes", "--table", "Customers"},
			wantOK: true,
		},
		{
			name:   "indexes_tables_json",
			args:   []string{"indexes", "--tables-json", `["Customers"]`},
			wantOK: true,
		},
		{
			name:   "indexes_live",
			args:   []string{"indexes", "--table", "Customers", "--live"},
			wantOK: true,
		},

		// ── Explain (flags: --analyze [bool], --tables-json [string]) ──
		{
			name:   "explain_basic",
			args:   []string{"explain", "SELECT TOP 5 * FROM Customers WHERE CustomerID = 'ALFKI'"},
			wantOK: true,
		},
		{
			name:   "explain_analyze",
			args:   []string{"explain", "--analyze", "SELECT TOP 5 * FROM Customers WHERE CustomerID = 'ALFKI'"},
			wantOK: true,
		},
		{
			name:   "explain_tables_json",
			args:   []string{"explain", "--tables-json", `["Customers"]`, "SELECT TOP 5 * FROM Customers WHERE CustomerID = 'ALFKI'"},
			wantOK: true,
		},

		// ── Validate (flags: --tables-json [string]) ──
		{
			name:   "validate_basic",
			args:   []string{"validate", "SELECT TOP 5 CustomerID, CompanyName FROM Customers"},
			wantOK: true,
		},
		{
			name:   "validate_tables_json",
			args:   []string{"validate", "--tables-json", `["Customers"]`, "SELECT TOP 5 CustomerID FROM Customers"},
			wantOK: true,
		},

		// ── Joins (flags: --table [stringArray], --tables-json [string]) ──
		{
			name:   "joins_single_table",
			args:   []string{"joins", "--table", "Customers"},
			wantOK: true,
		},
		{
			name:   "joins_multi_table",
			args:   []string{"joins", "--table", "Customers", "--table", "Orders"},
			wantOK: true,
		},
		{
			name:   "joins_tables_json",
			args:   []string{"joins", "--tables-json", `["Customers","Orders"]`},
			wantOK: true,
		},

		// ── Resolve (flags: --tables-json [string]) ──
		{
			name:   "resolve_basic",
			args:   []string{"resolve", "customer orders"},
			wantOK: true,
		},
		{
			name:   "resolve_tables_json",
			args:   []string{"resolve", "--tables-json", `["Customers","Orders"]`, "customer orders"},
			wantOK: true,
		},

		// ── Optimize (flags: --analyze [bool], --no-index [bool]) ──
		{
			name:         "optimize_analyze_fake_ai",
			args:         []string{"optimize", "--analyze", "SELECT TOP 5 * FROM Customers WHERE CustomerID = 'ALFKI'"},
			wantOK:       false,
			wantErrCodes: []string{"AI_CONFIG_MISSING", "AI_SERVICE_UNAVAILABLE", "AI_GENERATION_FAILED", "CREDENTIAL_UNAVAILABLE"},
			setupAIKey:   true,
		},
		{
			name:   "optimize_noindex_fake_ai",
			args:   []string{"optimize", "--no-index", "SELECT TOP 5 * FROM Customers"},
			wantOK: false,
		},
		{
			name:       "optimize_invalid_flag",
			args:       []string{"optimize", "--nonexistent-flag"},
			wantOK:     false,
			wantStderr: "unknown flag",
		},

		// ── Sql (no flags — positional args only) ──
		{
			name:         "sql_fake_ai",
			args:         []string{"sql", "show me all customers"},
			wantOK:       false,
			wantErrCodes: []string{"AI_CONFIG_MISSING", "AI_SERVICE_UNAVAILABLE", "AI_GENERATION_FAILED", "CREDENTIAL_UNAVAILABLE"},
			setupAIKey:   true,
		},
		{
			name:   "sql_no_args",
			args:   []string{"sql"},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set QUERYLEX_AI_API_KEY=fake for AI subcommand tests
			if tt.setupAIKey {
				t.Setenv("QUERYLEX_AI_API_KEY", "fake")
			}

			stdout, stderr, exitCode := testhelper.RunQuerylex(t, tt.args...)

			if tt.wantOK {
				// Valid flag combinations: exit 0, success JSON
				if exitCode != 0 {
					t.Errorf("want exit code 0, got %d\nstdout: %s\nstderr: %s",
						exitCode, stdout, stderr)
				}
				var resp map[string]any
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Errorf("failed to parse JSON response: %v\nstdout: %s", err, stdout)
				} else if success, ok := resp["success"].(bool); !ok || !success {
					t.Errorf("expected success=true, got success=%v\nstdout: %s",
						resp["success"], stdout)
				}
			} else {
				// Error cases: non-zero exit code
				if exitCode == 0 {
					t.Errorf("want non-zero exit code, got 0\nstdout: %s\nstderr: %s",
						stdout, stderr)
				}

				// Check stderr for cobra flag errors
				if tt.wantStderr != "" {
					if !strings.Contains(stderr, tt.wantStderr) {
						t.Errorf("expected stderr containing %q, got: %s", tt.wantStderr, stderr)
					}
				}

				// Check JSON error code for AI errors (any of the listed codes matches)
				if len(tt.wantErrCodes) > 0 {
					var resp map[string]any
					if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
						if errObj, ok := resp["error"].(map[string]any); ok {
							if code, ok := errObj["code"].(string); ok {
								matched := false
								for _, wantCode := range tt.wantErrCodes {
									if code == wantCode {
										matched = true
										break
									}
								}
								if !matched {
									t.Errorf("error.code=%q does not match any expected code %v\nstdout: %s",
										code, tt.wantErrCodes, stdout)
								}
							} else {
								t.Errorf("error.code missing from error object: %v", errObj)
							}
						} else {
							t.Errorf("response missing 'error' object: %s", stdout)
						}
					} else {
						t.Errorf("failed to parse JSON for error code check: %v\nstdout: %s", err, stdout)
					}
				}
			}
		})
	}
}

// writeSchemaSlim creates a minimal schema_slim.json with Northwind table
// metadata. The resolve command reads this file directly from the workspace's
// schema directory. Without it, resolve returns SCHEMA_PARSE_ERROR.
func writeSchemaSlim(t *testing.T, home string) {
	t.Helper()

	slimPath := filepath.Join(home, ".querylex", "e2e-test-db", "schema", "schema_slim.json")

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(slimPath), 0755); err != nil {
		t.Fatalf("mkdir schema_slim dir: %v", err)
	}

	type column struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type table struct {
		Name    string   `json:"name"`
		Columns []column `json:"columns"`
	}

	tables := []table{
		{
			Name: "Customers",
			Columns: []column{
				{Name: "CustomerID", Type: "nchar(5)"},
				{Name: "CompanyName", Type: "nvarchar(40)"},
				{Name: "ContactName", Type: "nvarchar(30)"},
				{Name: "ContactTitle", Type: "nvarchar(30)"},
				{Name: "Address", Type: "nvarchar(60)"},
				{Name: "City", Type: "nvarchar(15)"},
				{Name: "Region", Type: "nvarchar(15)"},
				{Name: "PostalCode", Type: "nvarchar(10)"},
				{Name: "Country", Type: "nvarchar(15)"},
				{Name: "Phone", Type: "nvarchar(24)"},
				{Name: "Fax", Type: "nvarchar(24)"},
			},
		},
		{
			Name: "Orders",
			Columns: []column{
				{Name: "OrderID", Type: "int"},
				{Name: "CustomerID", Type: "nchar(5)"},
				{Name: "EmployeeID", Type: "int"},
				{Name: "OrderDate", Type: "datetime"},
				{Name: "RequiredDate", Type: "datetime"},
				{Name: "ShippedDate", Type: "datetime"},
				{Name: "ShipVia", Type: "int"},
				{Name: "Freight", Type: "money"},
				{Name: "ShipName", Type: "nvarchar(40)"},
				{Name: "ShipAddress", Type: "nvarchar(60)"},
				{Name: "ShipCity", Type: "nvarchar(15)"},
				{Name: "ShipRegion", Type: "nvarchar(15)"},
				{Name: "ShipPostalCode", Type: "nvarchar(10)"},
				{Name: "ShipCountry", Type: "nvarchar(15)"},
			},
		},
		{
			Name: "OrderDetails",
			Columns: []column{
				{Name: "OrderID", Type: "int"},
				{Name: "ProductID", Type: "int"},
				{Name: "UnitPrice", Type: "money"},
				{Name: "Quantity", Type: "smallint"},
				{Name: "Discount", Type: "real"},
			},
		},
		{
			Name: "Products",
			Columns: []column{
				{Name: "ProductID", Type: "int"},
				{Name: "ProductName", Type: "nvarchar(40)"},
				{Name: "SupplierID", Type: "int"},
				{Name: "CategoryID", Type: "int"},
				{Name: "QuantityPerUnit", Type: "nvarchar(20)"},
				{Name: "UnitPrice", Type: "money"},
				{Name: "UnitsInStock", Type: "smallint"},
				{Name: "UnitsOnOrder", Type: "smallint"},
				{Name: "ReorderLevel", Type: "smallint"},
				{Name: "Discontinued", Type: "bit"},
			},
		},
	}

	slim := map[string]any{"tables": tables}
	data, err := json.Marshal(slim)
	if err != nil {
		t.Fatalf("marshal schema_slim.json: %v", err)
	}
	if err := os.WriteFile(slimPath, data, 0644); err != nil {
		t.Fatalf("write schema_slim.json: %v", err)
	}
}
