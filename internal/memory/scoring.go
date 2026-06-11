package memory

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/cskiller24/querylex/internal/index"
)

// fromRe matches table names after FROM keyword.
var fromRe = regexp.MustCompile(`(?i)\bFROM\s+(\w+)`)

// joinRe matches table names after JOIN keyword.
var joinRe = regexp.MustCompile(`(?i)\bJOIN\s+(\w+)`)

// ComputeSimilarity computes a 5-component lexical-only weighted similarity score
// between the input and a stored memory entry. Returns a value in [0, 1].
//
// Components:
//   - Schema-entity overlap (0.30)
//   - SQL structure overlap (0.15)
//   - Intent classification (0.25)
//   - Filter/temporal overlap (0.20)
//   - Recency decay (0.10)
func ComputeSimilarity(input string, entry MemoryEntry, schemaTokens map[string]struct{}, now time.Time) float64 {
	inputTokens := tokenize(input)
	if len(inputTokens) == 0 {
		return 0
	}

	entityScore := computeEntityOverlap(inputTokens, schemaTokens)
	sqlStructureScore := computeSQLStructureScore(input, entry.SQL, schemaTokens)
	intentScore := computeIntentScore(entry.Input, entry.SQL)
	filterScore := computeFilterOverlap(input, entry.SQL)
	recencyScore := computeRecencyScore(entry, now)

	return 0.30*entityScore + 0.15*sqlStructureScore + 0.25*intentScore + 0.20*filterScore + 0.10*recencyScore
}

// ExtractSchemaTokens reads the schema_map.json for the given dbDir and
// returns a set of all table and column names (case-insensitive).
// Returns an empty map (not error) if schema_map.json is missing.
func ExtractSchemaTokens(dbDir string) (map[string]struct{}, error) {
	sm, err := index.ReadSchemaMap(dbDir)
	if err != nil {
		return nil, err
	}
	if sm == nil {
		return make(map[string]struct{}), nil
	}

	tokens := make(map[string]struct{})
	for _, entry := range sm {
		tokens[strings.ToLower(entry.Table)] = struct{}{}
		for _, col := range entry.PKColumns {
			tokens[strings.ToLower(col)] = struct{}{}
		}
		for _, col := range entry.IndexedColumns {
			tokens[strings.ToLower(col)] = struct{}{}
		}
		for _, fk := range entry.FKIn {
			tokens[strings.ToLower(fk.Table)] = struct{}{}
			tokens[strings.ToLower(fk.Column)] = struct{}{}
		}
		for _, fk := range entry.FKOut {
			tokens[strings.ToLower(fk.Table)] = struct{}{}
			tokens[strings.ToLower(fk.Column)] = struct{}{}
		}
		for _, composite := range entry.CompositeIndexes {
			for _, col := range composite {
				tokens[strings.ToLower(col)] = struct{}{}
			}
		}
	}

	return tokens, nil
}

