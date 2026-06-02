package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnlock_FirstUse(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// First use — file doesn't exist, Unlock should return nil
	err := store.Unlock("my-passphrase")
	if err != nil {
		t.Fatalf("Unlock on first use should return nil, got: %v", err)
	}

	// After Unlock, we should be able to store a credential
	ref, err := store.Store("test-account", "test-secret")
	if err != nil {
		t.Fatalf("Store after Unlock failed: %v", err)
	}
	if ref.Account != "test-account" {
		t.Errorf("expected account test-account, got %s", ref.Account)
	}
}

func TestUnlock_CorrectPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// First, create an encrypted file with a known passphrase
	err := store.Unlock("correct-passphrase")
	if err != nil {
		t.Fatalf("Unlock (first use) failed: %v", err)
	}

	_, err = store.Store("stored-account", "stored-secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Now create a new store pointing to the same file and unlock with correct passphrase
	store2 := NewEncryptedFileStore(filePath)
	err = store2.Unlock("correct-passphrase")
	if err != nil {
		t.Fatalf("Unlock with correct passphrase should return nil, got: %v", err)
	}

	// Verify we can retrieve the stored credential
	secret, err := store2.Retrieve(&CredentialReference{Account: "stored-account"})
	if err != nil {
		t.Fatalf("Retrieve after Unlock failed: %v", err)
	}
	if secret != "stored-secret" {
		t.Errorf("expected 'stored-secret', got '%s'", secret)
	}
}

func TestUnlock_WrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// Create encrypted file with one passphrase
	err := store.Unlock("correct-passphrase")
	if err != nil {
		t.Fatalf("Unlock (first use) failed: %v", err)
	}
	_, err = store.Store("account", "secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Try to unlock with wrong passphrase
	store2 := NewEncryptedFileStore(filePath)
	err = store2.Unlock("wrong-passphrase")
	if err == nil {
		t.Fatal("Unlock with wrong passphrase should return error, got nil")
	}
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("expected ErrWrongPassphrase, got: %v", err)
	}
}

func TestUnlock_TamperedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// Create encrypted file
	err := store.Unlock("test-passphrase")
	if err != nil {
		t.Fatalf("Unlock (first use) failed: %v", err)
	}
	_, err = store.Store("account", "secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Tamper with the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	data[len(data)-1] ^= 0xFF // flip last byte to corrupt GCM tag
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Try to unlock with the correct passphrase (but file is tampered)
	store2 := NewEncryptedFileStore(filePath)
	err = store2.Unlock("test-passphrase")
	if err == nil {
		t.Fatal("Unlock on tampered file should return error, got nil")
	}
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("expected ErrWrongPassphrase for tampered file, got: %v", err)
	}
}

func TestUnlock_RetrieveWithoutUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// Without calling Unlock or SetPassphrase, Retrieve should fail
	_, err := store.Retrieve(&CredentialReference{Account: "any"})
	if err == nil {
		t.Fatal("Retrieve without Unlock should return error, got nil")
	}
	if !errors.Is(err, ErrPassphraseRequired) {
		t.Errorf("expected ErrPassphraseRequired, got: %v", err)
	}
}

func TestSetPassphrase_Deprecated(t *testing.T) {
	// Verify that SetPassphrase still works (backward compatibility)
	// but is documented as deprecated.
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	store.SetPassphrase("old-way-passphrase")
	ref, err := store.Store("account", "secret")
	if err != nil {
		t.Fatalf("Store via SetPassphrase failed: %v", err)
	}
	if ref.Account != "account" {
		t.Errorf("expected account 'account', got %s", ref.Account)
	}
}
