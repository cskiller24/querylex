package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to path atomically: temp file → fsync → rename.
// The caller MUST acquire an exclusive lock before calling this function.
//
// The temp file is created in the same directory as the target path
// (ensuring the rename is on the same filesystem and thus atomic),
// then fsynced for durability, and finally atomically renamed over
// the target path. The parent directory is fsynced afterwards for
// metadata durability (best-effort on non-macOS).
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmpFile, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		tmpFile.Close()
		return fmt.Errorf("write temp: %w", err)
	}

	// Fsync the temp file for durability
	if err := tmpFile.Sync(); err != nil {
		os.Remove(tmpPath)
		tmpFile.Close()
		return fmt.Errorf("fsync temp: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	// Atomic rename on the same filesystem
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}

	// Fsync parent directory for metadata durability (best-effort on non-macOS)
	if f, err := os.Open(dir); err == nil {
		f.Sync()
		f.Close()
	}

	return nil
}

// AtomicRead reads the file at path. The caller should hold a shared lock.
// Returns nil data with no error if the file does not exist.
func AtomicRead(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // file may not exist yet — not an error
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}
