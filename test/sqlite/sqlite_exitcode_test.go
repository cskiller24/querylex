//go:build e2e

package sqlite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestSQLiteExitCodes verifies that querylex returns correct exit codes and
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
func TestSQLiteExitCodes(t *testing.T) {
	// Set up a valid workspace with Chinook schema for tests that need it.
	// The connection_failed sub-test creates its own workspace pointing to
	// an unreachable host.
	db := testhelper.ConnectSQLite(t)
	loadChinookSchema(t, db)
	dbPath := getDatabasePath(t, db)
	setupE2EWorkspaceSQLite(t, dbPath)

	tests := []struct {
		name           string
		args           []string
		wantExitCode   int
		wantErrCode    string   // expected error.code substring, empty if not JSON
		stderrSubstr   string   // expected stderr substring (for cobra errors)
		setupWorkspace bool     // true = create per-subtest workspace
		setupDBPath    string   // dbPath for per-subtest workspace (SQLite only)
	}{
		{
			name:         "success_schema",
			args:         []string{"schema", "--table", "Album"},
			wantExitCode: 0,
			wantErrCode:  "",
		},
		{
			name:           "connection_failed",
			args:           []string{"schema"},
			wantExitCode:   1,
			wantErrCode:    "CONNECTION_FAILED",
			setupWorkspace: true,
			setupDBPath:    "/nonexistent/path/e2e_test.db",
		},
		{
			name:         "invalid_sql",
			args:         []string{"validate", "SYNTAX ERROR GARBAGE XYZ"},
			wantExitCode: 1,
			wantErrCode:  "INVALID_SQL",
		},
		{
			name:         "unsafe_sql_insert",
			args:         []string{"validate", "INSERT INTO Album (AlbumId, Title, ArtistId) VALUES (1, 'Test Album', 1)"},
			wantExitCode: 1,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_sql_update",
			args:         []string{"validate", "UPDATE Album SET Title = 'New Title' WHERE AlbumId = 1"},
			wantExitCode: 1,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_sql_delete",
			args:         []string{"validate", "DELETE FROM Album WHERE AlbumId = 1"},
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
			args:         []string{"validate", "SELECT nonexistent_col_xyz123 FROM Album"},
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
				setupE2EWorkspaceSQLite(t, tt.setupDBPath)
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
