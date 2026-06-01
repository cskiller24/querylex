package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileLockSharedAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.json")
	fl := NewFileLock(lockPath)

	if err := fl.Acquire(LockShared); err != nil {
		t.Fatalf("Acquire(shared) failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}

func TestFileLockExclusiveAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.json")
	fl := NewFileLock(lockPath)

	if err := fl.Acquire(LockExclusive); err != nil {
		t.Fatalf("Acquire(exclusive) failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}

func TestFileLockReacquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.json")
	fl := NewFileLock(lockPath)

	// Acquire and release exclusive
	if err := fl.Acquire(LockExclusive); err != nil {
		t.Fatalf("First Acquire(exclusive) failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("First Release failed: %v", err)
	}

	// Acquire again — should succeed
	if err := fl.Acquire(LockExclusive); err != nil {
		t.Fatalf("Second Acquire(exclusive) failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("Second Release failed: %v", err)
	}
}

func TestFileLockSharedConcurrent(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "concurrent.json")

	// Both goroutines should be able to hold shared locks concurrently.
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fl := NewFileLock(lockPath)
			if err := fl.Acquire(LockShared); err != nil {
				errs <- err
				return
			}
			defer fl.Release()
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Concurrent shared lock failed: %v", err)
		}
	}
}

func TestFileLockLockFileCreated(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "data.json")
	fl := NewFileLock(lockPath)

	if err := fl.Acquire(LockShared); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer fl.Release()

	// Verify lock file exists alongside data file
	expectedLockFile := lockPath + ".lock"
	if _, err := os.Stat(expectedLockFile); os.IsNotExist(err) {
		t.Fatalf("Lock file %s was not created", expectedLockFile)
	}
}

func TestFileLockExclusiveBlocksShared(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "exclusive_block.json")

	// Hold an exclusive lock in goroutine A
	flExclusive := NewFileLock(lockPath)
	if err := flExclusive.Acquire(LockExclusive); err != nil {
		t.Fatalf("Acquire exclusive failed: %v", err)
	}
	defer flExclusive.Release()

	// In this goroutine, try to acquire shared — should timeout
	// because the exclusive lock is held by goroutine A.
	// We use a separate FileLock instance (different file handle).
	ch := make(chan error, 1)
	go func() {
		flShared := NewFileLock(lockPath)
		ch <- flShared.Acquire(LockShared)
	}()

	err := <-ch
	if err != ErrLockAcquisitionTimeout {
		t.Fatalf("Expected ErrLockAcquisitionTimeout, got: %v", err)
	}
}

func TestFileLockDoubleReleaseIsSafe(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "double_release.json")
	fl := NewFileLock(lockPath)

	if err := fl.Acquire(LockShared); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("First Release failed: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("Second Release (no-op) should not error: %v", err)
	}
}
