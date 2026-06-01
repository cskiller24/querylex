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
