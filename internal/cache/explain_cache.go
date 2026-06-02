package explaincache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/index"
	"github.com/querylex/querylex/internal/state"
)

// CacheEntry represents a single cached explain plan entry stored on disk.
type CacheEntry struct {
	Plan          *db.ExplainPlan `json:"plan"`
	NormalizedSQL string          `json:"normalized_sql"`
	Fingerprint   CacheFingerprint `json:"fingerprint"`
	CreatedAt     string          `json:"created_at"`
	TTLSeconds    int             `json:"ttl_seconds"`
	Analyze       bool            `json:"analyze"`
}

// CacheFingerprint holds the composite fingerprint used for cache invalidation.
// When any of these values changes, the cache entry is invalidated at read time.
type CacheFingerprint struct {
	DBType              string `json:"db_type"`
	DBVersion           string `json:"db_version"`
	ExplainMode         string `json:"explain_mode"` // "estimated" or "analyze"
	SchemaManifestHash  string `json:"schema_manifest_hash"`
	IndexManifestHash   string `json:"index_manifest_hash"`
	StatsFreshness      string `json:"stats_freshness,omitempty"`       // nullable, not tracked in Phase 4
	SessionSettingsHash string `json:"session_settings_hash,omitempty"` // nullable, not tracked in Phase 4
}

var whitespaceRegexp = regexp.MustCompile(`\s+`)

// normalizeSQL trims whitespace and collapses multiple spaces to a single space.
func normalizeSQL(sql string) string {
	s := strings.TrimSpace(sql)
	return whitespaceRegexp.ReplaceAllString(s, " ")
}

// sqlHash computes a short SHA-256 hex prefix of the normalized SQL string.
// Format: "sha256:" + first 12 hex characters (6 bytes).
func sqlHash(normalizedSQL string) string {
	h := sha256.Sum256([]byte(normalizedSQL))
	return "sha256:" + hex.EncodeToString(h[:6])
}

// cachePath returns the file path for a cache entry given the database directory
// and SQL hash: <dbDir>/explain_cache/<sqlHash>.json
func cachePath(dbDir, sqlHash string) string {
	return filepath.Join(dbDir, "explain_cache", sqlHash+".json")
}

// computeFingerprint builds a CacheFingerprint from the current index manifest.
// Returns nil, nil if the manifest is not available (skip caching).
func computeFingerprint(dbDir string, analyze bool, dbType string) (*CacheFingerprint, error) {
	manifest, err := index.ReadIndexManifest(dbDir)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		return nil, nil
	}

	// Compute IndexManifestHash from serialized ArtifactChecksums
	var indexHash string
	if manifest.ArtifactChecksums != nil {
		checksumData, err := json.Marshal(manifest.ArtifactChecksums)
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(checksumData)
		indexHash = hex.EncodeToString(h[:])
	}

	explainMode := "estimated"
	if analyze {
		explainMode = "analyze"
	}

	return &CacheFingerprint{
		DBType:              dbType,
		DBVersion:           manifest.DBVersion,
		ExplainMode:         explainMode,
		SchemaManifestHash:  manifest.SchemaVersionHash,
		IndexManifestHash:   indexHash,
		StatsFreshness:      "",
		SessionSettingsHash: "",
	}, nil
}

// fingerprintsMatch compares two cache fingerprints field-by-field.
// StatsFreshness and SessionSettingsHash are ignored in Phase 4 (always empty).
func fingerprintsMatch(a, b *CacheFingerprint) bool {
	if a == nil || b == nil {
		return false
	}
	return a.DBType == b.DBType &&
		a.DBVersion == b.DBVersion &&
		a.ExplainMode == b.ExplainMode &&
		a.SchemaManifestHash == b.SchemaManifestHash &&
		a.IndexManifestHash == b.IndexManifestHash
}

// Check looks up a cached explain plan for the given SQL and database.
// Returns the cached plan and true on cache hit, or nil and false on miss.
// Errors (missing file, parse failure, TTL expiry, fingerprint mismatch) are
// treated as cache misses — never returned as errors.
func Check(dbDir string, sql string, analyze bool, dbType string) (*db.ExplainPlan, bool) {
	normalized := normalizeSQL(sql)
	hash := sqlHash(normalized)

	// Compute current fingerprint — nil means cannot cache
	fp, err := computeFingerprint(dbDir, analyze, dbType)
	if err != nil || fp == nil {
		return nil, false
	}

	// Check file exists
	path := cachePath(dbDir, hash)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, false
	}

	// Read cache entry
	data, err := state.AtomicRead(path)
	if err != nil || data == nil {
		return nil, false
	}

	// Parse cache entry
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Fingerprint comparison — cache invalidation on schema/index change
	if !fingerprintsMatch(fp, &entry.Fingerprint) {
		return nil, false
	}

	// TTL check
	createdAt, err := time.Parse(time.RFC3339, entry.CreatedAt)
	if err != nil {
		return nil, false
	}

	elapsed := time.Since(createdAt).Seconds()
	if elapsed > float64(entry.TTLSeconds) {
		return nil, false
	}

	return entry.Plan, true
}

// Write persists a new cache entry after a successful adapter.Explain call.
// If the fingerprint cannot be computed (no manifest), the write is silently skipped.
// The write is atomic via state.AtomicWrite.
func Write(dbDir string, plan *db.ExplainPlan, normalizedSQL string, analyze bool, dbType string) error {
	fp, err := computeFingerprint(dbDir, analyze, dbType)
	if err != nil || fp == nil {
		return nil // silently skip — can't cache without manifest
	}

	hash := sqlHash(normalizedSQL)

	// TTL: 86400s (24h) for estimated, 900s (15m) for analyze
	ttl := 86400
	if analyze {
		ttl = 900
	}

	entry := CacheEntry{
		Plan:          plan,
		NormalizedSQL: normalizedSQL,
		Fingerprint:   *fp,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		TTLSeconds:    ttl,
		Analyze:       analyze,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(dbDir, "explain_cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	return state.AtomicWrite(cachePath(dbDir, hash), data)
}

// CleanupStale removes cache entries older than 7 days.
// Returns the number of entries removed. Skips non-JSON files and read errors.
func CleanupStale(dbDir string) (int, error) {
	cacheDir := filepath.Join(dbDir, "explain_cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0, err
	}

	removed := 0
	maxAge := 7 * 24 * time.Hour
	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fullPath := filepath.Join(cacheDir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue // skip unreadable files
		}

		var ce CacheEntry
		if err := json.Unmarshal(data, &ce); err != nil {
			// Unparseable entry — remove it
			os.Remove(fullPath)
			removed++
			continue
		}

		createdAt, err := time.Parse(time.RFC3339, ce.CreatedAt)
		if err != nil {
			// Can't parse time — remove as stale
			os.Remove(fullPath)
			removed++
			continue
		}

		if now.Sub(createdAt) > maxAge {
			if err := os.Remove(fullPath); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// CountEntries counts the number of .json cache entry files in explain_cache/.
// Returns 0 if the directory does not exist.
func CountEntries(dbDir string) int {
	cacheDir := filepath.Join(dbDir, "explain_cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	return count
}

// DirSize returns the total size in bytes of all .json cache entry files.
// Returns 0 if the directory does not exist.
func DirSize(dbDir string) int64 {
	cacheDir := filepath.Join(dbDir, "explain_cache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0
	}

	var total int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total
}
