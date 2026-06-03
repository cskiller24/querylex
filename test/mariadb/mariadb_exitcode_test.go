//go:build e2e

package mariadb

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestMariaDBExitCodes verifies that querylex returns correct exit codes and
// JSON error codes for success and all documented error paths.
//
// Table-driven with 9 sub-tests:
//   - success_schema: exit 0, success JSON for valid schema query
//   - connection_failed: exit 1, error.code "CONNECTION_FAILED"
//   - invalid_sql: exit 1, error.code "INVALID_SQL"
//   - unsafe_sql_insert: exit 1, error.code "UNSAFE_SQL"
//   - unsafe_sql_update: exit 1, error.code "UNSAFE_SQL"
//   - unsafe_sql_delete: exit 1, error.code "UNSAFE_SQL"
//   - table_not_found: exit 1, error.code "TABLE_NOT_FOUND"
//   - column_not_found: exit 1, error.code "COLUMN_NOT_FOUND"
//   - invalid_argument_flag: exit 1, cobra stderr "unknown flag"
func TestMariaDBExitCodes(t *testing.T) {
	// Set up a valid workspace with employees schema for tests that need it.
	// The connection_failed sub-test creates its own workspace pointing to
	// an unreachable host.
	db := testhelper.ConnectMariaDB(t)
	loadEmployeesSchema(t, db)
	host, port, dbName := extractConnectionInfo(t, db)
	setupE2EWorkspace(t, host, port, dbName)

	tests := []struct {
		name          string
		args          []string
		wantExitCode  int
		wantErrCode   string   // expected error.code substring, empty if not JSON
		stderrSubstr  string   // expected stderr substring (for cobra errors)
		setupWorkspace bool   // true = create per-subtest workspace
		setupHost     string   // host for per-subtest workspace
		setupPort     int      // port for per-subtest workspace
		setupDB       string   // dbName for per-subtest workspace
	}{
		{
			name:         "success_schema",
			args:         []string{"schema", "--table", "employees"},
			wantExitCode: 0,
			wantErrCode:  "",
		},
		{
			name:           "connection_failed",
			args:           []string{"schema"},
			wantExitCode:   1,
			wantErrCode:    "CONNECTION_FAILED",
			setupWorkspace: true,
			setupHost:      "127.0.0.1",
			setupPort:      19999,
			setupDB:        "nonexistent",
		},
		{
			name:         "invalid_sql",
			args:         []string{"validate", "SYNTAX ERROR GARBAGE XYZ"},
			wantExitCode: 1,
			wantErrCode:  "INVALID_SQL",
		},
		{
			name:         "unsafe_sql_insert",
			args:         []string{"validate", "INSERT INTO employees (emp_no, birth_date, first_name, last_name, gender, hire_date) VALUES (1, '2000-01-01', 'X', 'Y', 'M', '2020-01-01')"},
			wantExitCode: 1,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_sql_update",
			args:         []string{"validate", "UPDATE employees SET first_name = 'Z' WHERE emp_no = 10001"},
			wantExitCode: 1,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_sql_delete",
			args:         []string{"validate", "DELETE FROM employees WHERE emp_no = 10001"},
			wantExitCode: 1,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "table_not_found",
			args:         []string{"validate", "SELECT * FROM nonexistent_table_xyz123"},
			wantExitCode: 1,
			wantErrCode:  "TABLE_NOT_FOUND",
		},
		{
			name:         "column_not_found",
			args:         []string{"validate", "SELECT nonexistent_col_xyz123 FROM employees"},
			wantExitCode: 1,
			wantErrCode:  "COLUMN_NOT_FOUND",
		},
		{
			name:           "invalid_argument_flag",
			args:           []string{"schema", "--nonexistent-flag-xyz"},
			wantExitCode:   1,
			wantErrCode:    "",
			stderrSubstr:   "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For sub-tests that need their own workspace (e.g., connection_failed
			// pointing to unreachable host), create a separate workspace.
			if tt.setupWorkspace {
				setupE2EWorkspace(t, tt.setupHost, tt.setupPort, tt.setupDB)
			}

			stdout, stderr, exitCode := testhelper.RunQuerylex(t, tt.args...)

			// Assert exit code
			if exitCode != tt.wantExitCode {
				t.Errorf("exit code: want %d, got %d\nstdout: %s\nstderr: %s",
					tt.wantExitCode, exitCode, stdout, stderr)
			}

			// Assert error code in JSON stdout (for application errors)
			if tt.wantErrCode != "" {
				var resp map[string]any
				if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
					if errObj, ok := resp["error"].(map[string]any); ok {
						if code, ok := errObj["code"].(string); ok {
							if !strings.Contains(code, tt.wantErrCode) {
								t.Errorf("expected error.code containing %q, got %q",
									tt.wantErrCode, code)
							}
						} else {
							t.Errorf("error.code missing or not a string: %v", errObj["code"])
						}
					} else {
						t.Errorf("response missing 'error' object: %s", stdout)
					}
				} else {
					// JSON parse failed — check stderr as fallback
					if !strings.Contains(stderr, tt.wantErrCode) {
						t.Errorf("expected error containing %q in stderr, got: %s",
							tt.wantErrCode, stderr)
					}
				}
			}

			// Assert stderr content (for cobra flag errors that don't produce JSON)
			if tt.stderrSubstr != "" {
				if !strings.Contains(stderr, tt.stderrSubstr) {
					t.Errorf("expected stderr containing %q, got: %s",
						tt.stderrSubstr, stderr)
				}
			}
		})
	}
}
