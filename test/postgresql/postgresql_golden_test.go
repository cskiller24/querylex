//go:build e2e

package postgresql

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/test/testhelper"
)

// Package-level flag to control golden file regeneration.
// Usage: go test -tags e2e -update
var update = flag.Bool("update", false, "update golden files")

// normalizeGoldenJSON parses JSON output and replaces non-deterministic fields
// with stable placeholder values so golden file comparison is repeatable.
//
// Non-deterministic fields normalized:
//   - meta.trace_id → "00000000-0000-0000-0000-000000000000"
//   - meta.duration_ms → 0
//   - meta.active_database_id → nil
//
// For non-JSON input (e.g., cobra stderr errors), the raw string is returned
// unchanged so tests can handle both JSON and plaintext paths.
func normalizeGoldenJSON(t *testing.T, raw string) string {
	t.Helper()

	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		// Not JSON — return raw unchanged (handles cobra stderr errors)
		return raw
	}

	// Normalize top-level meta fields (present in both success and error responses)
	if meta, ok := resp["meta"].(map[string]any); ok {
		meta["trace_id"] = "00000000-0000-0000-0000-000000000000"
		meta["duration_ms"] = float64(0)
		meta["active_database_id"] = nil
	}

	// Normalize PostgreSQL-specific volatile fields (oid, etc.)
	normalizeEngineFields(resp)

	pretty, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		// Should never happen for valid Go types, but handle gracefully
		return raw
	}
	return string(pretty) + "\n"
}

// TestPostgreSQLGolden verifies the JSON output of querylex schema against a
// committed golden file. It normalizes non-deterministic fields before
// comparison and supports the -update flag for golden file regeneration.
//
// Run with: go test -tags e2e -run TestPostgreSQLGolden -v
// To update golden files: go test -tags e2e -run TestPostgreSQLGolden -update
// normalizeEngineFields normalizes PostgreSQL-specific volatile fields in the
// response data to enable stable golden file comparison.
//
// PostgreSQL volatile fields:
//   - oid: object identifiers that vary between installations
//   - *_oid: any field ending with _oid
func normalizeEngineFields(data map[string]any) {
	for k, v := range data {
		switch val := v.(type) {
		case string:
			if k == "oid" || strings.HasSuffix(k, "_oid") {
				data[k] = "0"
			}
		case float64:
			if k == "oid" || strings.HasSuffix(k, "_oid") {
				data[k] = float64(0)
			}
		case map[string]any:
			normalizeEngineFields(val)
		case []any:
			for _, item := range val {
				if m, ok := item.(map[string]any); ok {
					normalizeEngineFields(m)
				}
			}
		}
	}
}

// TestPostgreSQLExplain verifies the JSON output of querylex explain against a
// committed golden file. It normalizes non-deterministic fields before
// comparison and supports the -update flag for golden file regeneration.
//
// Run with: go test -tags e2e -run TestPostgreSQLExplain -v
// To update golden files: go test -tags e2e -run TestPostgreSQLExplain -update
func TestPostgreSQLExplain(t *testing.T) {
	db := testhelper.ConnectPostgreSQL(t)

	// Load Pagila schema into the per-test database
	loadPagilaSchema(t, db)

	// Extract connection info from the live PostgreSQL connection
	host, port, dbName := extractConnectionInfo(t, db)

	// Set up workspace pointing to the per-test database
	setupE2EWorkspace(t, host, port, dbName)

	// Run querylex explain with engine-specific SQL
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "explain", "SELECT * FROM actor WHERE actor_id = 1")

	// Assert exit code 0
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s",
			exitCode, stdout, stderr)
	}

	// Normalize non-deterministic fields
	normalized := normalizeGoldenJSON(t, stdout)

	goldenPath := filepath.Join("test", "testdata", "golden", "postgresql", "TestExplainOutput.json")

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
		t.Fatalf("read golden file %s: %v (run with -update to generate)", goldenPath, err)
	}

	// Compare normalized output against golden file
	if normalized != string(expected) {
		t.Errorf("output mismatch (-want +got):\n--- expected (golden):\n%s--- got (normalized):\n%s",
			string(expected), normalized)
	}
}

func TestPostgreSQLGolden(t *testing.T) {
	db := testhelper.ConnectPostgreSQL(t)

	// Load Pagila schema into the per-test database
	loadPagilaSchema(t, db)

	// Extract connection info from the live PostgreSQL connection
	host, port, dbName := extractConnectionInfo(t, db)

	// Set up workspace pointing to the per-test database
	setupE2EWorkspace(t, host, port, dbName)

	// Run querylex schema --table actor
	stdout, stderr, exitCode := testhelper.RunQuerylex(t, "schema", "--table", "actor")

	// Assert exit code 0
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s",
			exitCode, stdout, stderr)
	}

	// Normalize non-deterministic fields
	normalized := normalizeGoldenJSON(t, stdout)

	goldenPath := filepath.Join("test", "testdata", "golden", "postgresql", "TestSchemaOutput.json")

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
		t.Fatalf("read golden file %s: %v (run with -update to generate)", goldenPath, err)
	}

	// Compare normalized output against golden file
	if normalized != string(expected) {
		t.Errorf("output mismatch (-want +got):\n--- expected (golden):\n%s--- got (normalized):\n%s",
			string(expected), normalized)
	}
}
