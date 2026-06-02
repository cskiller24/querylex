package memory

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/querylex/querylex/internal/state"
)

// MemoryIndex represents the derived keyword index for memory search.
// It is rebuildable from the SQLite store and tracks revision for staleness detection.
type MemoryIndex struct {
	Revision     int                 `json:"revision"`
	EntryCount   int                 `json:"entry_count"`
	KeywordIndex map[string][]string `json:"keyword_index"` // token → entry IDs
	LastRebuiltAt string             `json:"last_rebuilt_at"`
	SchemaVersion int                `json:"schema_version"`
}

// ReadIndex reads the memory_index.json file from the given dbDir.
// Returns nil, nil if the file does not exist.
func ReadIndex(dbDir string) (*MemoryIndex, error) {
	path := indexPath(dbDir)
	data, err := state.AtomicRead(path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var idx MemoryIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	return &idx, nil
}

// WriteIndex writes the memory_index.json file atomically using state.AtomicWrite.
func WriteIndex(dbDir string, index *MemoryIndex) error {
	index.LastRebuiltAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return state.AtomicWrite(indexPath(dbDir), data)
}

// RebuildIndex builds a MemoryIndex from a slice of MemoryEntry values.
// It tokenizes each entry's input and builds a keyword_index mapping token → entry IDs.
func RebuildIndex(dbDir string, entries []MemoryEntry) (*MemoryIndex, error) {
	index := &MemoryIndex{
		EntryCount:    len(entries),
		KeywordIndex:  make(map[string][]string),
		SchemaVersion: 1,
	}

	for i := range entries {
		entry := entries[i]
		tokens := tokenize(entry.Input)
		seen := make(map[string]bool)
		for _, token := range tokens {
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			index.KeywordIndex[token] = append(index.KeywordIndex[token], entry.ID)
		}
	}

	return index, nil
}

// indexPath returns the path to the memory_index.json file.
func indexPath(dbDir string) string {
	return filepath.Join(dbDir, "memory_index.json")
}

// tokenize splits a string into lowercase tokens, removing punctuation.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.Fields(s)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		// Strip leading/trailing punctuation
		clean := strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if clean != "" {
			tokens = append(tokens, clean)
		}
	}
	return tokens
}
