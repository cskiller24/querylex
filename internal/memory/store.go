package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

// MemoryEntry represents a single saved query in the memory store.
type MemoryEntry struct {
	ID                  string `json:"id"`
	Input               string `json:"input"`
	SQL                 string `json:"sql,omitempty"`
	SQLHash             string `json:"sql_hash,omitempty"`
	MatchType           string `json:"match_type,omitempty"`
	OptimizationSummary string `json:"optimization_summary,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
	LastUsedAt          string `json:"last_used_at,omitempty"`
	DatabaseID          string `json:"database_id,omitempty"`
}

// OpenStore opens (or creates) the memory.sqlite database at the given dbDir.
// The database is opened with WAL journal mode for concurrent reads.
// Creates the directory if it does not exist.
func OpenStore(dbDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}
	dbPath := filepath.Join(dbDir, "memory.sqlite")
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open memory.sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return db, nil
}

// createTables ensures the memory store schema exists.
func createTables(db *sql.DB) error {
	var err error

	entriesSQL := `CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		input TEXT NOT NULL UNIQUE,
		sql TEXT NOT NULL,
		sql_hash TEXT NOT NULL,
		match_type TEXT NOT NULL,
		optimization_summary TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_used_at TEXT,
		database_id TEXT NOT NULL
	)`
	if _, err := db.Exec(entriesSQL); err != nil {
		return fmt.Errorf("create entries table: %w", err)
	}

	metaSQL := `CREATE TABLE IF NOT EXISTS _meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`
	if _, err := db.Exec(metaSQL); err != nil {
		return fmt.Errorf("create _meta table: %w", err)
	}

	// Create FTS5 virtual table for full-text search on input and sql columns
	ftsSQL := `CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
		input, sql, content='entries', content_rowid='rowid'
	)`
	if _, err := db.Exec(ftsSQL); err != nil {
		return fmt.Errorf("create entries_fts: %w", err)
	}

	// Triggers to keep FTS5 index in sync with entries table
	_, err = db.Exec(`CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
		INSERT INTO entries_fts(rowid, input, sql) VALUES (new.rowid, new.input, new.sql);
	END`)
	if err != nil {
		return fmt.Errorf("create entries_ai trigger: %w", err)
	}

	_, err = db.Exec(`CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, input, sql) VALUES('delete', old.rowid, old.input, old.sql);
	END`)
	if err != nil {
		return fmt.Errorf("create entries_ad trigger: %w", err)
	}

	_, err = db.Exec(`CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, input, sql) VALUES('delete', old.rowid, old.input, old.sql);
		INSERT INTO entries_fts(rowid, input, sql) VALUES (new.rowid, new.input, new.sql);
	END`)
	if err != nil {
		return fmt.Errorf("create entries_au trigger: %w", err)
	}

	// Insert default index_revision if not exists
	_, err = db.Exec(`INSERT OR IGNORE INTO _meta (key, value) VALUES ('index_revision', '0')`)
	if err != nil {
		return fmt.Errorf("insert default index_revision: %w", err)
	}

	return nil
}

// SaveEntry upserts a memory entry. It returns the saved entry, whether it
// updated an existing entry, and any error. On success, the _meta.index_revision
// is incremented atomically within the same transaction.
func SaveEntry(ctx context.Context, db *sql.DB, input, sql, dbID string) (*MemoryEntry, bool, error) {
	normalizedInput := NormalizeInput(input)
	normalizedSQL := strings.TrimSpace(sql)
	normalizedSQL = whitespaceRe.ReplaceAllString(normalizedSQL, " ")
	hash := sqlHash(normalizedSQL)
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("MEMORY_WRITE_FAILED: begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if entry already exists
	var existingID string
	var existingCreatedAt string
	err = tx.QueryRowContext(ctx, `SELECT id, created_at FROM entries WHERE input = ?`, normalizedInput).Scan(&existingID, &existingCreatedAt)
	updatedExisting := err == nil

	var entryID string
	var createdAt string
	if updatedExisting {
		entryID = existingID
		createdAt = existingCreatedAt
	} else {
		entryID = generateMemoryID()
		createdAt = now
	}

	matchType := "question_to_sql"

	_, err = tx.ExecContext(ctx, `
		INSERT INTO entries (id, input, sql, sql_hash, match_type, created_at, updated_at, database_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(input) DO UPDATE SET
			sql = excluded.sql,
			sql_hash = excluded.sql_hash,
			updated_at = excluded.updated_at,
			match_type = excluded.match_type
	`, entryID, normalizedInput, normalizedSQL, hash, matchType, createdAt, now, dbID)
	if err != nil {
		return nil, false, fmt.Errorf("MEMORY_WRITE_FAILED: upsert entry: %w", err)
	}

	// Increment index revision
	_, err = tx.ExecContext(ctx, `UPDATE _meta SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT) WHERE key = 'index_revision'`)
	if err != nil {
		return nil, false, fmt.Errorf("MEMORY_WRITE_FAILED: increment revision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("MEMORY_WRITE_FAILED: commit transaction: %w", err)
	}

	entry := &MemoryEntry{
		ID:        entryID,
		Input:     normalizedInput,
		SQL:       normalizedSQL,
		SQLHash:   hash,
		MatchType: matchType,
		CreatedAt: createdAt,
		UpdatedAt: now,
		DatabaseID: dbID,
	}

	return entry, updatedExisting, nil
}

// GetEntryByID retrieves an entry by its ID. Returns nil, nil if not found.
func GetEntryByID(ctx context.Context, db *sql.DB, id string) (*MemoryEntry, error) {
	entry := &MemoryEntry{}
	err := db.QueryRowContext(ctx, `
		SELECT id, input, sql, sql_hash, match_type, COALESCE(optimization_summary, ''),
		       created_at, updated_at, COALESCE(last_used_at, ''), database_id
		FROM entries WHERE id = ?
	`, id).Scan(
		&entry.ID, &entry.Input, &entry.SQL, &entry.SQLHash,
		&entry.MatchType, &entry.OptimizationSummary,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.LastUsedAt,
		&entry.DatabaseID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entry by id: %w", err)
	}

	// Update last_used_at best-effort
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.ExecContext(ctx, `UPDATE entries SET last_used_at = ? WHERE id = ?`, now, id)

	return entry, nil
}

// GetEntry retrieves an entry by normalized input. It updates the last_used_at
// timestamp on access. Returns nil, nil if not found.
func GetEntry(ctx context.Context, db *sql.DB, normalizedInput string) (*MemoryEntry, error) {
	entry := &MemoryEntry{}
	err := db.QueryRowContext(ctx, `
		SELECT id, input, sql, sql_hash, match_type, COALESCE(optimization_summary, ''),
		       created_at, updated_at, COALESCE(last_used_at, ''), database_id
		FROM entries WHERE input = ?
	`, normalizedInput).Scan(
		&entry.ID, &entry.Input, &entry.SQL, &entry.SQLHash,
		&entry.MatchType, &entry.OptimizationSummary,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.LastUsedAt,
		&entry.DatabaseID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entry: %w", err)
	}

	// Update last_used_at best-effort
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.ExecContext(ctx, `UPDATE entries SET last_used_at = ? WHERE input = ?`, now, normalizedInput)

	return entry, nil
}

// DeleteEntry deletes an entry by normalized input. It returns true if an entry
// was deleted. The _meta.index_revision is incremented on successful deletion.
func DeleteEntry(ctx context.Context, db *sql.DB, normalizedInput string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE input = ?`, normalizedInput)
	if err != nil {
		return false, fmt.Errorf("delete entry: %w", err)
	}

	rows, _ := res.RowsAffected()
	deleted := rows > 0

	if deleted {
		_, err = tx.ExecContext(ctx, `UPDATE _meta SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT) WHERE key = 'index_revision'`)
		if err != nil {
			return false, fmt.Errorf("increment revision: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit transaction: %w", err)
	}

	return deleted, nil
}

