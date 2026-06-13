package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type MockCredentialStore struct {
	store map[string]string
}

func NewMockCredentialStore() *MockCredentialStore {
	return &MockCredentialStore{store: make(map[string]string)}
}

func (m *MockCredentialStore) Store(account string, secret string) (*CredentialReference, error) {
	m.store[account] = secret
	return &CredentialReference{Provider: "mock", Account: account, SecretKind: "database-password"}, nil
}

func (m *MockCredentialStore) Retrieve(ref *CredentialReference) (string, error) {
	secret, ok := m.store[ref.Account]
	if !ok {
		return "", errors.New("mock: credential not found")
	}
	return secret, nil
}

func (m *MockCredentialStore) Delete(account string) error {
	delete(m.store, account)
	return nil
}

func (m *MockCredentialStore) Available() bool {
	return true
}

func TestMockCredentialStoreRoundTrip(t *testing.T) {
	s := NewMockCredentialStore()
	ref, err := s.Store("test-account", "test-secret-123")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if ref.Account != "test-account" {
		t.Fatalf("expected account test-account, got %s", ref.Account)
	}
	secret, err := s.Retrieve(ref)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if secret != "test-secret-123" {
		t.Fatalf("expected test-secret-123, got %s", secret)
	}
}

func TestMockCredentialStoreDelete(t *testing.T) {
	s := NewMockCredentialStore()
	ref, err := s.Store("delete-account", "delete-secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := s.Delete("delete-account"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = s.Retrieve(ref)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestEncryptedFileRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	ref, err := store.Store("encrypted-account", "encrypted-secret-456")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if ref.Account != "encrypted-account" {
		t.Fatalf("expected account encrypted-account, got %s", ref.Account)
	}

	secret, err := store.Retrieve(ref)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if secret != "encrypted-secret-456" {
		t.Fatalf("expected encrypted-secret-456, got %s", secret)
	}
}

func TestEncryptedFileTampering(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	_, err := store.Store("test", "test-secret")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	data[len(data)-1] ^= 0xFF

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	store2 := NewEncryptedFileStore(filePath)
	_, err = store2.Retrieve(&CredentialReference{Account: "test"})
	if err == nil {
		t.Fatal("expected ErrTamperedFile on tampered data, got nil")
	}
}

func TestEnvStore(t *testing.T) {
	os.Setenv("QUERYLEX_DB_PASSWORD", "env-test-password")
	defer os.Unsetenv("QUERYLEX_DB_PASSWORD")

	store := NewEnvStore()
	if !store.Available() {
		t.Fatal("expected Available() true when env var is set")
	}

	secret, err := store.Retrieve(&CredentialReference{
		Account:    "test",
		SecretKind: "database-password",
	})
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if secret != "env-test-password" {
		t.Fatalf("expected env-test-password, got %s", secret)
	}
}

func TestEnvStoreUnavailable(t *testing.T) {
	store := NewEnvStore()
	if store.Available() {
		t.Fatal("expected Available() false when no env vars set")
	}

	_, err := store.Retrieve(&CredentialReference{
		Account:    "test",
		SecretKind: "database-password",
	})
	if err == nil {
		t.Fatal("expected error when no env vars set, got nil")
	}
}

func TestEnvStoreStoreAndDeleteErrors(t *testing.T) {
	store := NewEnvStore()
	_, err := store.Store("account", "secret")
	if err == nil {
		t.Fatal("expected error on Store, got nil")
	}
	err = store.Delete("account")
	if err == nil {
		t.Fatal("expected error on Delete, got nil")
	}
}

func TestEncryptedFileStoreRecoversOnTamperedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// Store initial credential.
	ref1, err := store.Store("account-1", "secret-1")
	if err != nil {
		t.Fatalf("initial Store failed: %v", err)
	}
	if ref1.Account != "account-1" {
		t.Fatalf("expected account-1, got %s", ref1.Account)
	}

	// Tamper with the encrypted file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Store another credential — should recover from tampered file.
	ref2, err := store.Store("account-2", "secret-2")
	if err != nil {
		t.Fatalf("Store after tamper should recover, got: %v", err)
	}
	if ref2.Account != "account-2" {
		t.Fatalf("expected account-2, got %s", ref2.Account)
	}

	// The new credential should be retrievable.
	got, err := store.Retrieve(&CredentialReference{Account: "account-2"})
	if err != nil {
		t.Fatalf("Retrieve new credential after recovery failed: %v", err)
	}
	if got != "secret-2" {
		t.Fatalf("expected secret-2, got %s", got)
	}

	// The old credential is lost (file was corrupted) — but that's expected.
	_, err = store.Retrieve(&CredentialReference{Account: "account-1"})
	if err == nil {
		t.Fatal("expected old credential to be lost after recovery, but it was found")
	}
}

func TestEncryptedFileDeleteRecoversOnTamperedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)

	// Store a credential.
	_, err := store.Store("account-1", "secret-1")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Tamper with the encrypted file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Delete should succeed even on tampered file.
	if err := store.Delete("account-1"); err != nil {
		t.Fatalf("Delete on tampered file should succeed, got: %v", err)
	}
}

func TestEncryptedFileStoreAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "credentials.json.enc")
	store := NewEncryptedFileStore(filePath)
	if !store.Available() {
		t.Fatal("expected Available() true for writable directory")
	}
}
