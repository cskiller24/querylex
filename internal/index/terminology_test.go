package index

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
)

func TestGenerateTerminologyTemplate_CreatesValidOutput(t *testing.T) {
	// Test 1: GenerateTerminologyTemplate produces valid Markdown with a fenced
	// ```querylex-terms block containing YAML with one example entry
	dir := t.TempDir()
	tables := []db.TableInfo{
		{
			Schema: "testdb",
			Name:   "users",
			Columns: []db.ColumnInfo{
				{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				{Name: "email", Ordinal: 2, ColumnType: "varchar(255)"},
			},
		},
	}

	err := GenerateTerminologyTemplate(dir, tables)
	if err != nil {
		t.Fatalf("GenerateTerminologyTemplate failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "terminologies.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("terminologies.md not found: %v", err)
	}

	content := string(data)

	// Must contain the fenced block
	if !contains(content, "```querylex-terms") {
		t.Error("terminologies.md missing querylex-terms fenced block")
	}

	// Must contain a closing fence
	if !contains(content, "\n```") && !hasSuffix(content, "```") {
		t.Error("terminologies.md missing closing fence")
	}

	// Must contain YAML fields
	if !contains(content, "term:") {
		t.Error("terminologies.md missing 'term:' field")
	}
	if !contains(content, "type:") {
		t.Error("terminologies.md missing 'type:' field")
	}
	if !contains(content, "maps_to:") {
		t.Error("terminologies.md missing 'maps_to:' field")
	}
}

func TestGenerateTerminologyTemplate_UsesActualTableAndColumn(t *testing.T) {
	// Test 2: GenerateTerminologyTemplate example entry's maps_to uses an actual
	// table name and column name from the input SchemaResult.Tables
	dir := t.TempDir()
	tables := []db.TableInfo{
		{
			Schema: "testdb",
			Name:   "orders",
			Columns: []db.ColumnInfo{
				{Name: "order_id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				{Name: "total_amount", Ordinal: 2, ColumnType: "decimal(10,2)"},
			},
		},
		{
			Schema: "testdb",
			Name:   "customers",
			Columns: []db.ColumnInfo{
				{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
			},
		},
	}

	err := GenerateTerminologyTemplate(dir, tables)
	if err != nil {
		t.Fatalf("GenerateTerminologyTemplate failed: %v", err)
	}

	path := filepath.Join(dir, "terminologies.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("terminologies.md not found: %v", err)
	}

	content := string(data)

	// Must reference an actual table from the input
	if !contains(content, "orders") && !contains(content, "customers") {
		t.Error("example entry should reference an actual table name")
	}

	// Must reference an actual column from the chosen table
	if !contains(content, "order_id") && !contains(content, "total_amount") &&
		!contains(content, "id") {
		t.Error("example entry should reference an actual column name")
	}
}

func TestParseTerminology_ValidYAML(t *testing.T) {
	// Test 3: ParseTerminology extracts fenced block and returns TerminologyDoc
	content := []byte(`# Business Terminology

Freeform markdown before the block.

` + "```querylex-terms" + `
terms:
  - term: "active_users"
    type: metric
    maps_to:
      - table: "users"
        column: "status"
` + "```" + `

Freeform markdown after the block.
`)

	doc, err := ParseTerminology(content)
	if err != nil {
		t.Fatalf("ParseTerminology failed: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}
	if len(doc.Terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(doc.Terms))
	}
	if doc.Terms[0].Term != "active_users" {
		t.Errorf("expected term='active_users', got '%s'", doc.Terms[0].Term)
	}
	if doc.Terms[0].Type != "metric" {
		t.Errorf("expected type='metric', got '%s'", doc.Terms[0].Type)
	}
	if len(doc.Terms[0].MapsTo) != 1 {
		t.Fatalf("expected 1 maps_to entry, got %d", len(doc.Terms[0].MapsTo))
	}
	if doc.Terms[0].MapsTo[0].Table != "users" {
		t.Errorf("expected maps_to.table='users', got '%s'", doc.Terms[0].MapsTo[0].Table)
	}
	if doc.Terms[0].MapsTo[0].Column != "status" {
		t.Errorf("expected maps_to.column='status', got '%s'", doc.Terms[0].MapsTo[0].Column)
	}
}

func TestParseTerminology_MultipleEntries(t *testing.T) {
	// Test 4: Parsing a block with 2+ entries returns all entries
	content := []byte("```querylex-terms\n" +
		`terms:
  - term: "revenue"
    type: metric
    maps_to:
      - table: "transactions"
        column: "amount"
  - term: "customer_region"
    type: dimension
    maps_to:
      - table: "customers"
        column: "region"
` + "```")

	doc, err := ParseTerminology(content)
	if err != nil {
		t.Fatalf("ParseTerminology failed: %v", err)
	}
	if len(doc.Terms) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(doc.Terms))
	}
}

func TestParseTerminology_MissingFencedBlock(t *testing.T) {
	// Test 5: Missing fenced block returns error wrapping ErrTerminologyParse
	content := []byte("# Just markdown\n\nNo fenced block here")

	doc, err := ParseTerminology(content)
	if err == nil {
		t.Fatal("expected error for missing fenced block")
	}
	if doc != nil {
		t.Fatal("expected nil doc for missing fenced block")
	}
	if !errors.Is(err, ErrTerminologyParse) {
		t.Errorf("expected error wrapping ErrTerminologyParse, got: %v", err)
	}
}

func TestParseTerminology_MalformedYAML(t *testing.T) {
	// Test 6: Malformed YAML returns error wrapping ErrTerminologyParse
	content := []byte("# Terms\n\n" +
		"```querylex-terms\n" +
		"bad: [malformed yaml\n" +
		"```")

	doc, err := ParseTerminology(content)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if doc != nil {
		t.Fatal("expected nil doc for malformed YAML")
	}
	if !errors.Is(err, ErrTerminologyParse) {
		t.Errorf("expected error wrapping ErrTerminologyParse, got: %v", err)
	}
}

func TestParseTerminology_EmptyFencedBlock(t *testing.T) {
	// Test 7: Empty fenced block returns error wrapping ErrTerminologyParse
	content := []byte("```querylex-terms\n```")

	doc, err := ParseTerminology(content)
	if err == nil {
		t.Fatal("expected error for empty fenced block")
	}
	if doc != nil {
		t.Fatal("expected nil doc for empty fenced block")
	}
	if !errors.Is(err, ErrTerminologyParse) {
		t.Errorf("expected error wrapping ErrTerminologyParse, got: %v", err)
	}
}

func TestParseTerminology_NonYAMLContent(t *testing.T) {
	// Test 8: Non-YAML content in fenced block returns error wrapping ErrTerminologyParse
	content := []byte("```querylex-terms\n" +
		"This is not YAML at all\n" +
		"Just some random text\n" +
		"```")

	doc, err := ParseTerminology(content)
	if err == nil {
		t.Fatal("expected error for non-YAML content")
	}
	if doc != nil {
		t.Fatal("expected nil doc for non-YAML content")
	}
	if !errors.Is(err, ErrTerminologyParse) {
		t.Errorf("expected error wrapping ErrTerminologyParse, got: %v", err)
	}
}

func TestGenerateTerminologyTemplate_Idempotent(t *testing.T) {
	// Test 9: Two calls with same input produce identical output (deterministic)
	// Note: When file exists, second call is a no-op (does not overwrite)
	dir := t.TempDir()
	tables := []db.TableInfo{
		{
			Schema: "testdb",
			Name:   "products",
			Columns: []db.ColumnInfo{
				{Name: "id", Ordinal: 1, ColumnType: "int", IsPrimaryKey: true},
				{Name: "name", Ordinal: 2, ColumnType: "varchar(200)"},
			},
		},
	}

	// First call
	err1 := GenerateTerminologyTemplate(dir, tables)
	if err1 != nil {
		t.Fatalf("first call failed: %v", err1)
	}

	// Second call — file already exists, should be no-op
	err2 := GenerateTerminologyTemplate(dir, tables)
	if err2 != nil {
		t.Fatalf("second call failed: %v", err2)
	}

	// Verify file exists and content is valid
	path := filepath.Join(dir, "terminologies.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("terminologies.md not found after second call: %v", err)
	}

	content := string(data)
	if !contains(content, "```querylex-terms") {
		t.Error("terminologies.md missing fenced block after idempotent call")
	}
}

func TestParseTerminology_MarkdownOutsideBlockPreserved(t *testing.T) {
	// Test 10: ParseTerminology ignores text before/after the fenced block
	content := []byte(`# My Business Terms

This is introductory text about business terminology.

` + "```querylex-terms" + `
terms:
  - term: "monthly_recurring_revenue"
    type: metric
    maps_to:
      - table: "subscriptions"
        column: "amount"
` + "```" + `

## Additional Notes

This text after the block should be ignored by the parser.

- The parser only cares about the querylex-terms fenced block
- Everything else is freeform Markdown
`)

	doc, err := ParseTerminology(content)
	if err != nil {
		t.Fatalf("ParseTerminology failed: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}
	if len(doc.Terms) != 1 {
		t.Fatalf("expected 1 term, got %d", len(doc.Terms))
	}
	if doc.Terms[0].Term != "monthly_recurring_revenue" {
		t.Errorf("expected term='monthly_recurring_revenue', got '%s'", doc.Terms[0].Term)
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

// containsStr is a simple contains helper.
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// hasSuffix checks if a string ends with a suffix.
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Test that ErrTerminologyParse sentinel is accessible
func TestTerminologyParse_ErrSentinelDefined(t *testing.T) {
	if ErrTerminologyParse == nil {
		t.Fatal("ErrTerminologyParse sentinel should be defined")
	}
	if ErrTerminologyParse.Error() == "" {
		t.Fatal("ErrTerminologyParse should have a non-empty error message")
	}
}
