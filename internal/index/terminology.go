package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cskiller24/querylex/internal/db"
	"gopkg.in/yaml.v3"
)

// ErrTerminologyParse is the sentinel error for parse failures.
// Use errors.Is(err, ErrTerminologyParse) to check for terminology parse errors.
var ErrTerminologyParse = errors.New("TERMINOLOGY_PARSE_ERROR")

// TermEntry represents a single business term mapped to database columns.
type TermEntry struct {
	Term        string         `yaml:"term"`
	Type        string         `yaml:"type"` // metric, entity, entity_filter, dimension, date_window, synonym
	MapsTo      []TermMapping  `yaml:"maps_to"`
	Description string         `yaml:"description,omitempty"`
	Filters     []string       `yaml:"filters,omitempty"`
	Values      []string       `yaml:"values,omitempty"`
}

// TermMapping maps a business term to a specific database column.
type TermMapping struct {
	Table  string `yaml:"table"`
	Column string `yaml:"column"`
}

// TerminologyDoc is the parsed representation of a terminologies.md file.
type TerminologyDoc struct {
	Terms []TermEntry `yaml:"terms"`
}

// validTermTypes is the set of allowed term type values.
var validTermTypes = map[string]bool{
	"metric":         true,
	"entity":         true,
	"entity_filter":  true,
	"dimension":      true,
	"date_window":    true,
	"synonym":        true,
}

// GenerateTerminologyTemplate creates a skeleton terminologies.md file with
// one example term entry in a fenced querylex-terms YAML block.
// If the file already exists, it is a no-op (preserves user edits).
func GenerateTerminologyTemplate(dbDir string, tables []db.TableInfo) error {
	path := filepath.Join(dbDir, "terminologies.md")

	// Check if file already exists — preserve user edits
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// Pick example table: first table with at least one column
	var exampleTable *db.TableInfo
	var exampleColumn *db.ColumnInfo
	for i := range tables {
		if len(tables[i].Columns) > 0 {
			exampleTable = &tables[i]
			exampleColumn = &tables[i].Columns[0]
			break
		}
	}

	tableName := "example_table"
	columnName := "example_column"
	schemaName := "your_database"
	if exampleTable != nil {
		tableName = exampleTable.Name
		schemaName = exampleTable.Schema
		if exampleColumn != nil {
			columnName = exampleColumn.Name
		}
	}

	// Build schema placeholder from first table's schema
	dbPlaceholder := schemaName
	if dbPlaceholder == "" {
		dbPlaceholder = "Your Database"
	}

	content := fmt.Sprintf(`# Business Terminology — %s

Edit this file to map business terms to database columns.
The `+"`querylex-terms`"+` fenced block contains YAML entries.
Freeform Markdown outside the block is preserved and ignored by the parser.

`+"```querylex-terms"+`
terms:
  - term: "example_term"
    type: entity
    maps_to:
      - table: "%s"
        column: "%s"
`+"```"+`
`, dbPlaceholder, tableName, columnName)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write terminologies.md: %w", err)
	}

	return nil
}

// ParseTerminology extracts the querylex-terms fenced YAML block from
// terminologies.md content and returns structured TermEntry objects.
// Returns error wrapping ErrTerminologyParse on any parse failure.
func ParseTerminology(content []byte) (*TerminologyDoc, error) {
	block, err := extractFencedBlock(content)
	if err != nil {
		return nil, err
	}

	// Check for empty block
	trimmed := strings.TrimSpace(string(block))
	if trimmed == "" {
		return nil, fmt.Errorf("%w: empty querylex-terms block", ErrTerminologyParse)
	}

	// Unmarshal YAML
	var doc TerminologyDoc
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTerminologyParse, err)
	}

	// Validate terms exist
	if len(doc.Terms) == 0 {
		return nil, fmt.Errorf("%w: no terms defined in querylex-terms block", ErrTerminologyParse)
	}

	// Validate each term
	for _, entry := range doc.Terms {
		if !validTermTypes[entry.Type] {
			return nil, fmt.Errorf("%w: invalid term type '%s' for term '%s'",
				ErrTerminologyParse, entry.Type, entry.Term)
		}
		if len(entry.MapsTo) == 0 {
			return nil, fmt.Errorf("%w: term '%s' has no maps_to entries",
				ErrTerminologyParse, entry.Term)
		}
		for _, m := range entry.MapsTo {
			if m.Table == "" {
				return nil, fmt.Errorf("%w: term '%s' has empty table in maps_to",
					ErrTerminologyParse, entry.Term)
			}
			if m.Column == "" {
				return nil, fmt.Errorf("%w: term '%s' has empty column in maps_to",
					ErrTerminologyParse, entry.Term)
			}
		}
	}

	return &doc, nil
}

// extractFencedBlock finds the querylex-terms fenced code block and extracts
// the YAML content. The opening fence must be exactly "```querylex-terms"
// (allowing trailing whitespace). The closing fence is the next line starting
// with "```" that is not the opening fence.
func extractFencedBlock(content []byte) ([]byte, error) {
	text := string(content)
	lines := strings.Split(text, "\n")

	// Find opening fence
	startIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == "```querylex-terms" {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return nil, fmt.Errorf("%w: missing querylex-terms fenced block", ErrTerminologyParse)
	}

	// Find closing fence (next line starting with "```")
	endIdx := -1
	for i := startIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], " \t\r")
		if strings.HasPrefix(trimmed, "```") {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, fmt.Errorf("%w: unclosed querylex-terms fenced block", ErrTerminologyParse)
	}

	// Extract YAML content between fences (exclusive)
	blockLines := lines[startIdx+1 : endIdx]
	block := strings.Join(blockLines, "\n")

	return []byte(block), nil
}
