package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/querylex/querylex/internal/db"
)

// JoinGraphResult holds the complete join graph with metadata.
type JoinGraphResult struct {
	Edges            []db.JoinEdge `json:"edges"`
	GeneratedAt      string        `json:"generated_at"`
	TableCount       int           `json:"table_count"`
	DeclaredFKCount  int           `json:"declared_fk_count"`
	InferredJoinCount int          `json:"inferred_join_count"`
}

// BuildJoinGraph transforms a SchemaResult into a JoinGraphResult.
// Extracts declared foreign keys (confidence=1.0) and infers joins via
// column-name pattern matching (_id suffix → table name).
// Deduplicates: if both declared FK and inferred exist for same (source, target),
// only the declared FK is kept.
func BuildJoinGraph(result *db.SchemaResult) (*JoinGraphResult, error) {
	graph := &JoinGraphResult{
		Edges:      []db.JoinEdge{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TableCount: len(result.Tables),
	}

	// Build table name set for lookup
	tableNames := make(map[string]bool)
	for _, t := range result.Tables {
		tableNames[t.Name] = true
	}

	// Track existing (source, target) pairs to avoid duplicates
	edgeKeys := make(map[string]bool)

	// Step 1: Extract declared FKs
	for _, t := range result.Tables {
		for _, c := range t.Constraints {
			if c.Type == "FOREIGN_KEY" {
				columns := make([][2]string, len(c.Columns))
				for i := range c.Columns {
					var refCol string
					if i < len(c.ReferencedColumns) {
						refCol = c.ReferencedColumns[i]
					}
					columns[i] = [2]string{c.Columns[i], refCol}
				}

				edge := db.JoinEdge{
					Source:     t.Name,
					Target:     c.ReferencedTable,
					Columns:    columns,
					Confidence: 1.0,
					SourceType: "declared_foreign_key",
					Composite:  len(c.Columns) > 1,
				}
				graph.Edges = append(graph.Edges, edge)
				key := edgeKey(t.Name, c.ReferencedTable)
				edgeKeys[key] = true
				graph.DeclaredFKCount++
			}
		}
	}

	// Step 2: Infer joins via column-name pattern matching
	for _, t := range result.Tables {
		for _, col := range t.Columns {
			// Look for _id suffix
			if !strings.HasSuffix(col.Name, "_id") {
				continue
			}
			prefix := strings.TrimSuffix(col.Name, "_id")
			if prefix == "" || col.Name == "id" {
				continue
			}

			// Check for matching table
			var matchedTable string
			var confidence float64

			// Check exact match (e.g., user_id → user)
			if tableNames[prefix] {
				matchedTable = prefix
				confidence = 0.85
			} else if tableNames[prefix+"s"] {
				// Check plural match (e.g., user_id → users)
				matchedTable = prefix + "s"
				confidence = 0.70
			} else {
				// Check irregular plurals: try common patterns
				matchedTable = findIrregularPlural(prefix, tableNames)
				if matchedTable != "" {
					confidence = 0.60
				}
			}

			if matchedTable == "" {
				continue
			}

			// Skip if same table (self-reference already handled by declared FK)
			if matchedTable == t.Name {
				continue
			}

			// Skip if duplicate of a declared FK for same (source, target)
			key := edgeKey(t.Name, matchedTable)
			if edgeKeys[key] {
				continue
			}

			edge := db.JoinEdge{
				Source:     t.Name,
				Target:     matchedTable,
				Columns:    [][2]string{{col.Name, "id"}},
				Confidence: confidence,
				SourceType: "inferred_naming_match",
				Composite:  false,
			}
			graph.Edges = append(graph.Edges, edge)
			edgeKeys[key] = true
			graph.InferredJoinCount++
		}
	}

	return graph, nil
}

// findIrregularPlural attempts to match irregular plural forms.
// This handles common English plural patterns but is intentionally
// conservative to avoid false positives.
func findIrregularPlural(prefix string, tableNames map[string]bool) string {
	// Try common irregular patterns
	candidates := []string{
		prefix + "es",     // e.g., box → boxes
		prefix + "ies",    // e.g., category → categories (would need singular truncation)
		prefix[:len(prefix)-1] + "ies", // e.g., category → categories (if prefix ends in 'y')
	}

	// If prefix ends in 'y', try -ies
	if strings.HasSuffix(prefix, "y") && len(prefix) > 1 {
		base := prefix[:len(prefix)-1]
		candidates = append(candidates, base+"ies")
	}

	for _, c := range candidates {
		if tableNames[c] {
			return c
		}
	}

	return ""
}

// edgeKey creates a deterministic key for (source, target) deduplication.
func edgeKey(source, target string) string {
	return source + "→" + target
}

// FindJoinPath performs BFS on the join graph to find the shortest path
// between two tables. Prefers declared FK edges (Confidence=1.0) over
// inferred edges when multiple paths exist.
func FindJoinPath(from, to string, graph *JoinGraphResult) ([]string, error) {
	if len(graph.Edges) == 0 {
		return nil, fmt.Errorf("no join path found from %s to %s", from, to)
	}

	// Build adjacency list
	type neighbor struct {
		table      string
		confidence float64
	}
	adjacency := make(map[string][]neighbor)

	for _, e := range graph.Edges {
		adjacency[e.Source] = append(adjacency[e.Source], neighbor{
			table:      e.Target,
			confidence: e.Confidence,
		})
		adjacency[e.Target] = append(adjacency[e.Target], neighbor{
			table:      e.Source,
			confidence: e.Confidence,
		})
	}

	// BFS
	type node struct {
		table string
		path  []string
	}

	visited := make(map[string]bool)
	queue := []node{{table: from, path: []string{from}}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.table == to {
			return current.path, nil
		}

		neighbors := adjacency[current.table]
		// Sort: prefer higher confidence first
		sortedNeighbors := make([]neighbor, len(neighbors))
		copy(sortedNeighbors, neighbors)
		// Simple bubble sort by confidence (descending) — small graphs only
		for i := 0; i < len(sortedNeighbors); i++ {
			for j := i + 1; j < len(sortedNeighbors); j++ {
				if sortedNeighbors[j].confidence > sortedNeighbors[i].confidence {
					sortedNeighbors[i], sortedNeighbors[j] = sortedNeighbors[j], sortedNeighbors[i]
				}
			}
		}

		for _, n := range sortedNeighbors {
			if !visited[n.table] {
				visited[n.table] = true
				newPath := make([]string, len(current.path)+1)
				copy(newPath, current.path)
				newPath[len(current.path)] = n.table
				queue = append(queue, node{table: n.table, path: newPath})
			}
		}
	}

	return nil, fmt.Errorf("no join path found from %s to %s", from, to)
}

// WriteJoinGraph marshals the JoinGraphResult to JSON and writes it to
// <dbDir>/schema/join_graph.json.
func WriteJoinGraph(dbDir string, graph *JoinGraphResult) error {
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal join graph: %w", err)
	}

	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}

	return os.WriteFile(filepath.Join(schemaDir, "join_graph.json"), data, 0644)
}
