//go:build windows

package state

import (
	"syscall"
	"unsafe"
)

// acquireFlock attempts to acquire an advisory lock on the file descriptor
// using syscall.LockFileEx with non-blocking mode.
//
// LockShared    → no exclusive flag (shared/read lock)
// LockExclusive → LOCKFILE_EXCLUSIVE_LOCK (write lock)
//
// Returns nil on success, or an error if the lock is already held.
// The caller (osFileLock.Acquire) handles retry with bounded exponential backoff.
func acquireFlock(fd uintptr, mode LockMode) error {
	handle := syscall.Handle(fd)

	var flags uint32
	flags = 0 // shared (read) lock — no exclusive flag
	if mode == LockExclusive {
		flags |= syscall.LOCKFILE_EXCLUSIVE_LOCK
	}
	// Non-blocking: fail immediately if lock cannot be acquired.
	flags |= syscall.LOCKFILE_FAIL_IMMEDIATELY

	// LockFileEx requires an OVERLAPPED structure for the byte range.
	// We lock bytes 0..0 (entire file) using a zeroed OVERLAPPED,
	// which locks from the current file position (0) for the whole file.
	var overlapped syscall.Overlapped
	return syscall.LockFileEx(handle, flags, 0, 1, 0, &overlapped)
}

// releaseFlock releases the advisory lock using syscall.UnlockFileEx.
func releaseFlock(fd uintptr) error {
	handle := syscall.Handle(fd)
	var overlapped syscall.Overlapped
	return syscall.UnlockFileEx(handle, 0, 1, 0, &overlapped)
}
