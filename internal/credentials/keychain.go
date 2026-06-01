package credentials

import (
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const keychainService = "querylex"

// KeychainStore implements CredentialStore using the platform-native OS
// keychain via the zalando/go-keyring library.
//
// Supported platforms:
//   - macOS: Keychain Services
//   - Windows: Credential Manager
//   - Linux: Secret Service (libsecret) via D-Bus
//
// On headless Linux without D-Bus Secret Service, Available() returns false
// and the caller should fall back to EncryptedFileStore.
type KeychainStore struct{}

// NewKeychainStore creates a new KeychainStore.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

// Store saves a secret in the OS keychain.
func (k *KeychainStore) Store(account string, secret string) (*CredentialReference, error) {
	if err := keyring.Set(keychainService, account, secret); err != nil {
		return nil, fmt.Errorf("keychain store: %w", err)
	}

	return &CredentialReference{
		Provider:   "os-keychain",
		Service:    keychainService,
		Account:    account,
		SecretKind: "database-password",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Retrieve gets a secret from the OS keychain by its credential reference.
func (k *KeychainStore) Retrieve(ref *CredentialReference) (string, error) {
	secret, err := keyring.Get(ref.Service, ref.Account)
	if err != nil {
		return "", fmt.Errorf("keychain retrieve: %w", err)
	}
	return secret, nil
}

// Delete removes a secret from the OS keychain.
func (k *KeychainStore) Delete(account string) error {
	if err := keyring.Delete(keychainService, account); err != nil {
		return fmt.Errorf("keychain delete: %w", err)
	}
	return nil
}

// Available checks whether the OS keychain is usable on the current platform.
// It attempts a Get operation with a test key; if the keychain is functional
// the call returns keyring.ErrNotFound (test key doesn't exist). If it returns
// a platform-not-available error, Available returns false.
func (k *KeychainStore) Available() bool {
	_, err := keyring.Get(keychainService, "__querylex_availability_check__")
	if err == nil {
		// Unexpected: test key exists. Keychain is still usable.
		return true
	}
	if err == keyring.ErrNotFound {
		// Expected: test key doesn't exist, but keychain is functional.
		return true
	}
	// Some other error — keychain likely unavailable (e.g., no D-Bus Secret
	// Service on headless Linux, or no keychain daemon running).
	return false
}
