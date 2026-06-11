package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRetrieve_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	ref, err := store.Store("test-account", "test-secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if ref.Account != "test-account" {
		t.Errorf("expected account test-account, got %s", ref.Account)
	}

	secret, err := store.Retrieve(ref)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if secret != "test-secret" {
		t.Errorf("expected 'test-secret', got '%s'", secret)
	}
}

func TestRetrieve_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	_, err := store.Retrieve(&CredentialReference{Account: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent credential, got nil")
	}
}

func TestTamperedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	_, err := store.Store("account", "secret")
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

	_, err = store.Retrieve(&CredentialReference{Account: "account"})
	if err == nil {
		t.Fatal("expected error for tampered file, got nil")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	ref, err := store.Store("delete-account", "delete-secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if err := store.Delete("delete-account"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Retrieve(ref)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
