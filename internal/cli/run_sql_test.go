package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/internal/state"
)

func TestStripMarkdownCodeFences(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"```sql\nSELECT * FROM users\n```", "SELECT * FROM users"},
		{"```\nSELECT 1\n```", "SELECT 1"},
		{"SELECT 1", "SELECT 1"},
		{"  SELECT 1  ", "SELECT 1"},
	}

	for _, tt := range tests {
		result := stripMarkdownCodeFences(tt.input)
		if result != tt.expected {
			t.Errorf("stripMarkdownCodeFences(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRunSQLGeneration_NoActiveDatabase(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{})
	t.Setenv("HOME", home)

	err := RunSQLGeneration(context.Background(), "test question")
	if err == nil {
		t.Fatal("expected error for no active database")
	}
}

func TestRunSQLGeneration_NoAiConfig(t *testing.T) {
	home := setupPreflightTestWorkspace(t, []state.DatabaseEntry{
		{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed},
	})
	t.Setenv("HOME", home)

	err := RunSQLGeneration(context.Background(), "show me orders")
	if err == nil {
		t.Fatal("expected error for missing AI config")
	}
}

type mockWriter struct {
	strings.Builder
}

func TestRenderStatsHuman_NoDatabases(t *testing.T) {
	var buf strings.Builder
	data := StatsData{
		ActiveDatabaseID:   nil,
		ConnectedDatabases: []state.DatabaseEntry{},
		Health:             nil,
	}

	RenderStatsHuman(&buf, data)
	output := buf.String()
	if !strings.Contains(output, "No databases connected") {
		t.Errorf("expected 'No databases connected', got: %s", output)
	}
}

func TestRenderStatsHuman_WithDatabase(t *testing.T) {
	var buf strings.Builder
	dbID := "db-1"
	data := StatsData{
		ActiveDatabaseID: &dbID,
		ConnectedDatabases: []state.DatabaseEntry{
			{ID: "db-1", Name: "TestDB", Type: "mysql", Status: "indexed"},
		},
		Health: &HealthReport{
			Databases: []DatabaseHealth{
				{
					DatabaseID:          "db-1",
					DatabaseName:        "TestDB",
					Status:              "indexed",
					ProgressPercent:     100,
					CredentialStatus:    "available",
					MemoryIndexState:    "healthy",
					ExplainCacheSummary: "5 entries",
					Artifacts: map[string]string{
						"schema/schema.json": "present",
					},
				},
			},
		},
	}

	RenderStatsHuman(&buf, data)
	output := buf.String()
	if !strings.Contains(output, "TestDB") {
		t.Errorf("expected database name 'TestDB' in output, got: %s", output)
	}
	if !strings.Contains(output, "Active database: db-1") {
		t.Errorf("expected active database reference, got: %s", output)
	}
}
