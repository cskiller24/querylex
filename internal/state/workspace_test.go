package state

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestWorkspaceLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")
	store := NewFileWorkspaceStore(path)

	// Load non-existent file — should return empty workspace
	ws, err := store.Load()
	if err != nil {
		t.Fatalf("Load of non-existent file failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 0 {
		t.Fatalf("Expected empty ConnectedDatabases, got %d", len(ws.ConnectedDatabases))
	}
	if ws.Revision != 0 {
		t.Fatalf("Expected Revision 0, got %d", ws.Revision)
	}
	if ws.ActiveDatabaseID != nil {
		t.Fatalf("Expected nil ActiveDatabaseID, got %v", *ws.ActiveDatabaseID)
	}

	// Save and reload
	ws.ConnectedDatabases = []DatabaseEntry{
		{ID: "test-1", Name: "Test DB", Type: "mysql", Status: StatusNotIndexed, IndexingProgress: 0},
	}
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if len(reloaded.ConnectedDatabases) != 1 {
		t.Fatalf("Expected 1 database, got %d", len(reloaded.ConnectedDatabases))
	}
	if reloaded.ConnectedDatabases[0].ID != "test-1" {
		t.Fatalf("Expected ID 'test-1', got %s", reloaded.ConnectedDatabases[0].ID)
	}
	if reloaded.Revision < 1 {
		t.Fatalf("Expected Revision >= 1, got %d", reloaded.Revision)
	}
	if reloaded.UpdatedAt == "" {
		t.Fatalf("Expected non-empty UpdatedAt")
	}
}

func TestWorkspaceAddRemoveDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")
	store := NewFileWorkspaceStore(path)

	// Add two databases
	db1 := DatabaseEntry{ID: "db1", Name: "Database 1", Type: "mysql", Status: StatusIndexed, IndexingProgress: 100}
	db2 := DatabaseEntry{ID: "db2", Name: "Database 2", Type: "postgres", Status: StatusNotIndexed, IndexingProgress: 0}

	if err := store.AddDatabase(db1); err != nil {
		t.Fatalf("AddDatabase db1 failed: %v", err)
	}
	if err := store.AddDatabase(db2); err != nil {
		t.Fatalf("AddDatabase db2 failed: %v", err)
	}

	// Verify both present
	ws, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 2 {
		t.Fatalf("Expected 2 databases, got %d", len(ws.ConnectedDatabases))
	}

	// Remove one
	if err := store.RemoveDatabase("db1"); err != nil {
		t.Fatalf("RemoveDatabase db1 failed: %v", err)
	}

	ws, err = store.Load()
	if err != nil {
		t.Fatalf("Load after remove failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 1 {
		t.Fatalf("Expected 1 database after remove, got %d", len(ws.ConnectedDatabases))
	}
	if ws.ConnectedDatabases[0].ID != "db2" {
		t.Fatalf("Expected remaining db2, got %s", ws.ConnectedDatabases[0].ID)
	}
}

func TestWorkspaceRevision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")
	store := NewFileWorkspaceStore(path)

	// Each Save should increment revision
	rev1, err := store.GetRevision()
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}

	ws, _ := store.Load()
	ws.ConnectedDatabases = append(ws.ConnectedDatabases, DatabaseEntry{ID: "test", Name: "Test", Type: "mysql"})
	if err := store.Save(ws); err != nil {
		t.Fatalf("First Save failed: %v", err)
	}

	rev2, err := store.GetRevision()
	if err != nil {
		t.Fatalf("GetRevision after save failed: %v", err)
	}
	if rev2 <= rev1 {
		t.Fatalf("Revision should increase: rev1=%d, rev2=%d", rev1, rev2)
	}

	// Another save
	if err := store.Save(ws); err != nil {
		t.Fatalf("Second Save failed: %v", err)
	}

	rev3, err := store.GetRevision()
	if err != nil {
		t.Fatalf("GetRevision after second save failed: %v", err)
	}
	if rev3 <= rev2 {
		t.Fatalf("Revision should increase: rev2=%d, rev3=%d", rev2, rev3)
	}
}

func TestWorkspaceActiveDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")
	store := NewFileWorkspaceStore(path)

	// Set active database
	if err := store.SetActiveDatabase("prod-db"); err != nil {
		t.Fatalf("SetActiveDatabase failed: %v", err)
	}

	ws, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ws.ActiveDatabaseID == nil {
		t.Fatal("Expected non-nil ActiveDatabaseID")
	}
	if *ws.ActiveDatabaseID != "prod-db" {
		t.Fatalf("Expected 'prod-db', got %s", *ws.ActiveDatabaseID)
	}
}

func TestWorkspaceMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")

	// Write malformed JSON
	if err := AtomicWrite(path, []byte(`{invalid json}`)); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	store := NewFileWorkspaceStore(path)
	_, err := store.Load()
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

func TestWorkspaceRemoveActiveDatabaseClearsID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "querylex.json")
	store := NewFileWorkspaceStore(path)

	// Add and set active
	if err := store.AddDatabase(DatabaseEntry{ID: "test-db", Name: "Test", Type: "mysql"}); err != nil {
		t.Fatalf("AddDatabase failed: %v", err)
	}
	if err := store.SetActiveDatabase("test-db"); err != nil {
		t.Fatalf("SetActiveDatabase failed: %v", err)
	}

	// Remove — should clear active
	if err := store.RemoveDatabase("test-db"); err != nil {
		t.Fatalf("RemoveDatabase failed: %v", err)
	}

	ws, _ := store.Load()
	if ws.ActiveDatabaseID != nil {
		t.Fatalf("Expected nil ActiveDatabaseID after removing active db, got %v", *ws.ActiveDatabaseID)
	}
}

// --- InMemoryWorkspaceStore tests ---

func TestInMemoryWorkspaceStore(t *testing.T) {
	store := NewInMemoryWorkspaceStore()

	// Load empty
	ws, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 0 {
		t.Fatalf("Expected empty store")
	}

	// Add database
	if err := store.AddDatabase(DatabaseEntry{ID: "mem-1", Name: "Memory DB", Type: "sqlite"}); err != nil {
		t.Fatalf("AddDatabase failed: %v", err)
	}

	// Set active
	if err := store.SetActiveDatabase("mem-1"); err != nil {
		t.Fatalf("SetActiveDatabase failed: %v", err)
	}

	// Load and verify
	ws, err = store.Load()
	if err != nil {
		t.Fatalf("Load after add failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 1 {
		t.Fatalf("Expected 1 database, got %d", len(ws.ConnectedDatabases))
	}
	if ws.ActiveDatabaseID == nil || *ws.ActiveDatabaseID != "mem-1" {
		t.Fatalf("Expected active 'mem-1'")
	}
	// AddDatabase + SetActiveDatabase each increment revision internally
	if ws.Revision == 0 {
		t.Fatal("Expected non-zero Revision after AddDatabase+SetActiveDatabase")
	}

	// GetRevision returns the current revision (incremented by mutations)
	rev, err := store.GetRevision()
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if rev < 1 {
		t.Fatalf("Expected Revision >= 1 after mutations, got %d", rev)
	}

	// Remove
	if err := store.RemoveDatabase("mem-1"); err != nil {
		t.Fatalf("RemoveDatabase failed: %v", err)
	}
	rev, _ = store.GetRevision()
	if rev != 3 {
		t.Fatalf("Expected Revision 3 after remove, got %d", rev)
	}

	ws, _ = store.Load()
	if len(ws.ConnectedDatabases) != 0 {
		t.Fatalf("Expected 0 after remove")
	}
}

func TestInMemoryWorkspaceSave(t *testing.T) {
	store := NewInMemoryWorkspaceStore()

	ws, _ := store.Load()
	ws.ConnectedDatabases = []DatabaseEntry{
		{ID: "saved-1", Name: "Saved", Type: "mysql"},
	}
	if err := store.Save(ws); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, _ := store.Load()
	if len(reloaded.ConnectedDatabases) != 1 {
		t.Fatalf("Expected 1 after save, got %d", len(reloaded.ConnectedDatabases))
	}
	if reloaded.Revision != 1 {
		t.Fatalf("Expected Revision 1 after Save, got %d", reloaded.Revision)
	}
}

func TestWorkspaceConcurrentAccess(t *testing.T) {
	store := NewInMemoryWorkspaceStore()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "db-" + string(rune('0'+n))
			if err := store.AddDatabase(DatabaseEntry{ID: id, Name: id, Type: "mysql"}); err != nil {
				t.Errorf("AddDatabase %s failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	ws, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ws.ConnectedDatabases) != 10 {
		t.Fatalf("Expected 10 databases after concurrent adds, got %d", len(ws.ConnectedDatabases))
	}
}

func TestWorkspaceConcurrentReads(t *testing.T) {
	store := NewInMemoryWorkspaceStore()

	// Set up initial state
	if err := store.AddDatabase(DatabaseEntry{ID: "read-test", Name: "Read Test", Type: "postgres"}); err != nil {
		t.Fatalf("AddDatabase failed: %v", err)
	}

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws, err := store.Load()
			if err != nil {
				t.Errorf("Load failed: %v", err)
				return
			}
			if len(ws.ConnectedDatabases) < 1 {
				t.Error("Expected at least 1 database")
			}
		}()
	}

	wg.Wait()
}
