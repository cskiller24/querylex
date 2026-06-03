//go:build e2e

package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// TestSQLiteSnapshot verifies that the full Chinook schema output matches a
// committed golden file. It extracts the entire schema via querylex schema
// (no --table filter), normalizes non-deterministic fields, and compares
// against test/testdata/golden/sqlite/TestSnapshotOutput.json.
//
// Run with: go test -tags e2e -run TestSQLiteSnapshot -v
// To regenerate golden file: go test -tags e2e -run TestSQLiteSnapshot -update
func TestSQLiteSnapshot(t *testing.T) {
	db := testhelper.ConnectSQLite(t)

	// Load Chinook schema into the SQLite database
	loadChinookSchema(t, db)

	// Get the database file path for workspace config
	dbPath := getDatabasePath(t, db)

	// Set up workspace pointing to the SQLite file
	setupE2EWorkspaceSQLite(t, dbPath)

	// Run querylex schema WITHOUT --table filter to extract ALL tables
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "schema")

	// Assert exit code 0
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s",
			exitCode, stdout, stderr)
	}

	// Normalize non-deterministic fields for reproducible comparison
	normalized := normalizeGoldenJSON(t, stdout)

	goldenPath := filepath.Join("test", "testdata", "golden", "sqlite", "TestSnapshotOutput.json")

	// If -update flag is set, write normalized output to golden file and return
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(normalized), 0644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		return
	}

	// Read golden file
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found — run with -update to generate", goldenPath)
	}

	// Compare normalized output against golden file (byte-by-byte after normalization)
	if normalized != string(expected) {
		// Pretty-print diff for debugging
		var gotFormatted, wantFormatted string
		var gotMap, wantMap map[string]any
		if err := json.Unmarshal([]byte(normalized), &gotMap); err == nil {
			if pretty, err := json.MarshalIndent(gotMap, "", "  "); err == nil {
				gotFormatted = string(pretty)
			}
		}
		if err := json.Unmarshal(expected, &wantMap); err == nil {
			if pretty, err := json.MarshalIndent(wantMap, "", "  "); err == nil {
				wantFormatted = string(pretty)
			}
		}
		t.Errorf("output mismatch (-want +got):\n--- expected (golden):\n%s--- got (normalized):\n%s",
			wantFormatted, gotFormatted)
	}
}
