//go:build unix

package state

import (
	"golang.org/x/sys/unix"
)

// acquireFlock attempts to acquire an advisory lock on the file descriptor
// using unix.Flock with non-blocking mode.
//
// LockShared  → unix.LOCK_SH | unix.LOCK_NB
// LockExclusive → unix.LOCK_EX | unix.LOCK_NB
//
// Returns nil on success, or an error if the lock is already held by
// another process. The caller (osFileLock.Acquire) handles retry with
// bounded exponential backoff.
func acquireFlock(fd uintptr, mode LockMode) error {
	var how int
	switch mode {
	case LockShared:
		how = unix.LOCK_SH | unix.LOCK_NB
	case LockExclusive:
		how = unix.LOCK_EX | unix.LOCK_NB
	default:
		how = unix.LOCK_EX | unix.LOCK_NB
	}
	return unix.Flock(int(fd), how)
}

// releaseFlock releases the advisory lock using unix.LOCK_UN.
func releaseFlock(fd uintptr) error {
	return unix.Flock(int(fd), unix.LOCK_UN)
}