// computeEntityOverlap counts how many input tokens match schema tokens.
// Score = matches / max(len(inputTokens), 1), clamped to [0, 1].
func computeEntityOverlap(inputTokens []string, schemaTokens map[string]struct{}) float64 {
	if len(schemaTokens) == 0 {
		return 0
	}

	matches := 0
	for _, token := range inputTokens {
		if _, ok := schemaTokens[token]; ok {
			matches++
		}
	}

	score := float64(matches) / float64(max(len(inputTokens), 1))
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// computeSQLStructureScore scores how well the input's table/column mentions
// match the tables and columns used in the saved entry's SQL.
// It extracts table names (FROM/JOIN) from the SQL and checks them against
// schema tokens present in both the input and the SQL. Score is the Jaccard
// similarity between input-schema matches and SQL-schema matches.
func computeSQLStructureScore(input, entrySQL string, schemaTokens map[string]struct{}) float64 {
	if len(schemaTokens) == 0 || input == "" || entrySQL == "" {
		return 0
	}

	// Extract table names from the saved SQL's FROM and JOIN clauses
	sqlTables := make(map[string]struct{})
	for _, m := range fromRe.FindAllStringSubmatch(entrySQL, -1) {
		if len(m) >= 2 {
			sqlTables[strings.ToLower(m[1])] = struct{}{}
		}
	}
	for _, m := range joinRe.FindAllStringSubmatch(entrySQL, -1) {
		if len(m) >= 2 {
			sqlTables[strings.ToLower(m[1])] = struct{}{}
		}
	}

	if len(sqlTables) == 0 {
		return 0
	}

	// Find which SQL tables are also in the schema tokens
	sqlSchemaTables := make(map[string]struct{})
	for t := range sqlTables {
		if _, ok := schemaTokens[t]; ok {
			sqlSchemaTables[t] = struct{}{}
		}
	}

	if len(sqlSchemaTables) == 0 {
		return 0
	}

	// Find which of those tables appear in the input
	inputTokens := tokenizeSet(input)
	inputMatches := 0
	for t := range sqlSchemaTables {
		if _, ok := inputTokens[t]; ok {
			inputMatches++
		}
	}

	// Score = matches / total SQL-schema tables (Jaccard-like)
	score := float64(inputMatches) / float64(len(sqlSchemaTables))
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// aggregationRe matches SQL aggregation keywords.
var aggregationRe = regexp.MustCompile(`(?i)\b(SUM|COUNT|AVG|MIN|MAX|GROUP\s+BY)\b`)

// trendsSqlRe matches SQL ORDER BY keyword.
var trendsSqlRe = regexp.MustCompile(`(?i)\bORDER\s+BY\b`)

// trendsInputRe matches trend-related terms in input.
var trendsInputRe = regexp.MustCompile(`(?i)\b(trend|over\s+time|by\s+(day|week|month|year))\b`)

// computeIntentScore classifies the intent of a saved entry:
//   - "aggregation": SQL contains aggregation keywords → 1.0
//   - "trends": SQL has ORDER BY or input has trend terms → 0.8
//   - "lookup": SELECT without aggregation → 0.5
//   - Default: 0.0
func computeIntentScore(entryInput, entrySQL string) float64 {
	if aggregationRe.MatchString(entrySQL) {
		return 1.0
	}
	if trendsSqlRe.MatchString(entrySQL) || trendsInputRe.MatchString(entryInput) {
		return 0.8
	}
	if strings.Contains(strings.ToUpper(entrySQL), "SELECT") {
		return 0.5
	}
	return 0.0
}

// whereClauseRe extracts the WHERE clause from a SQL query.
var whereClauseRe = regexp.MustCompile(`(?i)WHERE\s+(.+?)(?:\s+GROUP\s+BY|\s+ORDER\s+BY|\s+LIMIT|\s*$)`)

// computeFilterOverlap computes Jaccard similarity between tokens in the
// WHERE clause of the saved SQL and tokens extracted from the input.
func computeFilterOverlap(input, entrySQL string) float64 {
	// Extract WHERE clause from the saved SQL
	match := whereClauseRe.FindStringSubmatch(entrySQL)
	if len(match) < 2 {
		return 0.0
	}
	whereClause := match[1]

	// Tokenize both
	whereTokens := tokenizeSet(whereClause)
	inputTokens := tokenizeSet(input)

	if len(whereTokens) == 0 || len(inputTokens) == 0 {
		return 0.0
	}

	// Jaccard = |intersection| / |union|
	intersection := 0
	for token := range inputTokens {
		if whereTokens[token] {
			intersection++
		}
	}

	union := len(whereTokens) + len(inputTokens) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// computeRecencyScore computes an exponential decay score based on days since
// last use. Half-life is 30 days: score = exp(-days / 43.3).
func computeRecencyScore(entry MemoryEntry, now time.Time) float64 {
	lastUsed := entry.LastUsedAt
	if lastUsed == "" {
		lastUsed = entry.UpdatedAt
	}
	if lastUsed == "" {
		return 0.0
	}

	lastTime, err := time.Parse(time.RFC3339, lastUsed)
	if err != nil {
		return 0.0
	}

	daysSinceLastUse := now.Sub(lastTime).Hours() / 24.0
	if daysSinceLastUse < 0 {
		daysSinceLastUse = 0
	}

	// 43.3 = 30 / ln(2) — 30-day half-life
	score := math.Exp(-daysSinceLastUse / 43.3)
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// tokenizeSet tokenizes a string and returns a set of unique tokens.
func tokenizeSet(s string) map[string]bool {
	tokens := tokenize(s)
	set := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		set[t] = true
	}
	return set
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
