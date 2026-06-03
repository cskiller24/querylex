//go:build e2e

package postgresql

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestPostgreSQLValidation verifies the querylex validate command against the
// Pagila schema. It runs 12 table-driven sub-tests covering:
//   - Valid SELECT queries (4 variants): pass with exit code 0 and success=true
//   - DML statements (INSERT, UPDATE, DELETE): rejected with UNSAFE_SQL
//   - DCL statements (GRANT, REVOKE): rejected with UNSAFE_SQL
//   - Bad table reference: TABLE_NOT_FOUND
//   - Bad column reference: COLUMN_NOT_FOUND
//   - Invalid SQL syntax: INVALID_SQL
//
// Each sub-test runs against the same per-test PostgreSQL database with the
// Pagila schema loaded and uses setupE2EWorkspace for workspace state.
func TestPostgreSQLValidation(t *testing.T) {
	db := testhelper.ConnectPostgreSQL(t)

	// Load Pagila schema (DDL only, no data)
	loadPagilaSchema(t, db)

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
			sql:         "SELECT actor_id, first_name, last_name FROM actor LIMIT 5",
			wantSuccess: true,
		},
		{
			name:        "valid_select_join",
			sql:         "SELECT a.first_name, a.last_name, f.title FROM actor a JOIN film_actor fa ON a.actor_id = fa.actor_id JOIN film f ON fa.film_id = f.film_id LIMIT 3",
			wantSuccess: true,
		},
		{
			name:        "valid_select_aggregate",
			sql:         "SELECT category_id, COUNT(*) as cnt FROM film_category GROUP BY category_id",
			wantSuccess: true,
		},
		{
			name:        "valid_select_where",
			sql:         "SELECT * FROM actor WHERE last_name LIKE 'A%' LIMIT 5",
			wantSuccess: true,
		},

		// --- Unsafe DML (rejected as UNSAFE_SQL) ---
		{
			name:         "unsafe_dml_insert",
			sql:          "INSERT INTO actor (actor_id, first_name, last_name) VALUES (1, 'Test', 'User')",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_update",
			sql:          "UPDATE actor SET first_name = 'Changed' WHERE actor_id = 1",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dml_delete",
			sql:          "DELETE FROM actor WHERE actor_id = 1",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},

		// --- Unsafe DCL (rejected as UNSAFE_SQL) ---
		{
			name:         "unsafe_dcl_grant",
			sql:          "GRANT SELECT ON actor TO testuser",
			wantSuccess:  false,
			wantErrCode:  "UNSAFE_SQL",
		},
		{
			name:         "unsafe_dcl_revoke",
			sql:          "REVOKE SELECT ON actor FROM testuser",
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
			sql:          "SELECT nonexistent_col_xyz123 FROM actor",
			wantSuccess:  false,
			wantErrCode:  "COLUMN_NOT_FOUND",
		},

		// --- Invalid syntax ---
		{
			name:         "invalid_sql_syntax",
			sql:          "SELECT * FROM actor WHERE",
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
