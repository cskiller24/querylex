package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cskiller24/querylex/internal/format"
)

// ScoredEntry pairs a memory entry with its similarity score.
type ScoredEntry struct {
	Entry      MemoryEntry `json:"entry"`
	Similarity float64     `json:"similarity"`
}

// Search orchestrates a memory search: loads index, checks staleness,
// looks up candidate entries, computes similarity, and returns top results.
//
// The search flow:
//  1. Open memory.sqlite, get current revision
//  2. Read memory_index.json, compare revisions
//  3. If stale → rebuild index from SQLite (or fallback to direct scan)
//  4. Tokenize input → keyword lookup → collect candidates
//  5. Compute similarity for each candidate
//  6. Sort by score descending, return top maxResults
//
// Search uses 4-component lexical-only scoring (no embeddings). For
// embedding-enhanced scoring, use SearchWithEmbeddings.
func Search(dbDir string, input string, maxResults int) ([]ScoredEntry, *format.Warning, error) {
	normalizedInput := NormalizeInput(input)

	// Open store
	db, err := OpenStore(dbDir)
	if err != nil {
		return nil, nil, fmt.Errorf("MEMORY_STORE_UNAVAILABLE: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Get SQLite revision
	sqliteRevision, err := GetRevision(ctx, db)
	if err != nil {
		return nil, nil, fmt.Errorf("get revision: %w", err)
	}

	// Read index
	index, err := ReadIndex(dbDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read index: %w", err)
	}

	var entries []MemoryEntry
	var warning *format.Warning

	// Check for stale index
	indexStale := index == nil || index.Revision != sqliteRevision

	if indexStale {
		// Rebuild index from SQLite
		allEntries, listErr := ListEntries(ctx, db)
		if listErr != nil {
			return nil, nil, fmt.Errorf("list entries for rebuild: %w", listErr)
		}

		rebuildIndex, rebuildErr := RebuildIndex(dbDir, allEntries)
		if rebuildErr == nil {
			rebuildIndex.Revision = sqliteRevision
			if writeErr := WriteIndex(dbDir, rebuildIndex); writeErr == nil {
				index = rebuildIndex
			} else {
				// Index write failed but we can still search — fall through
				entries = allEntries
			}
		} else {
			// Rebuild failed — fall back to direct SQLite scan
			entries = allEntries
		}

		if index == nil {
			// Fallback: scan all entries directly
			if entries == nil {
				entries = allEntries
			}
		}

		warning = &format.Warning{
			Code:    "MEMORY_INDEX_STALE",
			Message: "Memory index was stale and has been rebuilt.",
		}
	}

	// Collect candidate entries
	if index != nil {
		inputTokens := tokenize(normalizedInput)
		candidateIDs := make(map[string]bool)
		for _, token := range inputTokens {
			if ids, ok := index.KeywordIndex[token]; ok {
				for _, id := range ids {
					candidateIDs[id] = true
				}
			}
		}

		if len(candidateIDs) > 0 {
			// Load candidates from SQLite by ID
			for id := range candidateIDs {
				entry, getErr := GetEntryByID(ctx, db, id)
				if getErr == nil && entry != nil {
					entries = append(entries, *entry)
				}
			}
		}

		// If no candidates from index, scan all
		if len(entries) == 0 {
			allEntries, listErr := ListEntries(ctx, db)
			if listErr != nil {
				return nil, nil, fmt.Errorf("list entries: %w", listErr)
			}
			entries = allEntries
		}
	}

	if len(entries) == 0 {
		return []ScoredEntry{}, warning, nil
	}

	// Extract schema tokens for scoring
	now := time.Now().UTC()
	schemaTokens, _ := ExtractSchemaTokens(dbDir)

	// Compute similarity for all candidates (lexical-only)
	scored := make([]ScoredEntry, 0, len(entries))
	for _, entry := range entries {
		sim := ComputeSimilarity(normalizedInput, entry, schemaTokens, now, false, nil, nil)
		scored = append(scored, ScoredEntry{
			Entry:      entry,
			Similarity: sim,
		})
	}

	// Sort by similarity descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})

	// Truncate to maxResults
	if maxResults > 0 && len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	// Update last_used_at on matched entries (best-effort)
	for i := range scored {
		nowStr := now.Format(time.RFC3339)
		_, _ = db.ExecContext(ctx, `UPDATE entries SET last_used_at = ? WHERE id = ?`, nowStr, scored[i].Entry.ID)
	}

	return scored, warning, nil
}

// SearchWithEmbeddings is the embedding-enhanced version of Search.
// It accepts an inputEmbedding vector for 5-component scoring and reads
// entry embeddings from the memory index. When embeddings are available,
// scoring uses: embedding cosine (0.45), entity overlap (0.25), intent
// (0.15), filter (0.10), recency (0.05). When inputEmbedding is nil or
// no embedding vectors exist for an entry, falls back to lexical-only scoring.
func SearchWithEmbeddings(dbDir string, input string, maxResults int, inputEmbedding []float32) ([]ScoredEntry, *format.Warning, error) {
	normalizedInput := NormalizeInput(input)

	db, err := OpenStore(dbDir)
	if err != nil {
		return nil, nil, fmt.Errorf("MEMORY_STORE_UNAVAILABLE: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	sqliteRevision, err := GetRevision(ctx, db)
	if err != nil {
		return nil, nil, fmt.Errorf("get revision: %w", err)
	}

	index, err := ReadIndex(dbDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read index: %w", err)
	}

	var entries []MemoryEntry
	var warning *format.Warning

	indexStale := index == nil || index.Revision != sqliteRevision

	if indexStale {
		allEntries, listErr := ListEntries(ctx, db)
		if listErr != nil {
			return nil, nil, fmt.Errorf("list entries for rebuild: %w", listErr)
		}

		rebuildIndex, rebuildErr := RebuildIndex(dbDir, allEntries)
		if rebuildErr == nil {
			rebuildIndex.Revision = sqliteRevision
			if writeErr := WriteIndex(dbDir, rebuildIndex); writeErr == nil {
				index = rebuildIndex
			} else {
				entries = allEntries
			}
		} else {
			entries = allEntries
		}

		if index == nil {
			if entries == nil {
				entries = allEntries
			}
		}

		warning = &format.Warning{
			Code:    "MEMORY_INDEX_STALE",
			Message: "Memory index was stale and has been rebuilt.",
		}
	}

	if index != nil {
		inputTokens := tokenize(normalizedInput)
		candidateIDs := make(map[string]bool)
		for _, token := range inputTokens {
			if ids, ok := index.KeywordIndex[token]; ok {
				for _, id := range ids {
					candidateIDs[id] = true
				}
			}
		}

		if len(candidateIDs) > 0 {
			for id := range candidateIDs {
				entry, getErr := GetEntryByID(ctx, db, id)
				if getErr == nil && entry != nil {
					entries = append(entries, *entry)
				}
			}
		}

		if len(entries) == 0 {
			allEntries, listErr := ListEntries(ctx, db)
			if listErr != nil {
				return nil, nil, fmt.Errorf("list entries: %w", listErr)
			}
			entries = allEntries
		}
	}

	if len(entries) == 0 {
		return []ScoredEntry{}, warning, nil
	}

	now := time.Now().UTC()
	schemaTokens, _ := ExtractSchemaTokens(dbDir)
	embeddingsActive := inputEmbedding != nil && index != nil && index.EmbeddingVectors != nil

	scored := make([]ScoredEntry, 0, len(entries))
	for _, entry := range entries {
		var entryEmbedding []float32
		if embeddingsActive {
			if meta, ok := index.EmbeddingVectors[entry.ID]; ok {
				entryEmbedding = meta.Vector
			}
		}

		sim := ComputeSimilarity(normalizedInput, entry, schemaTokens, now, embeddingsActive, inputEmbedding, entryEmbedding)
		scored = append(scored, ScoredEntry{
			Entry:      entry,
			Similarity: sim,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Similarity > scored[j].Similarity
	})

	if maxResults > 0 && len(scored) > maxResults {
		scored = scored[:maxResults]
	}

	for i := range scored {
		nowStr := now.Format(time.RFC3339)
		_, _ = db.ExecContext(ctx, `UPDATE entries SET last_used_at = ? WHERE id = ?`, nowStr, scored[i].Entry.ID)
	}

	return scored, warning, nil
}
