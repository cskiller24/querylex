//go:build e2e

// Package crossengine provides E2E tests that validate SQL behavior
// consistently across all 5 supported database engines. It runs the
// same SQL patterns against MySQL, PostgreSQL, MariaDB, MSSQL, and
// SQLite using per-engine dialect variations.
//
// IMPORTANT: This test requires all 5 database Docker containers to be
// running simultaneously. Run with:
//   make test-e2e-cross-engine
//
// Dependencies:
//   - ./bin/querylex (pre-built via "make build-test")
//   - Docker containers for mysql, postgresql, mariadb, mssql
//   - SQLite runs in-process (no Docker needed)
package crossengine

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "modernc.org/sqlite"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/index"
	"github.com/cskiller24/querylex/internal/state"
	"github.com/cskiller24/querylex/test/testhelper"
)

// engineConn holds a database connection and its extracted connection info
// for one engine's per-test database.
type engineConn struct {
	db     *sql.DB
	engine string
	host   string
	port   int
	dbName string
	user   string
}

// TestCrossEngineSQL validates SQL behavior consistency across all 5
// database engines. It runs 12 table-driven sub-tests covering SELECT
// with joins, aggregates, subqueries, CTEs, ORDER BY, LIMIT variations,
// type coercion, and error cases.
//
// Each sub-test provides per-engine SQL in the engine's native dialect:
//   - MySQL/MariaDB: backtick quoting (infrequent), CONCAT(), LIMIT
//   - PostgreSQL: double-quote identifiers, || concatenation, LIMIT
//   - MSSQL: bracket quoting, + concatenation, SELECT TOP n,
//     OFFSET/FETCH for pagination
//   - SQLite: double-quote identifiers, || concatenation, LIMIT
//
// Dataset references per engine:
//   - MySQL:     Employees DB (employees, departments, dept_emp)
//   - PostgreSQL: Pagila (actor, film, film_category)
//   - MariaDB:   Employees DB (employees, departments, dept_emp)
//   - MSSQL:     Northwind (Customers, Orders, Employees)
//   - SQLite:    Chinook (Album, Track, Genre, Artist)
func TestCrossEngineSQL(t *testing.T) {
	// Set up per-engine connections and load dataset schemas
	engines := setupEngineConnections(t)

	tests := []struct {
		name string
		sql  map[string]string // per-engine dialect SQL
		want map[string]string // per-engine expected error code ("" = success)
	}{
		// ── 1. SELECT simple ──
		{
			name: "select_simple",
			sql: map[string]string{
				"mysql":      "SELECT emp_no, first_name, last_name FROM employees LIMIT 3",
				"postgresql": `SELECT actor_id, first_name, last_name FROM actor LIMIT 3`,
				"mariadb":    "SELECT emp_no, first_name, last_name FROM employees LIMIT 3",
				"mssql":      "SELECT TOP 3 CustomerID, CompanyName, ContactName FROM Customers",
				"sqlite":     `SELECT AlbumId, Title FROM Album LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 2. SELECT with JOIN ──
		{
			name: "select_with_join",
			sql: map[string]string{
				"mysql":      "SELECT e.first_name, d.dept_name FROM employees e JOIN dept_emp de ON e.emp_no = de.emp_no JOIN departments d ON de.dept_no = d.dept_no LIMIT 3",
				"postgresql": `SELECT a.first_name, a.last_name FROM actor a JOIN film_actor fa ON a.actor_id = fa.actor_id JOIN film f ON fa.film_id = f.film_id LIMIT 3`,
				"mariadb":    "SELECT e.first_name, d.dept_name FROM employees e JOIN dept_emp de ON e.emp_no = de.emp_no JOIN departments d ON de.dept_no = d.dept_no LIMIT 3",
				"mssql":      "SELECT TOP 3 c.CompanyName, o.OrderDate FROM Customers c JOIN Orders o ON c.CustomerID = o.CustomerID",
				"sqlite":     `SELECT a.Title, ar.Name FROM Album a JOIN Artist ar ON a.ArtistId = ar.ArtistId LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 3. SELECT with aggregate (GROUP BY + COUNT) ──
		{
			name: "select_with_aggregate",
			sql: map[string]string{
				"mysql":      "SELECT dept_no, COUNT(*) as cnt FROM dept_emp GROUP BY dept_no",
				"postgresql": `SELECT fc.category_id, COUNT(*) as cnt FROM film_category fc GROUP BY fc.category_id`,
				"mariadb":    "SELECT dept_no, COUNT(*) as cnt FROM dept_emp GROUP BY dept_no",
				"mssql":      "SELECT CustomerID, COUNT(*) as cnt FROM Orders GROUP BY CustomerID",
				"sqlite":     `SELECT GenreId, COUNT(*) as cnt FROM Track GROUP BY GenreId`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 4. SELECT with subquery ──
		{
			name: "select_with_subquery",
			sql: map[string]string{
				"mysql":      "SELECT emp_no, first_name FROM employees WHERE emp_no IN (SELECT emp_no FROM dept_emp WHERE dept_no = 'd005') LIMIT 3",
				"postgresql": `SELECT actor_id, first_name FROM actor WHERE actor_id IN (SELECT actor_id FROM film_actor WHERE film_id = 1) LIMIT 3`,
				"mariadb":    "SELECT emp_no, first_name FROM employees WHERE emp_no IN (SELECT emp_no FROM dept_emp WHERE dept_no = 'd005') LIMIT 3",
				"mssql":      "SELECT TOP 3 CompanyName FROM Customers WHERE CustomerID IN (SELECT CustomerID FROM Orders WHERE OrderDate > '1998-01-01')",
				"sqlite":     `SELECT Title FROM Album WHERE AlbumId IN (SELECT AlbumId FROM Track WHERE UnitPrice > 0.99) LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 5. SELECT with CTE ──
		{
			name: "select_with_cte",
			sql: map[string]string{
				"mysql":      "WITH dept_summary AS (SELECT dept_no, COUNT(*) as emp_count FROM dept_emp GROUP BY dept_no) SELECT dept_no, emp_count FROM dept_summary LIMIT 3",
				"postgresql": `WITH actor_count AS (SELECT fa.film_id, COUNT(*) as num_actors FROM film_actor fa GROUP BY fa.film_id) SELECT ac.film_id, ac.num_actors FROM actor_count ac LIMIT 3`,
				"mariadb":    "WITH dept_summary AS (SELECT dept_no, COUNT(*) as emp_count FROM dept_emp GROUP BY dept_no) SELECT dept_no, emp_count FROM dept_summary LIMIT 3",
				"mssql":      "WITH order_summary AS (SELECT CustomerID, COUNT(*) as order_count FROM Orders GROUP BY CustomerID) SELECT TOP 3 CustomerID, order_count FROM order_summary ORDER BY order_count DESC",
				"sqlite":     `WITH track_count AS (SELECT AlbumId, COUNT(*) as num_tracks FROM Track GROUP BY AlbumId) SELECT AlbumId, num_tracks FROM track_count LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 6. SELECT with ORDER BY ──
		{
			name: "select_order_by",
			sql: map[string]string{
				"mysql":      "SELECT emp_no, first_name, last_name FROM employees ORDER BY last_name LIMIT 3",
				"postgresql": `SELECT actor_id, first_name, last_name FROM actor ORDER BY last_name LIMIT 3`,
				"mariadb":    "SELECT emp_no, first_name, last_name FROM employees ORDER BY last_name LIMIT 3",
				"mssql":      "SELECT TOP 3 CustomerID, CompanyName FROM Customers ORDER BY CompanyName",
				"sqlite":     `SELECT AlbumId, Title FROM Album ORDER BY Title LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 7. SELECT with LIMIT/OFFSET variation ──
		{
			name: "select_limit_variation",
			sql: map[string]string{
				"mysql":      "SELECT emp_no, first_name FROM employees ORDER BY emp_no LIMIT 5 OFFSET 2",
				"postgresql": `SELECT actor_id, first_name FROM actor ORDER BY actor_id LIMIT 5 OFFSET 2`,
				"mariadb":    "SELECT emp_no, first_name FROM employees ORDER BY emp_no LIMIT 5 OFFSET 2",
				"mssql":      "SELECT CustomerID, CompanyName FROM Customers ORDER BY CustomerID OFFSET 0 ROWS FETCH NEXT 3 ROWS ONLY",
				"sqlite":     `SELECT AlbumId, Title FROM Album ORDER BY AlbumId LIMIT 5 OFFSET 2`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 8. Type coercion: string concatenation ──
		{
			name: "type_coercion_string",
			sql: map[string]string{
				// MySQL uses CONCAT() function
				"mysql":      "SELECT CONCAT(first_name, ' ', last_name) AS full_name FROM employees LIMIT 3",
				// PostgreSQL uses || (standard SQL)
				"postgresql": `SELECT first_name || ' ' || last_name AS full_name FROM actor LIMIT 3`,
				// MariaDB uses CONCAT() function
				"mariadb":    "SELECT CONCAT(first_name, ' ', last_name) AS full_name FROM employees LIMIT 3",
				// MSSQL uses + (T-SQL style)
				"mssql":      "SELECT TOP 3 CompanyName + ' (' + ContactName + ')' AS company_info FROM Customers",
				// SQLite uses || (standard SQL)
				"sqlite":     `SELECT Title || ' (' || Name || ')' AS track_info FROM Track t JOIN Artist a ON t.AlbumId = a.ArtistId LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 9. Type coercion: numeric expression ──
		{
			name: "type_coercion_numeric",
			sql: map[string]string{
				"mysql":      "SELECT emp_no, salary * 1.1 AS adjusted FROM salaries LIMIT 3",
				"postgresql": `SELECT actor_id, 100 + actor_id AS numeric_expr FROM actor LIMIT 3`,
				"mariadb":    "SELECT emp_no, salary * 1.1 AS adjusted FROM salaries LIMIT 3",
				"mssql":      "SELECT TOP 3 ProductID, UnitPrice * 1.1 AS adjusted_price FROM Products",
				"sqlite":     `SELECT AlbumId, AlbumId * 2 AS doubled FROM Album LIMIT 3`,
			},
			want: map[string]string{
				"mysql": "", "postgresql": "", "mariadb": "", "mssql": "", "sqlite": "",
			},
		},

		// ── 10. Invalid syntax ──
		{
			name: "invalid_syntax",
			sql: map[string]string{
				"mysql":      "SYNTAX ERROR GARBAGE XYZ",
				"postgresql": "SYNTAX ERROR GARBAGE XYZ",
				"mariadb":    "SYNTAX ERROR GARBAGE XYZ",
				"mssql":      "SYNTAX ERROR GARBAGE XYZ",
				"sqlite":     "SYNTAX ERROR GARBAGE XYZ",
			},
			want: map[string]string{
				"mysql": "INVALID_SQL", "postgresql": "INVALID_SQL", "mariadb": "INVALID_SQL",
				"mssql": "INVALID_SQL", "sqlite": "INVALID_SQL",
			},
		},

		// ── 11. Table not found ──
		{
			name: "table_not_found",
			sql: map[string]string{
				"mysql":      "SELECT * FROM nonexistent_table_xyz123",
				"postgresql": "SELECT * FROM nonexistent_table_xyz123",
				"mariadb":    "SELECT * FROM nonexistent_table_xyz123",
				"mssql":      "SELECT * FROM nonexistent_table_xyz123",
				"sqlite":     "SELECT * FROM nonexistent_table_xyz123",
			},
			want: map[string]string{
				"mysql": "TABLE_NOT_FOUND", "postgresql": "TABLE_NOT_FOUND", "mariadb": "TABLE_NOT_FOUND",
				"mssql": "TABLE_NOT_FOUND", "sqlite": "TABLE_NOT_FOUND",
			},
		},

		// ── 12. Column not found ──
		{
			name: "column_not_found",
			sql: map[string]string{
				"mysql":      "SELECT nonexistent_col_xyz123 FROM employees",
				"postgresql": `SELECT nonexistent_col_xyz123 FROM actor`,
				"mariadb":    "SELECT nonexistent_col_xyz123 FROM employees",
				"mssql":      "SELECT nonexistent_col_xyz123 FROM Customers",
				"sqlite":     `SELECT nonexistent_col_xyz123 FROM Album`,
			},
			want: map[string]string{
				"mysql": "COLUMN_NOT_FOUND", "postgresql": "COLUMN_NOT_FOUND", "mariadb": "COLUMN_NOT_FOUND",
				"mssql": "COLUMN_NOT_FOUND", "sqlite": "COLUMN_NOT_FOUND",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, engine := range []string{"mysql", "postgresql", "mariadb", "mssql", "sqlite"} {
				t.Run(engine, func(t *testing.T) {
					sqlStr, hasSQL := tt.sql[engine]
					if !hasSQL {
						t.Skip("no SQL defined for this engine")
					}
					wantCode, hasWant := tt.want[engine]
					if !hasWant {
						t.Skip("no expected result defined for this engine")
					}

					conn := engines[engine]

					// Create per-subtest workspace in temp dir
					setupEngineWorkspace(t, engine, conn)

					// Run querylex validate
					stdout, stderr, exitCode := testhelper.RunQuerylex(t, "validate", sqlStr)

					if wantCode == "" {
						// Expect success: exit 0 + success=true
						if exitCode != 0 {
							t.Errorf("expected exit 0 for %s/%s, got %d\nstdout: %s\nstderr: %s",
								tt.name, engine, exitCode, stdout, stderr)
							return
						}
						var resp map[string]any
						if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
							t.Errorf("invalid JSON for %s/%s: %v\nstdout: %s",
								tt.name, engine, err, stdout)
							return
						}
						if success, ok := resp["success"].(bool); !ok || !success {
							t.Errorf("expected success=true for %s/%s, got success=%v\nstdout: %s",
								tt.name, engine, resp["success"], stdout)
						}
					} else {
						// Expect error: exit 1 + error.code == wantCode
						if exitCode != 1 {
							t.Errorf("expected exit 1 for %s/%s, got %d\nstdout: %s\nstderr: %s",
								tt.name, engine, exitCode, stdout, stderr)
							return
						}
						var resp map[string]any
						if err := json.Unmarshal([]byte(stdout), &resp); err == nil {
							if errObj, ok := resp["error"].(map[string]any); ok {
								if code, ok := errObj["code"].(string); ok {
									if code != wantCode {
										t.Errorf("expected error.code=%q for %s/%s, got %q\nstdout: %s",
											wantCode, tt.name, engine, code, stdout)
									}
								} else {
									t.Errorf("error.code missing or not a string: %v", errObj)
								}
							} else {
								t.Errorf("response missing 'error' object: %s", stdout)
							}
						} else {
							// JSON parse failed — check stderr for error code as fallback
							if !strings.Contains(stderr, wantCode) {
								t.Errorf("expected error containing %q in stderr for %s/%s, got: %s",
									wantCode, tt.name, engine, stderr)
							}
						}
					}
				})
			}
		})
	}
}

// ── Engine connection setup ──

// setupEngineConnections connects to all 5 engines, loads engine-specific
// dataset schemas, and returns a map of engine name -> connection info.
func setupEngineConnections(t *testing.T) map[string]*engineConn {
	t.Helper()

	engines := make(map[string]*engineConn, 5)

	// MySQL — Employees DB
	t.Log("Connecting to MySQL...")
	mysqlDB := testhelper.ConnectMySQL(t)
	loadEmployeesSchema(t, mysqlDB)
	engines["mysql"] = extractMySQLInfo(t, mysqlDB)

	// PostgreSQL — Pagila
	t.Log("Connecting to PostgreSQL...")
	pgDB := testhelper.ConnectPostgreSQL(t)
	loadPagilaSchema(t, pgDB)
	engines["postgresql"] = extractPGInfo(t, pgDB)

	// MariaDB — Employees DB
	t.Log("Connecting to MariaDB...")
	mariaDB := testhelper.ConnectMariaDB(t)
	loadEmployeesSchema(t, mariaDB)
	engines["mariadb"] = extractMySQLInfo(t, mariaDB)

	// MSSQL — Northwind
	t.Log("Connecting to MSSQL...")
	mssqlDB := testhelper.ConnectMSSQL(t)
	loadNorthwindSchema(t, mssqlDB)
	engines["mssql"] = extractMSSQLInfo(t, mssqlDB)

	// SQLite — Chinook
	t.Log("Connecting to SQLite...")
	sqliteDB := testhelper.ConnectSQLite(t)
	loadChinookSchema(t, sqliteDB)
	engines["sqlite"] = extractSQLiteInfo(t, sqliteDB)

	return engines
}

// ── Workspace setup ──

// setupEngineWorkspace creates a minimal querylex workspace in t.TempDir()
// for the given engine, with bare env var credentials (no encrypted store).
// Sets HOME env var so the querylex subprocess finds the workspace.
func setupEngineWorkspace(t *testing.T, engine string, conn *engineConn) {
	t.Helper()

	dbID := "e2e-test-db"
	home := t.TempDir()
	wsDir := filepath.Join(home, ".querylex")

	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("mkdir .querylex: %v", err)
	}

	// Write querylex.json with one DatabaseEntry
	activeDBID := dbID
	ws := &state.Workspace{
		ConnectedDatabases: []state.DatabaseEntry{
			{ID: dbID, Name: conn.dbName, Type: conn.engine, Status: state.StatusIndexed, IndexingProgress: 100},
		},
		ActiveDatabaseID: &activeDBID,
	}
	wsData, err := json.Marshal(ws)
	if err != nil {
		t.Fatalf("marshal workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "querylex.json"), wsData, 0644); err != nil {
		t.Fatalf("write querylex.json: %v", err)
	}

	// Pre-populate the encrypted credential store
	encPath := filepath.Join(wsDir, "credentials.json.enc")
	encStore := credentials.NewEncryptedFileStore(encPath)
	if err := encStore.Unlock("e2e-test-passphrase"); err != nil {
		t.Fatalf("encrypted store unlock: %v", err)
	}
	credRef, err := encStore.Store(dbID, "testpass")
	if err != nil {
		t.Fatalf("encrypted store store: %v", err)
	}

	// Write database.json
	dbDir := filepath.Join(wsDir, dbID)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	dbCfg := map[string]interface{}{
		"id":                  dbID,
		"name":                conn.dbName,
		"type":                conn.engine,
		"host":                conn.host,
		"port":                conn.port,
		"database":            conn.dbName,
		"username":            conn.user,
		"ssl_mode":            "disable",
		"credential_reference": credRef,
	}
	dbData, err := json.Marshal(dbCfg)
	if err != nil {
		t.Fatalf("marshal database.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "database.json"), dbData, 0644); err != nil {
		t.Fatalf("write database.json: %v", err)
	}

	// Create minimal indexing artifacts for preflight gating
	schemaDir := filepath.Join(dbDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	schemaData := map[string]interface{}{
		"tables": []interface{}{},
	}
	schemaJSON, err := json.Marshal(schemaData)
	if err != nil {
		t.Fatalf("marshal schema.json: %v", err)
	}
	schemaPath := filepath.Join(schemaDir, "schema.json")
	if err := os.WriteFile(schemaPath, schemaJSON, 0644); err != nil {
		t.Fatalf("write schema.json: %v", err)
	}

	// Compute checksum and write index manifest
	schemaChecksum, err := index.ComputeChecksum(schemaPath)
	if err != nil {
		t.Fatalf("compute schema checksum: %v", err)
	}
	manifest := &index.IndexManifest{
		SchemaVersionHash: "e2e-test-hash",
		DBVersion:         conn.engine,
		TableCount:        0,
		ArtifactChecksums: map[string]string{
			"schema/schema.json": schemaChecksum,
		},
	}
	if err := index.WriteIndexManifest(dbDir, manifest); err != nil {
		t.Fatalf("write index manifest: %v", err)
	}

	// Set environment variables for the subprocess
	t.Setenv("HOME", home)
	t.Setenv("QUERYLEX_DB_PASSWORD", "testpass")
	t.Setenv("QUERYLEX_KEYCHAIN_PASSPHRASE", "e2e-test-passphrase")
}

// ── Dataset loading ──

// loadSQLFromFile reads a SQL file, splits into statements by semicolons,
// skips filtered lines (comments, source commands, etc.), and executes each
// statement against the database.
func loadSQLFromFile(t *testing.T, db *sql.DB, filePath string, skipPrefixes ...string) {
	t.Helper()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read SQL file %s: %v", filePath, err)
	}

	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Skip comment-only lines
		if strings.HasPrefix(stmt, "--") || strings.HasPrefix(stmt, "#") || strings.HasPrefix(stmt, "//") {
			continue
		}

		// Skip version-specific comments (MySQL /*!...*/)
		if strings.HasPrefix(stmt, "/*!") {
			continue
		}

		// Skip explicitly filtered prefixes
		upper := strings.ToUpper(stmt)
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(upper, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Execute the statement
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("execute DDL from %s: %v\nSQL: %.1000s", filePath, err, stmt)
		}
	}
}

// loadEmployeesSchema reads the Employees DB SQL from the cached download
// and creates the schema (6 tables + 2 views) in the given database.
func loadEmployeesSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	path := filepath.Join("test", "testdata", "cache", "test_db-extracted", "test_db-master", "employees.sql")
	loadSQLFromFile(t, db, path,
		"DROP DATABASE", "CREATE DATABASE", "USE ",
		"FLUSH", "SOURCE",
		"SELECT INFORMATION_SCHEMA", "SELECT ENGINE",
	)
}

// loadPagilaSchema reads the Pagila SQL from the cached download
// and creates the schema in the given PostgreSQL database.
func loadPagilaSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	path := filepath.Join("test", "testdata", "cache", "pagila-extracted", "pagila-schema.sql")
	loadSQLFromFile(t, db, path, "DROP DATABASE", "CREATE DATABASE")

	// Also load Pagila data
	dataPath := filepath.Join("test", "testdata", "cache", "pagila-extracted", "pagila-data.sql")
	loadSQLFromFile(t, db, dataPath, "DROP DATABASE", "CREATE DATABASE")
}

// loadNorthwindSchema reads the Northwind SQL from the cached download
// and creates the schema in the given MSSQL database.
func loadNorthwindSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	path := filepath.Join("test", "testdata", "cache", "northwind-extracted", "northwind.sql")
	loadSQLFromFile(t, db, path, "DROP DATABASE", "CREATE DATABASE", "USE ")
}

// loadChinookSchema reads the Chinook SQL from the cached download
// and creates the schema in the given SQLite database.
func loadChinookSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	path := filepath.Join("test", "testdata", "cache", "chinook-extracted", "chinook.sql")
	loadSQLFromFile(t, db, path, "DROP DATABASE")
}

// ── Connection info extraction ──

// extractMySQLInfo extracts host, port, and database name from a MySQL or
// MariaDB connection by querying server variables.
func extractMySQLInfo(t *testing.T, db *sql.DB) *engineConn {
	t.Helper()
	var host string
	var port int
	var dbName string
	err := db.QueryRow("SELECT @@hostname, @@port, DATABASE()").Scan(&host, &port, &dbName)
	if err != nil {
		t.Fatalf("extract MySQL connection info: %v", err)
	}
	// Determine the user from table-level grants just use "root" as default
	return &engineConn{db: db, engine: "mysql", host: host, port: port, dbName: dbName, user: "root"}
}

// extractPGInfo extracts connection info from a PostgreSQL connection.
func extractPGInfo(t *testing.T, db *sql.DB) *engineConn {
	t.Helper()
	var host string
	var port int
	var dbName string
	// inet_server_addr() returns nil if connected via Unix socket; fall back to localhost
	err := db.QueryRow("SELECT COALESCE(inet_server_addr()::text, 'localhost'), inet_server_port(), current_database()").Scan(&host, &port, &dbName)
	if err != nil {
		t.Fatalf("extract PostgreSQL connection info: %v", err)
	}
	return &engineConn{db: db, engine: "postgresql", host: host, port: port, dbName: dbName, user: "postgres"}
}

// extractMSSQLInfo extracts connection info from a MSSQL connection.
func extractMSSQLInfo(t *testing.T, db *sql.DB) *engineConn {
	t.Helper()
	var host string
	var dbName string
	err := db.QueryRow("SELECT @@SERVERNAME, DB_NAME()").Scan(&host, &dbName)
	if err != nil {
		// Fallback for connection failure
		host = "localhost"
		err = db.QueryRow("SELECT DB_NAME()").Scan(&dbName)
		if err != nil {
			t.Fatalf("extract MSSQL connection info: %v", err)
		}
	}
	// In Docker, MSSQL port is typically 1433
	return &engineConn{db: db, engine: "mssql", host: host, port: 1433, dbName: dbName, user: "sa"}
}

// extractSQLiteInfo extracts the database file path from a SQLite connection.
func extractSQLiteInfo(t *testing.T, db *sql.DB) *engineConn {
	t.Helper()
	var dbPath string
	err := db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&dbPath)
	if err != nil {
		t.Fatalf("extract SQLite database info: %v", err)
	}
	return &engineConn{db: db, engine: "sqlite", host: "localhost", port: 0, dbName: dbPath, user: ""}
}
