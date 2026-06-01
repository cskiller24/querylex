package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/querylex/querylex/internal/format"
)

// WorkspaceError wraps an error code and underlying error for workspace operations.
type WorkspaceError struct {
	Code format.ErrorCode
	Err  error
}

func (e *WorkspaceError) Error() string {
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *WorkspaceError) Unwrap() error {
	return e.Err
}

// ErrWorkspaceStateInvalid is a sentinel error for malformed workspace state.
// It wraps format.ErrCodeWorkspaceStateInvalid as a Go error.
var ErrWorkspaceStateInvalid = &WorkspaceError{
	Code: format.ErrCodeWorkspaceStateInvalid,
	Err:  errors.New("workspace state is missing, malformed, or internally inconsistent"),
}

// DatabaseStatus represents the indexing status of a connected database.
type DatabaseStatus string

const (
	StatusNotIndexed  DatabaseStatus = "not_indexed"
	StatusIndexing    DatabaseStatus = "indexing"
	StatusIndexed     DatabaseStatus = "indexed"
	StatusIndexFailed DatabaseStatus = "index_failed"
	StatusStale       DatabaseStatus = "stale"
)

// DatabaseEntry describes a single connected database in the workspace.
type DatabaseEntry struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Type             string         `json:"type"`
	Status           DatabaseStatus `json:"status"`
	IndexingProgress int            `json:"indexing_progress"`
}

// Workspace represents the content of querylex.json — the workspace registry.
type Workspace struct {
	ConnectedDatabases []DatabaseEntry `json:"connected_databases"`
	ActiveDatabaseID   *string         `json:"active_database_id"`
	Revision           int64           `json:"revision"`
	UpdatedAt          string          `json:"updated_at"`
}

// WorkspaceStore defines the interface for workspace state operations.
// Implementations include FileWorkspaceStore (production, backed by
// querylex.json) and InMemoryWorkspaceStore (unit tests, per D-07).
type WorkspaceStore interface {
	// Load reads and parses the workspace state.
	// Returns an empty Workspace with Revision=0 if the file does not exist.
	Load() (*Workspace, error)

	// Save persists the workspace state atomically with revision tracking.
	Save(ws *Workspace) error

	// AddDatabase appends a new database entry and saves.
	AddDatabase(entry DatabaseEntry) error

	// RemoveDatabase removes a database entry by ID and saves.
	RemoveDatabase(id string) error

	// SetActiveDatabase sets the active database ID and saves.
	SetActiveDatabase(id string) error

	// GetRevision returns the current revision number.
	GetRevision() (int64, error)
}

// FileWorkspaceStore is a production WorkspaceStore backed by a querylex.json file.
// It uses advisory file locking and atomic writes for durability.
type FileWorkspaceStore struct {
	path string // path to querylex.json
}

// NewFileWorkspaceStore creates a FileWorkspaceStore for the given file path.
func NewFileWorkspaceStore(path string) *FileWorkspaceStore {
	return &FileWorkspaceStore{path: path}
}

// Load reads and parses the workspace file. Returns an empty workspace if the
// file does not exist. On malformed JSON, returns ErrCodeWorkspaceStateInvalid.
func (s *FileWorkspaceStore) Load() (*Workspace, error) {
	lock := NewFileLock(s.path)
	if err := lock.Acquire(LockShared); err != nil {
		return nil, fmt.Errorf("acquire shared lock: %w", err)
	}
	defer lock.Release()

	data, err := AtomicRead(s.path)
	if err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}

	// File doesn't exist or is empty — return empty workspace
	if data == nil {
		return &Workspace{}, nil
	}

	var ws Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("malformed JSON in %s: %w: %w",
			s.path, ErrWorkspaceStateInvalid, err)
	}

	return &ws, nil
}

