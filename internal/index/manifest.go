package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cskiller24/querylex/internal/state"
)

// IndexManifest records metadata and checksums for indexing artifacts.
type IndexManifest struct {
	SchemaVersionHash string            `json:"schema_version_hash"`
	DBVersion         string            `json:"db_version"`
	TableCount        int               `json:"table_count"`
	ArtifactChecksums map[string]string `json:"artifact_checksums"`
	GeneratedAt       string            `json:"generated_at"`
}

// manifestPath returns the path to the index_manifest.json file.
func manifestPath(dbDir string) string {
	return filepath.Join(dbDir, "indexes", "index_manifest.json")
}

// ComputeChecksum computes the SHA-256 hex digest of a file.
func ComputeChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file for checksum: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// BuildManifest constructs an IndexManifest from artifact paths.
func BuildManifest(dbDir string, dbVersion string, tableCount int, artifactPaths ...string) (*IndexManifest, error) {
	checksums := make(map[string]string, len(artifactPaths))

	for _, artifactPath := range artifactPaths {
		fullPath := filepath.Join(dbDir, artifactPath)
		checksum, err := ComputeChecksum(fullPath)
		if err != nil {
			return nil, fmt.Errorf("compute checksum for %s: %w", artifactPath, err)
		}
		checksums[artifactPath] = checksum
	}

	return &IndexManifest{
		SchemaVersionHash: computeSchemaVersionHash(checksums),
		DBVersion:         dbVersion,
		TableCount:        tableCount,
		ArtifactChecksums: checksums,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// computeSchemaVersionHash produces a combined hash from all artifact checksums.
func computeSchemaVersionHash(checksums map[string]string) string {
	// Combine all checksums into a single hash
	combined := ""
	for _, path := range sortedStringKeys(checksums) {
		combined += path + ":" + checksums[path] + "\n"
	}
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// sortedStringKeys returns sorted keys from a string map.
func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteIndexManifest writes the manifest atomically to
// <dbDir>/indexes/index_manifest.json.
func WriteIndexManifest(dbDir string, manifest *IndexManifest) error {
	path := manifestPath(dbDir)

	if err := os.MkdirAll(filepath.Join(dbDir, "indexes"), 0755); err != nil {
		return fmt.Errorf("create indexes dir: %w", err)
	}

	lock := state.NewFileLock(path)
	if err := lock.Acquire(state.LockExclusive); err != nil {
		return fmt.Errorf("acquire exclusive lock for manifest: %w", err)
	}
	defer lock.Release()

	manifest.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := state.AtomicWrite(path, data); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

// ReadIndexManifest reads the manifest from <dbDir>/indexes/index_manifest.json.
// Returns nil, nil if the file does not exist.
func ReadIndexManifest(dbDir string) (*IndexManifest, error) {
	path := manifestPath(dbDir)

	// Check if file exists before trying to lock — avoids lock errors on missing dir
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	lock := state.NewFileLock(path)
	if err := lock.Acquire(state.LockShared); err != nil {
		return nil, fmt.Errorf("acquire shared lock for manifest: %w", err)
	}
	defer lock.Release()

	data, err := state.AtomicRead(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	var manifest IndexManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &manifest, nil
}

// VerifyManifest verifies that all artifact checksums in the manifest
// match the current files on disk. Returns true if all match.
func VerifyManifest(dbDir string, manifest *IndexManifest) (bool, error) {
	for filename, expectedHash := range manifest.ArtifactChecksums {
		fullPath := filepath.Join(dbDir, filename)

		// Check path traversal: ensure resolved path stays within dbDir
		cleanPath := filepath.Clean(fullPath)
		cleanDir := filepath.Clean(dbDir)
		if !hasPrefix(cleanPath, cleanDir+string(filepath.Separator)) && cleanPath != cleanDir {
			return false, fmt.Errorf("path traversal detected: %s", filename)
		}

		// Check if file exists before checksum computation
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return false, nil // artifact missing = stale
		}

		actualHash, err := ComputeChecksum(fullPath)
		if err != nil {
			return false, fmt.Errorf("compute checksum for %s: %w", filename, err)
		}

		if actualHash != expectedHash {
			return false, nil // checksum mismatch = stale
		}
	}

	return true, nil
}

// hasPrefix checks if s has the given prefix string.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
