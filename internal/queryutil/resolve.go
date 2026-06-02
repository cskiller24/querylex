package queryutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ColumnCandidate describes a resolved column match.
type ColumnCandidate struct {
	Name       string  `json:"name"`
	Table      string  `json:"table"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// TableCandidate describes a resolved table match.
type TableCandidate struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	MatchType   string  `json:"match_type"` // "exact", "terminology", "substring", "fuzzy", "semantic"
	MatchedTerm string  `json:"matched_term"`
}

// ResolveResult holds the complete result of multi-pass token resolution.
type ResolveResult struct {
	Tables     []TableCandidate   `json:"tables"`
	Columns    []ColumnCandidate  `json:"columns"`
	Confidence float64            `json:"confidence"`
}

// slimSchemaTable mirrors the output of BuildSlimSchema for parsing.
type slimSchemaTable struct {
	Name    string           `json:"name"`
	Columns []slimSchemaCol  `json:"columns"`
}

type slimSchemaCol struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type slimSchema struct {
	Tables []slimSchemaTable `json:"tables"`
}

// LevenshteinDistance computes the edit distance between two strings.
// Implements Wagner-Fischer algorithm. O(m*n) time, O(min(m,n)) space.
// Strings longer than 100 characters return max(len(s1), len(s2)) as
// approximate to avoid excessive allocation.
func LevenshteinDistance(s1, s2 string) int {
	// Identical strings always return 0 regardless of length
	if s1 == s2 {
		return 0
	}
	if len(s1) > 100 && len(s2) > 100 {
		if len(s1) > len(s2) {
			return len(s1)
		}
		return len(s2)
	}
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Ensure s1 is the shorter string for space optimization
	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	prev := make([]int, len(s1)+1)
	curr := make([]int, len(s1)+1)
	for i := 0; i <= len(s1); i++ {
		prev[i] = i
	}
	for j := 1; j <= len(s2); j++ {
		curr[0] = j
		for i := 1; i <= len(s1); i++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			curr[i] = min(prev[i]+1, min(curr[i-1]+1, prev[i-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(s1)]
}

// ResolveTokens performs multi-pass deterministic resolution of natural language
// tokens against a slim schema. Returns ranked table/column candidates with
// confidence scores.
func ResolveTokens(input string, slimSchemaData []byte) (*ResolveResult, error) {
	// Parse slim schema
	var schema slimSchema
	if err := json.Unmarshal(slimSchemaData, &schema); err != nil {
		return nil, fmt.Errorf("SCHEMA_PARSE_ERROR: failed to parse slim schema: %w", err)
	}

	result := &ResolveResult{
		Tables:  []TableCandidate{},
		Columns: []ColumnCandidate{},
	}

	if len(schema.Tables) == 0 {
		return result, nil
	}

	// Build lookup indices
	tableNames := make([]string, len(schema.Tables))
	tableSet := make(map[string]bool)
	columnsByTable := make(map[string][]slimSchemaCol)
	for i, t := range schema.Tables {
		tableNames[i] = strings.ToLower(t.Name)
		tableSet[t.Name] = true
		columnsByTable[t.Name] = t.Columns
	}

	// Build column lookup: column name → list of (table, type)
	allColumns := make(map[string][]slimSchemaCol)
	columnTables := make(map[string][]string)
	columnTypes := make(map[string]string) // column name → type (first encountered)
	for _, t := range schema.Tables {
		for _, c := range t.Columns {
			colLower := strings.ToLower(c.Name)
			allColumns[colLower] = append(allColumns[colLower], c)
			columnTables[colLower] = append(columnTables[colLower], t.Name)
			if _, ok := columnTypes[colLower]; !ok {
				columnTypes[colLower] = strings.ToLower(c.Type)
			}
		}
	}

	// Tokenize input
	tokens := tokenize(input)

	// Track matched tables to avoid duplicates
	matchedTables := make(map[string]bool)
	bestScores := make(map[string]float64)
	bestMatchTypes := make(map[string]string)
	bestMatchTerms := make(map[string]string)

	// --- Pass 1: Exact match (case-insensitive) ---
	for _, token := range tokens {
		tokenLower := strings.ToLower(token)
		for _, t := range schema.Tables {
			tableLower := strings.ToLower(t.Name)
			// Exact character-for-character match (highest confidence)
			if tableLower == tokenLower {
				key := t.Name
				score := 0.95
				if score > bestScores[key] {
					bestScores[key] = score
					bestMatchTypes[key] = "exact"
					bestMatchTerms[key] = token
				}
				matchedTables[key] = true
				continue
			}
			// Singularized exact match: token "customer" matches "customers", etc.
			// Check if token + "s" == table name (token is singular, table is plural)
			// Or if table name + "s" == token (table is singular, token is plural)
			if tokenLower+"s" == tableLower || tableLower+"s" == tokenLower {
				key := t.Name
				score := 0.85
				if score > bestScores[key] {
					bestScores[key] = score
					bestMatchTypes[key] = "exact"
					bestMatchTerms[key] = token
				}
				matchedTables[key] = true
			}
		}
		// Also check column names for exact match
		for colName, tables := range columnTables {
			if colName == tokenLower {
				for _, table := range tables {
					key := table
					score := 0.95
					if score > bestScores[key] {
						bestScores[key] = score
						bestMatchTypes[key] = "exact"
						bestMatchTerms[key] = token
					}
					matchedTables[key] = true
				}
			}
		}
	}

	// --- Pass 2: Terminology map lookup ---
	// (Phase 3 artifact — skip when terminologies.md is unavailable)

	// --- Pass 3: Substring match ---
	for _, token := range tokens {
		tokenLower := strings.ToLower(token)
		if len(tokenLower) < 2 {
			continue
		}
		for _, t := range schema.Tables {
			key := t.Name
			if matchedTables[key] && bestScores[key] >= 0.85 {
				continue // Already matched with higher score
			}
			tableLower := strings.ToLower(t.Name)
			if strings.Contains(tableLower, tokenLower) || strings.Contains(tokenLower, tableLower) {
				if len(tokenLower) >= 2 && len(tableLower) >= 2 {
					score := 0.70
					if score > bestScores[key] {
						bestScores[key] = score
						bestMatchTypes[key] = "substring"
						bestMatchTerms[key] = token
					}
					matchedTables[key] = true
				}
			}
		}
		// Also check column names for substring
		for colName, tables := range columnTables {
			if strings.Contains(colName, tokenLower) || strings.Contains(tokenLower, colName) {
				if len(tokenLower) >= 2 && len(colName) >= 2 {
					for _, table := range tables {
						key := table
						if matchedTables[key] && bestScores[key] >= 0.70 {
							continue
						}
						score := 0.70
						if score > bestScores[key] {
							bestScores[key] = score
							bestMatchTypes[key] = "substring"
							bestMatchTerms[key] = token
						}
						matchedTables[key] = true
					}
				}
			}
		}
	}

	// --- Pass 4: Fuzzy match (Levenshtein ≤ 2) ---
	for _, token := range tokens {
		tokenLower := strings.ToLower(token)
		if len(tokenLower) < 2 {
			continue
		}
		// Check against table names
		for _, t := range schema.Tables {
			key := t.Name
			if matchedTables[key] && bestScores[key] >= 0.50 {
				continue // Already matched with better or equal score
			}
			tableLower := strings.ToLower(t.Name)
			if len(tokenLower) > 100 || len(tableLower) > 100 {
				continue
			}
			// Limit fuzzy matching to names with length diff ≤ 4 (DoS protection)
			diff := len(tokenLower) - len(tableLower)
			if diff < 0 {
				diff = -diff
			}
			if diff > 4 {
				continue
			}
			if LevenshteinDistance(tokenLower, tableLower) <= 2 {
				score := 0.50
				if score > bestScores[key] {
					bestScores[key] = score
					bestMatchTypes[key] = "fuzzy"
					bestMatchTerms[key] = token
				}
				matchedTables[key] = true
			}
		}
		// Check against column names
		for colName, tables := range columnTables {
			if len(tokenLower) > 100 || len(colName) > 100 {
				continue
			}
			diff := len(tokenLower) - len(colName)
			if diff < 0 {
				diff = -diff
			}
			if diff > 4 {
				continue
			}
			if LevenshteinDistance(tokenLower, colName) <= 2 {
				for _, table := range tables {
					key := table
					if matchedTables[key] && bestScores[key] >= 0.50 {
						continue
					}
					score := 0.50
					if score > bestScores[key] {
						bestScores[key] = score
						bestMatchTypes[key] = "fuzzy"
						bestMatchTerms[key] = token
					}
					matchedTables[key] = true
				}
			}
		}
	}

	// --- Pass 5: Semantic hints ---
	for _, token := range tokens {
		tokenLower := strings.ToLower(token)
		for _, t := range schema.Tables {
			key := t.Name
			// "id" → primary key columns (columns named "id")
			if (tokenLower == "id" || tokenLower == "ids") && !matchedTables[key] {
				for _, c := range t.Columns {
					if strings.ToLower(c.Name) == "id" {
						score := 0.50
						if score > bestScores[key] {
							bestScores[key] = score
							bestMatchTypes[key] = "semantic"
							bestMatchTerms[key] = token
						}
						matchedTables[key] = true
						break
					}
				}
			}
		}

		// "date", "time", "created", "updated", "timestamp" → timestamp/date columns
		dateKeywords := map[string]bool{
			"date": true, "time": true, "created": true,
			"updated": true, "timestamp": true, "datetime": true,
		}
		if dateKeywords[tokenLower] {
			for colName, tables := range columnTables {
				colType, ok := columnTypes[colName]
				if !ok {
					continue
				}
				if strings.Contains(colType, "timestamp") || strings.Contains(colType, "date") || strings.Contains(colType, "datetime") || strings.Contains(colType, "time") {
					for _, table := range tables {
						key := table
						if !matchedTables[key] {
							score := 0.40
							if score > bestScores[key] {
								bestScores[key] = score
								bestMatchTypes[key] = "semantic"
								bestMatchTerms[key] = token
							}
							matchedTables[key] = true
						}
					}
				}
			}
		}

		// "name", "title" → varchar columns
		nameKeywords := map[string]bool{"name": true, "title": true, "label": true}
		if nameKeywords[tokenLower] {
			for colName, tables := range columnTables {
				colType, ok := columnTypes[colName]
				if !ok {
					continue
				}
				if strings.Contains(colType, "varchar") || strings.Contains(colType, "char") || strings.Contains(colType, "text") {
					if strings.Contains(colName, "name") || strings.Contains(colName, "title") || strings.Contains(colName, "label") {
						for _, table := range tables {
							key := table
							if !matchedTables[key] {
								score := 0.35
								if score > bestScores[key] {
									bestScores[key] = score
									bestMatchTypes[key] = "semantic"
									bestMatchTerms[key] = token
								}
								matchedTables[key] = true
							}
						}
					}
				}
			}
		}
	}

	// Build final result
	for _, t := range schema.Tables {
		if matchedTables[t.Name] {
			result.Tables = append(result.Tables, TableCandidate{
				Name:        t.Name,
				Score:       bestScores[t.Name],
				MatchType:   bestMatchTypes[t.Name],
				MatchedTerm: bestMatchTerms[t.Name],
			})
		}
	}

	// Compute overall confidence as maximum of table scores
	for _, tc := range result.Tables {
		if tc.Score > result.Confidence {
			result.Confidence = tc.Score
		}
	}

	return result, nil
}

// tokenize splits input on whitespace and punctuation, lowercases each token,
// and filters tokens < 2 chars (unless they match special patterns like "id").
func tokenize(input string) []string {
	var tokens []string
	current := strings.Builder{}
	special := map[string]bool{"id": true, "pk": true}

	for _, r := range input {
		if isWordRune(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tok := current.String()
				if len(tok) >= 2 || special[strings.ToLower(tok)] {
					tokens = append(tokens, tok)
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tok := current.String()
		if len(tok) >= 2 || special[strings.ToLower(tok)] {
			tokens = append(tokens, tok)
		}
	}

	return tokens
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}