// Save persists the workspace atomically. It increments the revision number
// and sets UpdatedAt to the current time (RFC3339).
func (s *FileWorkspaceStore) Save(ws *Workspace) error {
	lock := NewFileLock(s.path)
	if err := lock.Acquire(LockExclusive); err != nil {
		return fmt.Errorf("acquire exclusive lock: %w", err)
	}
	defer lock.Release()

	ws.Revision++
	ws.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(ws)
	if err != nil {
		return fmt.Errorf("marshal workspace: %w", err)
	}

	if err := AtomicWrite(s.path, data); err != nil {
		return fmt.Errorf("write workspace: %w", err)
	}

	return nil
}

// AddDatabase appends a database entry and saves the workspace.
func (s *FileWorkspaceStore) AddDatabase(entry DatabaseEntry) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}
	ws.ConnectedDatabases = append(ws.ConnectedDatabases, entry)
	return s.Save(ws)
}

// RemoveDatabase removes a database entry by ID and saves the workspace.
// If the ID is not found, no error is returned (no-op).
func (s *FileWorkspaceStore) RemoveDatabase(id string) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}

	filtered := make([]DatabaseEntry, 0, len(ws.ConnectedDatabases))
	for _, entry := range ws.ConnectedDatabases {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	ws.ConnectedDatabases = filtered

	// Clear active database if it was the removed one
	if ws.ActiveDatabaseID != nil && *ws.ActiveDatabaseID == id {
		ws.ActiveDatabaseID = nil
	}

	return s.Save(ws)
}

// SetActiveDatabase sets the active database ID and saves the workspace.
func (s *FileWorkspaceStore) SetActiveDatabase(id string) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}
	ws.ActiveDatabaseID = &id
	return s.Save(ws)
}

// GetRevision returns the current revision number from the workspace.
func (s *FileWorkspaceStore) GetRevision() (int64, error) {
	ws, err := s.Load()
	if err != nil {
		return 0, err
	}
	return ws.Revision, nil
}

// InMemoryWorkspaceStore is an in-memory WorkspaceStore for unit tests (D-07).
// It uses a sync.RWMutex for goroutine safety.
type InMemoryWorkspaceStore struct {
	mu sync.RWMutex
	ws *Workspace
}

// NewInMemoryWorkspaceStore creates an InMemoryWorkspaceStore with an empty workspace.
func NewInMemoryWorkspaceStore() *InMemoryWorkspaceStore {
	return &InMemoryWorkspaceStore{ws: &Workspace{}}
}

// Load returns the current workspace state.
func (s *InMemoryWorkspaceStore) Load() (*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ws == nil {
		return &Workspace{}, nil
	}

	// Return a copy to prevent external mutation
	wsCopy := *s.ws
	return &wsCopy, nil
}

// Save stores the workspace state with revision tracking.
func (s *InMemoryWorkspaceStore) Save(ws *Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ws.Revision++
	ws.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	wsCopy := *ws
	s.ws = &wsCopy
	return nil
}

// AddDatabase appends a database entry and saves.
func (s *InMemoryWorkspaceStore) AddDatabase(entry DatabaseEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ws == nil {
		s.ws = &Workspace{}
	}
	s.ws.ConnectedDatabases = append(s.ws.ConnectedDatabases, entry)
	s.ws.Revision++
	s.ws.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// RemoveDatabase removes a database entry by ID.
func (s *InMemoryWorkspaceStore) RemoveDatabase(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ws == nil {
		return nil
	}

	filtered := make([]DatabaseEntry, 0, len(s.ws.ConnectedDatabases))
	for _, entry := range s.ws.ConnectedDatabases {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	s.ws.ConnectedDatabases = filtered

	if s.ws.ActiveDatabaseID != nil && *s.ws.ActiveDatabaseID == id {
		s.ws.ActiveDatabaseID = nil
	}

	s.ws.Revision++
	s.ws.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// SetActiveDatabase sets the active database ID.
func (s *InMemoryWorkspaceStore) SetActiveDatabase(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ws == nil {
		s.ws = &Workspace{}
	}
	s.ws.ActiveDatabaseID = &id
	s.ws.Revision++
	s.ws.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// GetRevision returns the current revision number.
func (s *InMemoryWorkspaceStore) GetRevision() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ws == nil {
		return 0, nil
	}
	return s.ws.Revision, nil
}