// ListEntries returns all entries ordered by created_at DESC.
func ListEntries(ctx context.Context, db *sql.DB) ([]MemoryEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, input, sql, sql_hash, match_type, COALESCE(optimization_summary, ''),
		       created_at, updated_at, COALESCE(last_used_at, ''), database_id
		FROM entries ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
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
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if entries == nil {
		entries = []MemoryEntry{}
	}

	return entries, nil
}

// GetRevision returns the current index_revision from the _meta table.
func GetRevision(ctx context.Context, db *sql.DB) (int, error) {
	var revision int
	err := db.QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM _meta WHERE key = 'index_revision'`).Scan(&revision)
	if err != nil {
		return 0, fmt.Errorf("get revision: %w", err)
	}
	return revision, nil
}

// IncrementRevision increments the index_revision by 1.
func IncrementRevision(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `UPDATE _meta SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT) WHERE key = 'index_revision'`)
	if err != nil {
		return fmt.Errorf("increment revision: %w", err)
	}
	return nil
}

// generateMemoryID generates a unique memory entry ID with the format "mem_" + ULID.
func generateMemoryID() string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		panic(fmt.Errorf("generate ULID: %w", err))
	}
	return "mem_" + id.String()
}

// whitespaceRe matches one or more whitespace characters.
var whitespaceRe = regexp.MustCompile(`\s+`)

// NormalizeInput normalizes a string for consistency: lowercase, trim, collapse spaces.
func NormalizeInput(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	return whitespaceRe.ReplaceAllString(s, " ")
}

// sqlHash computes the SHA-256 hash of a normalized SQL string.
// Returns the format "sha256:<first 12 hex chars>".
func sqlHash(normalizedSQL string) string {
	h := sha256.Sum256([]byte(normalizedSQL))
	return "sha256:" + hex.EncodeToString(h[:6])
}
