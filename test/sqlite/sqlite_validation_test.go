//go:build e2e

package sqlite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestSQLiteValidation verifies the querylex validate command against the
// Chinook schema. It runs 10 table-driven sub-tests covering:
//   - Valid SELECT queries (4 variants): pass with exit code 0 and success=true
//   - DML statements (INSERT, UPDATE, DELETE): rejected with UNSAFE_SQL
//   - NO DCL (GRANT/REVOKE not supported by SQLite — omitted)
//   - Bad table reference: TABLE_NOT_FOUND
//   - Bad column reference: COLUMN_NOT_FOUND
//   - Invalid SQL syntax: INVALID_SQL
//
// Each sub-test runs against the same per-test SQLite database with the
// Chinook schema loaded and uses setupE2EWorkspaceSQLite for workspace state.
func TestSQLiteValidation(t *testing.T) {
	db := testhelper.ConnectSQLite(t)

	// Load Chinook schema
	loadChinookSchema(t, db)

	// Get dbPath and set up workspace
	dbPath := getDatabasePath(t, db)
	setupE2EWorkspaceSQLite(t, dbPath)

	tests := []struct {
		name        string
		sql         string
		wantSuccess bool
		wantErrCode string
	}{
		// --- Valid SELECT queries (4 variants) ---
		{
			name:        "valid_select_simple",
			sql:         "SELECT AlbumId, Title FROM Album LIMIT 5",
			wantSuccess: true,
		},
		{
			name:        "valid_select_join",
			sql:         "SELECT a.Title, ar.Name FROM Album a JOIN Artist ar ON a.ArtistId = ar.ArtistId LIMIT 3",
			wantSuccess: true,
		},
		{
			name:        "valid_select_aggregate",
			sql:         "SELECT ArtistId, COUNT(*) as cnt FROM Album GROUP BY ArtistId",
			wantSuccess: true,
		},
		{
			name:        "valid_select_where",
			sql:         "SELECT * FROM Album WHERE Title LIKE 'A%' LIMIT 5",
			wantSuccess: true,
		},

		// --- Unsafe DML (rejected as UNSAFE_SQL) ---
		{
			name:         "unsafe_dml_insert",
			sql:          "INSERT INTO Album (AlbumId, Title, ArtistId) VALUES (1, 'Test Album', 1)",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_update",
			sql:          "UPDATE Album SET Title = 'Changed' WHERE AlbumId = 1",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_delete",
			sql:          "DELETE FROM Album WHERE AlbumId = 1",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},

		// --- DCL tests OMITTED — SQLite does not support GRANT/REVOKE ---

		// --- Bad references ---
		{
			name:         "bad_table_ref",
			sql:          "SELECT * FROM nonexistent_table_xyz123",
			wantSuccess:  false,
			wantErrCode:  "TABLE_NOT_FOUND",
		},
		{
			name:         "bad_column_ref",
			sql:          "SELECT nonexistent_col_xyz123 FROM Album",
			wantSuccess:  false,
			wantErrCode:  "COLUMN_NOT_FOUND",
		},

		// --- Invalid syntax ---
		{
			name:         "invalid_sql_syntax",
			sql:          "SELECT * FROM Album WHERE",
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
