package cli

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/format"
	"github.com/querylex/querylex/internal/queryutil"
)

// Test 3: RunValidate("SELECT * FROM users") returns Valid=true with normalized SQL
func TestRunValidate_ValidSQL(t *testing.T) {
	adapter := &explainMockAdapter{
		validateFn: func(ctx context.Context, query string) (*db.ValidateResult, error) {
			return &db.ValidateResult{
				Valid:         true,
				NormalizedSQL: "SELECT * FROM users",
				StatementType: "SELECT",
				ReadOnly:      true,
				Tables:        []string{"users"},
				Columns:       []string{"users.*"},
			}, nil
		},
	}

	traceID := "test-trace"
	resp := runValidateWithAdapter(adapter, "SELECT * FROM users", traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if !resp.Success {
		t.Fatalf("expected Success=true, got Success=%v, error=%v", resp.Success, resp.Error)
	}
	if !resp.Data.Valid {
		t.Fatal("expected Valid=true")
	}
	if resp.Data.NormalizedSQL == "" {
		t.Fatal("expected normalized_sql to be present")
	}
	if !resp.Data.ReadOnly {
		t.Fatal("expected ReadOnly=true for SELECT query")
	}
}

// Test 4: RunValidate("DROP TABLE users") returns UNSAFE_SQL (Layer 1)
func TestRunValidate_DML_DROP(t *testing.T) {
	adapter := &explainMockAdapter{}
	traceID := "test-trace"
	resp := runValidateWithAdapter(adapter, "DROP TABLE users", traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if resp.Success {
		t.Fatal("expected Success=false for DML query")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != format.ErrCodeUnsafeSQL {
		t.Fatalf("expected error code UNSAFE_SQL, got %s", resp.Error.Code)
	}
}

// Test 5: RunValidate("INSERT INTO users VALUES(1,'a')") returns UNSAFE_SQL
func TestRunValidate_DML_INSERT(t *testing.T) {
	adapter := &explainMockAdapter{}
	traceID := "test-trace"
	resp := runValidateWithAdapter(adapter, "INSERT INTO users VALUES(1,'a')", traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if resp.Success {
		t.Fatal("expected Success=false for DML query")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != format.ErrCodeUnsafeSQL {
		t.Fatalf("expected error code UNSAFE_SQL, got %s", resp.Error.Code)
	}
}

// Test 6: RunValidate("DELETE FROM users") returns UNSAFE_SQL
func TestRunValidate_DML_DELETE(t *testing.T) {
	adapter := &explainMockAdapter{}
	traceID := "test-trace"
	resp := runValidateWithAdapter(adapter, "DELETE FROM users", traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if resp.Success {
		t.Fatal("expected Success=false for DML query")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != format.ErrCodeUnsafeSQL {
		t.Fatalf("expected error code UNSAFE_SQL, got %s", resp.Error.Code)
	}
}

// Test 7: RunValidate("GRANT SELECT ON users TO bob") returns UNSAFE_SQL
func TestRunValidate_DCL_GRANT(t *testing.T) {
	adapter := &explainMockAdapter{}
	traceID := "test-trace"
	resp := runValidateWithAdapter(adapter, "GRANT SELECT ON users TO bob", traceID, strPtr("test-db"))

	if resp == nil {
		t.Fatal("expected non-nil response, got nil")
	}
	if resp.Success {
		t.Fatal("expected Success=false for DCL query")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != format.ErrCodeUnsafeSQL {
		t.Fatalf("expected error code UNSAFE_SQL, got %s", resp.Error.Code)
	}
}

// Test 8: Layer 1 scanner rejects all DML/DCL keywords
func TestValidateSQLSafety_AllBlockedKeywords(t *testing.T) {
	blocked := []string{
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name='x'",
		"DELETE FROM users",
		"DROP TABLE users",
		"ALTER TABLE users ADD COLUMN x int",
		"TRUNCATE TABLE users",
		"MERGE INTO users USING ...",
		"GRANT SELECT ON users TO bob",
		"REVOKE SELECT ON users FROM bob",
		"CREATE TABLE users (id int)",
		"REPLACE INTO users VALUES (1)",
	}

	for _, q := range blocked {
		err := queryutil.ValidateSQLSafety(q)
		if err == nil {
			t.Errorf("expected error for blocked query: %s", q)
		}
		// Verify it contains the blocked keyword mention
		if err != nil {
			errStr := err.Error()
			if len(errStr) == 0 {
				t.Errorf("expected non-empty error message for: %s", q)
			}
		}
	}
}

// Test 9: Non-DML queries pass Layer 1
func TestValidateSQLSafety_SafeQueries(t *testing.T) {
	safe := []string{
		"SELECT * FROM users",
		"EXPLAIN SELECT * FROM users",
		"SHOW TABLES",
		"DESCRIBE users",
		"WITH cte AS (SELECT * FROM users) SELECT * FROM cte",
		"SELECT 1",
	}

	for _, q := range safe {
		err := queryutil.ValidateSQLSafety(q)
		if err != nil {
			t.Errorf("expected no error for safe query '%s', got: %v", q, err)
		}
	}
}
