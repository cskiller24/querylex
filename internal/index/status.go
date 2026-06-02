package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cskiller24/querylex/internal/state"
)

// IndexStatus tracks the progress of database indexing.
type IndexStatus struct {
	Status          string `json:"status"`
	CurrentPhase    string `json:"current_phase"`
	ProgressPercent int    `json:"progress_percent"`
	HeartbeatAt     string `json:"heartbeat_at"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at,omitempty"`
	Error           string `json:"error,omitempty"`
}

// NewIndexStatus creates a new IndexStatus with the given status and phase.
func NewIndexStatus(status string, phase string) *IndexStatus {
	now := time.Now().UTC().Format(time.RFC3339)
	return &IndexStatus{
		Status:          status,
		CurrentPhase:    phase,
		ProgressPercent: 0,
		HeartbeatAt:     now,
		StartedAt:       now,
	}
}

// statusPath returns the path to the index_status.json file.
func statusPath(dbDir string) string {
	return filepath.Join(dbDir, "indexes", "index_status.json")
}

// ReadIndexStatus reads the index status from <dbDir>/indexes/index_status.json.
// Returns a status with "not_indexed" if the file does not exist.
func ReadIndexStatus(dbDir string) (*IndexStatus, error) {
	path := statusPath(dbDir)

	// Check if file exists before trying to lock — avoids lock errors on missing dir
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &IndexStatus{Status: "not_indexed"}, nil
	}

	lock := state.NewFileLock(path)
	if err := lock.Acquire(state.LockShared); err != nil {
		return nil, fmt.Errorf("acquire shared lock for status: %w", err)
	}
	defer lock.Release()

	data, err := state.AtomicRead(path)
	if err != nil {
		return nil, fmt.Errorf("read index status: %w", err)
	}
	if data == nil {
		return &IndexStatus{Status: "not_indexed"}, nil
	}

	var status IndexStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse index status: %w", err)
	}

	return &status, nil
}

// WriteIndexStatus writes the index status atomically to
// <dbDir>/indexes/index_status.json.
func WriteIndexStatus(dbDir string, status *IndexStatus) error {
	path := statusPath(dbDir)

	// Create indexes directory if needed
	if err := os.MkdirAll(filepath.Join(dbDir, "indexes"), 0755); err != nil {
		return fmt.Errorf("create indexes dir: %w", err)
	}

	lock := state.NewFileLock(path)
	if err := lock.Acquire(state.LockExclusive); err != nil {
		return fmt.Errorf("acquire exclusive lock for status: %w", err)
	}
	defer lock.Release()

	// Set heartbeat timestamp
	status.HeartbeatAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal index status: %w", err)
	}

	if err := state.AtomicWrite(path, data); err != nil {
		return fmt.Errorf("write index status: %w", err)
	}

	return nil
}
