package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexStatus_WriteReadRoundtrip(t *testing.T) {
	dir := t.TempDir()

	status := &IndexStatus{
		Status:          "indexed",
		CurrentPhase:    "output_assembly",
		ProgressPercent: 100,
		HeartbeatAt:     "2024-01-15T10:30:00Z",
		StartedAt:       "2024-01-15T10:00:00Z",
		CompletedAt:     "2024-01-15T10:30:00Z",
	}

	if err := WriteIndexStatus(dir, status); err != nil {
		t.Fatalf("WriteIndexStatus failed: %v", err)
	}

	// Verify file exists
	statusPath := filepath.Join(dir, "indexes", "index_status.json")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Fatal("index_status.json not written")
	}

	// Read back
	readStatus, err := ReadIndexStatus(dir)
	if err != nil {
		t.Fatalf("ReadIndexStatus failed: %v", err)
	}
	if readStatus == nil {
		t.Fatal("ReadIndexStatus returned nil")
	}
	if readStatus.Status != "indexed" {
		t.Errorf("expected status='indexed', got '%s'", readStatus.Status)
	}
	if readStatus.CurrentPhase != "output_assembly" {
		t.Errorf("expected current_phase='output_assembly', got '%s'", readStatus.CurrentPhase)
	}
	if readStatus.ProgressPercent != 100 {
		t.Errorf("expected progress_percent=100, got %d", readStatus.ProgressPercent)
	}
	if readStatus.HeartbeatAt != "2024-01-15T10:30:00Z" {
		t.Errorf("expected heartbeat_at='2024-01-15T10:30:00Z', got '%s'", readStatus.HeartbeatAt)
	}
	if readStatus.StartedAt != "2024-01-15T10:00:00Z" {
		t.Errorf("expected started_at='2024-01-15T10:00:00Z', got '%s'", readStatus.StartedAt)
	}
	if readStatus.CompletedAt != "2024-01-15T10:30:00Z" {
		t.Errorf("expected completed_at='2024-01-15T10:30:00Z', got '%s'", readStatus.CompletedAt)
	}
}

func TestIndexStatus_MissingFile(t *testing.T) {
	dir := t.TempDir()

	// No indexes/ directory at all — should return not_indexed
	status, err := ReadIndexStatus(dir)
	if err != nil {
		t.Fatalf("ReadIndexStatus on missing file failed: %v", err)
	}
	if status == nil {
		t.Fatal("ReadIndexStatus returned nil for missing file")
	}
	if status.Status != "not_indexed" {
		t.Errorf("expected status='not_indexed' for missing file, got '%s'", status.Status)
	}
}

func TestIndexStatus_ErrorField(t *testing.T) {
	dir := t.TempDir()

	status := &IndexStatus{
		Status:          "index_failed",
		CurrentPhase:    "schema_extraction",
		ProgressPercent: 10,
		HeartbeatAt:     "2024-01-15T10:05:00Z",
		StartedAt:       "2024-01-15T10:00:00Z",
		Error:           "connection timeout",
	}

	if err := WriteIndexStatus(dir, status); err != nil {
		t.Fatalf("WriteIndexStatus failed: %v", err)
	}

	readStatus, err := ReadIndexStatus(dir)
	if err != nil {
		t.Fatalf("ReadIndexStatus failed: %v", err)
	}
	if readStatus.Status != "index_failed" {
		t.Errorf("expected status='index_failed', got '%s'", readStatus.Status)
	}
	if readStatus.Error != "connection timeout" {
		t.Errorf("expected error='connection timeout', got '%s'", readStatus.Error)
	}
}
