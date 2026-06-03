package testhelper

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FixtureRunner loads SQL fixture files from test/testdata/fixtures/ and
// executes them sequentially against the given *sql.DB. Files are split on
// semicolons and each non-empty statement is executed. Fail-fast: stops on
// first error with the file name and failing SQL statement in the error message.
func FixtureRunner(t *testing.T, db *sql.DB, files ...string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join("test", "testdata", "fixtures", file)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("fixture %s: %v", file, err)
		}

		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				t.Fatalf("fixture %s: exec error: %v\nSQL: %s", file, err, fmt.Sprintf("%.1000s", stmt))
			}
		}
	}
}
