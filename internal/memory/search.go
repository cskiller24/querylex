package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
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
// searchFTS performs a full-text search using the FTS5 index.
// It tokenizes the input and matches against the FTS index, falling back
// to a prefix-based LIKE query on the input column for short terms that
// FTS5 cannot match effectively.
func searchFTS(ctx context.Context, db *sql.DB, normalizedInput string) ([]MemoryEntry, error) {
	inputTokens := tokenize(normalizedInput)
	if len(inputTokens) == 0 {
		return nil, nil
	}

	// Build FTS5 query: each non-empty token as a phrase match
	var ftsTerms []string
	for _, t := range inputTokens {
		if len(t) >= 2 {
			ftsTerms = append(ftsTerms, t)
		}
	}
	if len(ftsTerms) == 0 {
		return nil, nil
	}

	ftsQuery := strings.Join(ftsTerms, " OR ")

	rows, err := db.QueryContext(ctx, `
		SELECT e.id, e.input, e.sql, e.sql_hash, e.match_type,
		       COALESCE(e.optimization_summary, ''),
		       e.created_at, e.updated_at, COALESCE(e.last_used_at, ''), e.database_id
		FROM entries_fts f
		JOIN entries e ON e.rowid = f.rowid
		WHERE entries_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("fts5 search: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(
			&e.ID, &e.Input, &e.SQL, &e.SQLHash,
			&e.MatchType, &e.OptimizationSummary,
			&e.CreatedAt, &e.UpdatedAt, &e.LastUsedAt,
			&e.DatabaseID,
		); err != nil {
			return nil, fmt.Errorf("scan fts entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fts rows iteration: %w", err)
	}

	return entries, nil
}

// Search uses 5-component lexical-only scoring (entity overlap, sql
// structure, intent, filter overlap, recency decay) and skips
// high-frequency tokens (frequency > 50) during keyword lookup.
// Bigram keys are also generated and looked up for better phrase matching.
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

		// Unigram lookup: skip high-frequency stop words (> 50 entries)
		for _, token := range inputTokens {
			if index.TokenFrequency[token] > 50 {
				continue
			}
			if ids, ok := index.KeywordIndex[token]; ok {
				for _, id := range ids {
					candidateIDs[id] = true
				}
			}
		}

		// Bigram lookup: collect candidates from bigram keys
		for j := 0; j < len(inputTokens)-1; j++ {
			bigramKey := "__bigram__" + inputTokens[j] + " " + inputTokens[j+1]
			if ids, ok := index.KeywordIndex[bigramKey]; ok {
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

		// If no candidates from index, use FTS5 fallback
		if len(entries) == 0 {
			ftsEntries, ftsErr := searchFTS(ctx, db, normalizedInput)
			if ftsErr == nil && len(ftsEntries) > 0 {
				entries = ftsEntries
			} else {
				allEntries, listErr := ListEntries(ctx, db)
				if listErr != nil {
					return nil, nil, fmt.Errorf("list entries: %w", listErr)
				}
				entries = allEntries
			}
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
		sim := ComputeSimilarity(normalizedInput, entry, schemaTokens, now)
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
