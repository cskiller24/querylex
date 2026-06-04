package testhelper

import (
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ConnectMySQL resolves TEST_MYSQL_DSN, waits for the port, opens a MySQL
// connection, creates an isolated per-test database, and returns *sql.DB.
func ConnectMySQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:testpass@tcp(localhost:3306)/testdb?parseTime=true"
	}
	host, port := ExtractHostPort(dsn)
	WaitForPort(t, host, port, 30*time.Second)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open MySQL connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	// Retry ping with exponential backoff (max 10 attempts, 500ms base)
	pingWithBackoff(t, db)

	// Create and switch to per-test database
	dbName := GenerateDBName()
	_, err = db.Exec("CREATE DATABASE " + dbName)
	if err != nil {
		t.Fatalf("failed to create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE " + dbName)
	})

	_, err = db.Exec("USE " + dbName)
	if err != nil {
		t.Fatalf("failed to switch to database %s: %v", dbName, err)
	}

	return db
}

// ConnectPostgreSQL resolves TEST_PG_DSN, waits for the port, opens a
// PostgreSQL connection, creates an isolated per-test database, and returns *sql.DB.
func ConnectPostgreSQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "postgres:testpass@localhost:5432/testdb?sslmode=disable"
	}
	host, port := ExtractHostPort(dsn)
	WaitForPort(t, host, port, 30*time.Second)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	pingWithBackoff(t, db)

	dbName := GenerateDBName()
	_, err = db.Exec("CREATE DATABASE " + dbName)
	if err != nil {
		t.Fatalf("failed to create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE " + dbName)
	})

	_, err = db.Exec("USE " + dbName)
	if err != nil {
		t.Fatalf("failed to switch to database %s: %v", dbName, err)
	}

	return db
}

// ConnectMariaDB resolves TEST_MARIADB_DSN, waits for the port, opens a
// MariaDB connection, creates an isolated per-test database, and returns *sql.DB.
func ConnectMariaDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MARIADB_DSN")
	if dsn == "" {
		dsn = "root:testpass@tcp(localhost:3306)/testdb?parseTime=true"
	}
	host, port := ExtractHostPort(dsn)
	WaitForPort(t, host, port, 30*time.Second)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open MariaDB connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	pingWithBackoff(t, db)

	dbName := GenerateDBName()
	_, err = db.Exec("CREATE DATABASE " + dbName)
	if err != nil {
		t.Fatalf("failed to create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE " + dbName)
	})

	_, err = db.Exec("USE " + dbName)
	if err != nil {
		t.Fatalf("failed to switch to database %s: %v", dbName, err)
	}

	return db
}

// ConnectMSSQL resolves TEST_MSSQL_DSN, waits for the port, opens a SQL Server
// connection, creates an isolated per-test database, and returns *sql.DB.
func ConnectMSSQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MSSQL_DSN")
	if dsn == "" {
		dsn = "sqlserver://sa:TestPass123!@localhost:1433?database=testdb&encrypt=false"
	}
	host, port := ExtractHostPort(dsn)
	WaitForPort(t, host, port, 30*time.Second)

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("failed to open SQL Server connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	pingWithBackoff(t, db)

	dbName := GenerateDBName()
	_, err = db.Exec("CREATE DATABASE " + dbName)
	if err != nil {
		t.Fatalf("failed to create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP DATABASE " + dbName)
	})

	_, err = db.Exec("USE " + dbName)
	if err != nil {
		t.Fatalf("failed to switch to database %s: %v", dbName, err)
	}

	return db
}

// ExtractHostPort parses host and port from a DSN string. Supports three formats:
//   - MySQL/MariaDB: user:pass@tcp(host:port)/db
//   - PostgreSQL:    user:pass@host:port/db
//   - MSSQL:         scheme://user:pass@host:port?params
func ExtractHostPort(dsn string) (string, int) {
	hp := ""

	// Try MySQL/MariaDB format: @tcp(host:port)
	if idx := strings.Index(dsn, "@tcp("); idx >= 0 {
		start := idx + 5 // len("@tcp(")
		end := strings.Index(dsn[start:], ")")
		if end >= 0 {
			hp = dsn[start : start+end]
		}
	}

	// Try general format: @host:port followed by / or ? or end
	if hp == "" {
		atIdx := strings.LastIndex(dsn, "@")
		if atIdx >= 0 {
			afterAt := dsn[atIdx+1:]
			endIdx := len(afterAt)
			if qi := strings.Index(afterAt, "/"); qi >= 0 && qi < endIdx {
				endIdx = qi
			}
			if qi := strings.Index(afterAt, "?"); qi >= 0 && qi < endIdx {
				endIdx = qi
			}
			hp = afterAt[:endIdx]
		}
	}

	if hp == "" {
		return "localhost", 3306
	}

	host, portStr, err := net.SplitHostPort(hp)
	if err != nil {
		return "localhost", 3306
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 3306
	}
	return host, port
}

// pingWithBackoff retries db.Ping with exponential backoff (max 10 attempts,
// 500ms base delay).
func pingWithBackoff(t *testing.T, db *sql.DB) {
	t.Helper()
	for i := 0; i < 10; i++ {
		if err := db.Ping(); err == nil {
			return
		}
		time.Sleep(time.Duration(500*(1<<i)) * time.Millisecond)
	}
	t.Fatalf("failed to ping database after 10 retries")
}

// ConnectSQLite resolves TEST_SQLITE_PATH (or auto-generates temp file), opens
// a SQLite connection via modernc.org/sqlite, and returns *sql.DB.
// No Docker required — SQLite runs in-process via the pure-Go driver.
//
// SQLite databases are stored as files. The function creates a temp .db file
// via t.TempDir() to ensure per-test isolation. The DSN uses absolute file
// paths to avoid working directory issues with the querylex subprocess.
func ConnectSQLite(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_SQLITE_PATH")
	if dsn == "" {
		// Auto-generate temp file with absolute path
		dbPath := filepath.Join(t.TempDir(), "e2e_test.db")
		dsn = "file:" + dbPath + "?_pragma=foreign_keys(1)"
	}

	// SQLite does not need port waiting (in-process)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open SQLite connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	// Ping to verify file was created
	pingWithBackoff(t, db)

	return db
}
