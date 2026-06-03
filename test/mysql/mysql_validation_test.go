//go:build e2e

package mysql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMySQLValidation verifies the querylex validate command against the
// employees schema. It runs 12 table-driven sub-tests covering:
//   - Valid SELECT queries (4 variants): pass with exit code 0 and success=true
//   - DML statements (INSERT, UPDATE, DELETE): rejected with UNSAFE_SQL
//   - DCL statements (GRANT, REVOKE): rejected with UNSAFE_SQL
//   - Bad table reference: TABLE_NOT_FOUND
//   - Bad column reference: COLUMN_NOT_FOUND
//   - Invalid SQL syntax: INVALID_SQL
//
// Each sub-test runs against the same per-test MySQL database with the
// employees schema loaded and uses setupE2EWorkspace for workspace state.
func TestMySQLValidation(t *testing.T) {
	db := testhelper.ConnectMySQL(t)

	// Load employees schema + small data tables (departments, dept_manager)
	loadEmployeesDB(t, db)

	// Extract connection info and set up workspace
	host, port, dbName := extractConnectionInfo(t, db)
	setupE2EWorkspace(t, host, port, dbName)

	tests := []struct {
		name        string
		sql         string
		wantSuccess bool
		wantErrCode string
	}{
		// --- Valid SELECT queries (4 variants) ---
		{
			name:        "valid_select_simple",
			sql:         "SELECT emp_no, first_name, last_name FROM employees LIMIT 5",
			wantSuccess: true,
		},
		{
			name:        "valid_select_join",
			sql:         "SELECT e.first_name, e.last_name, d.dept_name FROM employees e JOIN dept_emp de ON e.emp_no = de.emp_no JOIN departments d ON de.dept_no = d.dept_no LIMIT 3",
			wantSuccess: true,
		},
		{
			name:        "valid_select_aggregate",
			sql:         "SELECT dept_no, COUNT(*) as cnt FROM dept_emp GROUP BY dept_no",
			wantSuccess: true,
		},
		{
			name:        "valid_select_where",
			sql:         "SELECT * FROM employees WHERE hire_date > '2000-01-01' LIMIT 5",
			wantSuccess: true,
		},

		// --- Unsafe DML (rejected as UNSAFE_SQL) ---
		{
			name:         "unsafe_dml_insert",
			sql:          "INSERT INTO employees (emp_no, birth_date, first_name, last_name, gender, hire_date) VALUES (1, '2000-01-01', 'Test', 'User', 'M', '2020-01-01')",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_update",
			sql:          "UPDATE employees SET first_name = 'Changed' WHERE emp_no = 10001",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_delete",
			sql:          "DELETE FROM employees WHERE emp_no = 10001",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},

		// --- Unsafe DCL (rejected as UNSAFE_SQL) ---
		{
			name:         "unsafe_dcl_grant",
			sql:          "GRANT SELECT ON employees TO 'testuser'@'localhost'",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dcl_revoke",
			sql:          "REVOKE SELECT ON employees FROM 'testuser'@'localhost'",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},

		// --- Bad references ---
		{
			name:         "bad_table_ref",
			sql:          "SELECT * FROM nonexistent_table_xyz123",
			wantSuccess:  false,
			wantErrCode:  "TABLE_NOT_FOUND",
		},
		{
			name:         "bad_column_ref",
			sql:          "SELECT nonexistent_col_xyz123 FROM employees",
			wantSuccess:  false,
			wantErrCode:  "COLUMN_NOT_FOUND",
		},

		// --- Invalid syntax ---
		{
			name:         "invalid_sql_syntax",
			sql:          "SELECT * FROM employees WHERE",
			wantSuccess:  false,
			wantErrCode:  "INVALID_SQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := testhelper.RunQuerylex(t, "validate", tt.sql)

			if tt.wantSuccess {
				// Assert exit code 0
				if exitCode != 0 {
					t.Errorf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
						exitCode, stdout, stderr)
				}

				// Assert JSON response has success=true
				var resp map[string]any
				if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
					t.Fatalf("invalid JSON response: %v\nstdout: %s", err, stdout)
				}
				if success, ok := resp["success"].(bool); !ok || !success {
					t.Errorf("expected success=true, got success=%v", resp["success"])
				}
			} else {
				// Assert exit code 1 (or non-zero)
				if exitCode != 1 {
					t.Errorf("expected exit 1, got %d\nstdout: %s\nstderr: %s",
						exitCode, stdout, stderr)
				}

				// Assert JSON error response contains expected error code
				var resp map[string]any
				if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
					if errObj, ok := resp["error"].(map[string]any); ok {
						if code, ok := errObj["code"].(string); ok {
							if code != tt.wantErrCode {
								t.Errorf("expected error.code %q, got %q\nstdout: %s",
									tt.wantErrCode, code, stdout)
							}
						} else {
							t.Errorf("error.code missing or not a string: %v", errObj["code"])
						}
					} else {
						t.Errorf("response missing 'error' object: %s", stdout)
					}
				} else {
					// JSON parse failed — check stderr for error code as fallback
					if !strings.Contains(stderr, tt.wantErrCode) {
						t.Errorf("expected error containing %q in stderr, got: %s",
							tt.wantErrCode, stderr)
					}
				}
			}
		})
	}
}
