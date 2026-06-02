package state

import (
	"os"
	"path/filepath"
	"strings"
)

// CleanupTempFiles removes orphaned .tmp files from the workspace directory.
// These files may be left behind if the process crashes mid-write.
// Called during workspace initialization (PersistentPreRunE).
//
// Returns the number of files successfully removed and the number of
// removal errors encountered.
func CleanupTempFiles(workspaceDir string) (removed int, errors int) {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return 0, 0 // directory may not exist yet
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tmp") {
			path := filepath.Join(workspaceDir, entry.Name())
			if err := os.Remove(path); err == nil {
				removed++
			} else {
				errors++
			}
		}
	}
	return
}

// CleanupLockFiles removes orphaned .lock files from the workspace directory.
// A .lock file is considered orphaned if no other process holds an exclusive
// lock on it. This is verified by opening the file and probing with a
// non-blocking exclusive lock acquisition — if the lock can be acquired, the
// file is stale (the creating process no longer holds it).
//
// Called during workspace initialization (PersistentPreRunE).
//
// Returns the number of files successfully removed and the number of
// removal errors encountered.
func CleanupLockFiles(workspaceDir string) (removed int, errors int) {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return 0, 0 // directory may not exist yet
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}
		lockPath := filepath.Join(workspaceDir, entry.Name())

		// Open the lock file — if it fails, skip (may have been removed by another process).
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0644)
		if err != nil {
			continue
		}

		// Probe with non-blocking exclusive lock.
		// If acquire succeeds, no other process holds the lock → it's orphaned.
		err = acquireFlock(f.Fd(), LockExclusive)
		if err != nil {
			// Lock is held by another process — leave it alone.
			f.Close()
			continue
		}

		// Acquired the lock — release it, close the file, and remove the orphaned lock file.
		_ = releaseFlock(f.Fd())
		f.Close()

		if err := os.Remove(lockPath); err == nil {
			removed++
		} else {
			errors++
		}
	}
	return
}
