package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	data := []byte(`{"hello": "world"}`)
	if err := AtomicWrite(path, data); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	// Verify content was written correctly
	readback, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readback) != string(data) {
		t.Fatalf("Content mismatch: got %q, want %q", string(readback), string(data))
	}

	// Verify no .tmp file remains
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("Orphaned .tmp file found: %s", entry.Name())
		}
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	// First write
	if err := AtomicWrite(path, []byte(`{"version": 1}`)); err != nil {
		t.Fatalf("First AtomicWrite failed: %v", err)
	}

	// Second write — should replace atomically
	if err := AtomicWrite(path, []byte(`{"version": 2}`)); err != nil {
		t.Fatalf("Second AtomicWrite failed: %v", err)
	}

	readback, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readback) != `{"version": 2}` {
		t.Fatalf("Expected overwritten content, got: %s", string(readback))
	}
}

func TestAtomicWriteLargeData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.json")

	data := make([]byte, 100*1024) // 100KB
	for i := range data {
		data[i] = byte('A' + i%26)
	}

	if err := AtomicWrite(path, data); err != nil {
		t.Fatalf("AtomicWrite large data failed: %v", err)
	}

	readback, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(readback) != len(data) {
		t.Fatalf("Length mismatch: got %d, want %d", len(readback), len(data))
	}
}

func TestAtomicRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "read.json")

	expected := []byte(`{"key": "value"}`)
	if err := AtomicWrite(path, expected); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	data, err := AtomicRead(path)
	if err != nil {
		t.Fatalf("AtomicRead failed: %v", err)
	}
	if string(data) != string(expected) {
		t.Fatalf("Content mismatch: got %q, want %q", string(data), string(expected))
	}
}

func TestAtomicReadNonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	data, err := AtomicRead(path)
	if err != nil {
		t.Fatalf("AtomicRead for non-existent file should return nil, nil: %v", err)
	}
	if data != nil {
		t.Fatalf("Expected nil data for non-existent file, got: %v", data)
	}
}

func TestAtomicReadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")

	// Create an empty file
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	data, err := AtomicRead(path)
	if err != nil {
		t.Fatalf("AtomicRead for empty file should return nil, nil: %v", err)
	}
	if data != nil {
		t.Fatalf("Expected nil data for empty file, got: %v", data)
	}
}

func TestCleanupTempFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some orphaned .tmp files
	for _, name := range []string{"orphan1.tmp", "orphan2.tmp", "data.json.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("garbage"), 0644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	// Create a non-tmp file that should be left alone
	if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("WriteFile data.json failed: %v", err)
	}

	removed, errors := CleanupTempFiles(dir)
	if errors > 0 {
		t.Fatalf("CleanupTempFiles reported %d errors", errors)
	}
	if removed != 3 {
		t.Fatalf("Expected 3 files removed, got %d", removed)
	}

	// Verify .tmp files are gone
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("Orphaned .tmp file still present: %s", entry.Name())
		}
	}

	// Verify non-tmp file is still present
	if _, err := os.Stat(filepath.Join(dir, "data.json")); os.IsNotExist(err) {
		t.Errorf("Non-tmp file data.json should still exist")
	}
}

func TestCleanupTempFilesNonExistentDir(t *testing.T) {
	removed, errors := CleanupTempFiles("/nonexistent/path/that/does/not/exist")
	if removed != 0 || errors != 0 {
		t.Fatalf("Expected (0,0) for non-existent dir, got (%d,%d)", removed, errors)
	}
}
