package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/querylex/querylex/internal/state"
)

func TestAIConfig_NoCredentialStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsDir := filepath.Join(home, ".querylex")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &state.Workspace{
		ConnectedDatabases: []state.DatabaseEntry{
			{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusIndexed, IndexingProgress: 100},
		},
	}
	ws.ActiveDatabaseID = &ws.ConnectedDatabases[0].ID
	wsData, _ := json.Marshal(ws)
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	dbDir := filepath.Join(home, ".querylex", "db-1")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db: %v", err)
	}

	_, errResp := PreflightForAICommand()
	if errResp == nil {
		t.Fatal("expected error for unconfigured AI, got nil")
	}
	if errResp.Success {
		t.Fatal("expected success=false")
	}
}

func TestAIPreflight_NoActiveDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsDir := filepath.Join(home, ".querylex")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &state.Workspace{
		ConnectedDatabases: []state.DatabaseEntry{},
	}
	wsData, _ := json.Marshal(ws)
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	_, errResp := PreflightForAICommand()
	if errResp == nil {
		t.Fatal("expected error for no active database, got nil")
	}
	if errResp.Success {
		t.Fatal("expected success=false")
	}
}

func TestAIPreflight_NotIndexedDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wsDir := filepath.Join(home, ".querylex")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &state.Workspace{
		ConnectedDatabases: []state.DatabaseEntry{
			{ID: "db-1", Name: "TestDB", Type: "mysql", Status: state.StatusNotIndexed, IndexingProgress: 0},
		},
	}
	ws.ActiveDatabaseID = &ws.ConnectedDatabases[0].ID
	wsData, _ := json.Marshal(ws)
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}

	_, errResp := PreflightForAICommand()
	if errResp == nil {
		t.Fatal("expected error for not_indexed database, got nil")
	}
	if errResp.Success {
		t.Fatal("expected success=false")
	}
}
