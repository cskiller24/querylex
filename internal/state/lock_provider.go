package state

import (
	"errors"
	"time"
)

// LockMode represents the type of file lock to acquire.
type LockMode int

const (
	// LockShared is a read lock. Multiple readers may hold it concurrently.
	LockShared LockMode = 0
	// LockExclusive is a write lock. Only one writer may hold it at a time.
	LockExclusive LockMode = 1
)

// FileLock provides advisory file locking with shared and exclusive modes.
// Lock files are created alongside the target data file with a ".lock" suffix.
// Locks are bound to the file descriptor and auto-released on process exit.
type FileLock interface {
	// Acquire attempts to acquire the lock with the given mode.
	// Returns ErrLockAcquisitionTimeout if the lock cannot be acquired
	// within the bounded retry period.
	Acquire(mode LockMode) error

	// Release releases the acquired lock.
	Release() error
}

// ErrLockAcquisitionTimeout is returned when a lock cannot be acquired
// after the bounded exponential backoff retry period.
var ErrLockAcquisitionTimeout = errors.New("lock acquisition timed out")

// Backoff constants for lock acquisition retry.
const (
	LockBaseDelay  = 50 * time.Millisecond
	LockMaxDelay   = 2 * time.Second
	LockMaxRetries = 5
)

// NewFileLock creates a new FileLock for the given data file path.
// The lock file is created at {path}.lock alongside the data file.
func NewFileLock(path string) FileLock {
	return &osFileLock{lockPath: path + ".lock"}
}
