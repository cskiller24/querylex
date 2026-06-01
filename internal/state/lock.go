package state

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

// osFileLock implements FileLock using OS-level advisory locking.
// It holds the lock file handle and defers to platform-specific
// acquire/release functions via build-tag-separated files.
type osFileLock struct {
	lockPath string   // path to the .lock file
	file     *os.File // open file handle for the lock file
	mode     LockMode // current lock mode
}

// Acquire attempts to acquire the lock with bounded exponential backoff.
// It retries up to LockMaxRetries times with delay starting at LockBaseDelay
// and doubling each retry, capped at LockMaxDelay, with jitter.
func (l *osFileLock) Acquire(mode LockMode) error {
	f, err := os.OpenFile(l.lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file %s: %w", l.lockPath, err)
	}

	l.file = f
	l.mode = mode

	delay := LockBaseDelay
	for i := 0; i < LockMaxRetries; i++ {
		err := acquireFlock(f.Fd(), mode)
		if err == nil {
			return nil
		}

		// On the last attempt, don't sleep — just return the error.
		if i == LockMaxRetries-1 {
			break
		}

		// Sleep with jitter: add up to 50% of current delay.
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		time.Sleep(delay + jitter)

		delay *= 2
		if delay > LockMaxDelay {
			delay = LockMaxDelay
		}
	}

	// All retries exhausted — cleanup and return timeout.
	l.file.Close()
	l.file = nil
	return ErrLockAcquisitionTimeout
}

// Release releases the lock and closes the lock file.
func (l *osFileLock) Release() error {
	if l.file == nil {
		return nil
	}
	err := releaseFlock(l.file.Fd())
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
