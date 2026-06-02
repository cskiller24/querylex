package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectCredentialStore_ReturnsStoreOnNormalSystem(t *testing.T) {
	store, err := SelectCredentialStore()
	if err != nil {
		t.Skipf("Skipping: no credential store available on this system: %v", err)
	}
	if store == nil {
		t.Fatal("SelectCredentialStore() returned nil store with nil error")
	}
}

func TestSelectCredentialStore_PriorityOrder(t *testing.T) {
	store, err := SelectCredentialStore()
	if err != nil {
		t.Skipf("Skipping: no credential store available: %v", err)
	}

	switch s := store.(type) {
	case *KeychainStore:
		if !s.Available() {
			t.Error("KeychainStore returned but Available() is false")
		}
	case *EncryptedFileStore:
		if NewKeychainStore().Available() {
			t.Error("EncryptedFileStore returned but KeychainStore was available (priority violation)")
		}
	case *EnvStore:
		if NewKeychainStore().Available() {
			t.Error("EnvStore returned but KeychainStore was available (priority violation)")
		}
		home, err := os.UserHomeDir()
		if err == nil {
			encStore := NewEncryptedFileStore(filepath.Join(home, ".querylex", "credentials.json.enc"))
			if encStore.Available() {
				t.Error("EnvStore returned but EncryptedFileStore was available (priority violation)")
			}
		}
	default:
		t.Errorf("unexpected store type: %T", store)
	}
}

func TestSelectCredentialStore_ErrorWhenNoBackend(t *testing.T) {
	cleanup := setEnvForTest("", "")
	defer cleanup()

	store, err := SelectCredentialStore()
	if store != nil {
		t.Skip("Skipping: at least one store was available even with env vars cleared")
	}
	if err == nil {
		t.Fatal("expected error when no backend available, got nil")
	}
	if !strings.Contains(err.Error(), "no credential store available") {
		t.Errorf("expected error containing 'no credential store available', got: %v", err)
	}
}

func TestSelectCredentialStore_ReturnsEnvStore(t *testing.T) {
	keychain := NewKeychainStore()
	if keychain.Available() {
		t.Skip("Skipping: keychain is available — cannot test env-only fallback")
	}

	// Check that encrypted file store is NOT available, otherwise EnvStore
	// won't be selected (encrypted file has higher priority).
	home, err := os.UserHomeDir()
	if err == nil {
		encStore := NewEncryptedFileStore(filepath.Join(home, ".querylex", "credentials.json.enc"))
		if encStore.Available() {
			t.Skip("Skipping: EncryptedFileStore is available — cannot test env-only fallback")
		}
	}

	cleanup := setEnvForTest("test-db-pass", "test-ai-key")
	defer cleanup()

	store, err := SelectCredentialStore()
	if err != nil {
		t.Fatalf("expected EnvStore, got error: %v", err)
	}

	if _, ok := store.(*EnvStore); !ok {
		t.Errorf("expected *EnvStore, got %T", store)
	}
}

func TestSelectCredentialStore_ReturnsEncryptedFileStore(t *testing.T) {
	keychain := NewKeychainStore()
	if keychain.Available() {
		t.Skip("Skipping: keychain is available — cannot test encrypted file fallback")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Skipping: cannot determine home directory")
	}
	encStore := NewEncryptedFileStore(filepath.Join(home, ".querylex", "credentials.json.enc"))
	if !encStore.Available() {
		t.Skip("Skipping: EncryptedFileStore is not available")
	}

	cleanup := setEnvForTest("", "")
	defer cleanup()

	store, err := SelectCredentialStore()
	if err != nil {
		t.Fatalf("expected EncryptedFileStore, got error: %v", err)
	}

	if _, ok := store.(*EncryptedFileStore); !ok {
		t.Errorf("expected *EncryptedFileStore, got %T", store)
	}
}

func setEnvForTest(dbPass, aiKey string) func() {
	origDB := os.Getenv("QUERYLEX_DB_PASSWORD")
	origAI := os.Getenv("QUERYLEX_AI_KEY")
	os.Setenv("QUERYLEX_DB_PASSWORD", dbPass)
	os.Setenv("QUERYLEX_AI_KEY", aiKey)
	return func() {
		if origDB != "" {
			os.Setenv("QUERYLEX_DB_PASSWORD", origDB)
		} else {
			os.Unsetenv("QUERYLEX_DB_PASSWORD")
		}
		if origAI != "" {
			os.Setenv("QUERYLEX_AI_KEY", origAI)
		} else {
			os.Unsetenv("QUERYLEX_AI_KEY")
		}
	}
}
