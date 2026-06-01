// Package credentials provides a secure credential storage abstraction
// with OS keychain, encrypted file, and environment variable backends.
package credentials

import "encoding/json"

// StoreType represents the type of credential store.
type StoreType int

const (
	// OSKeychain uses the platform-native keychain (macOS Keychain, Windows
	// Credential Manager, Linux Secret Service via D-Bus).
	OSKeychain StoreType = iota
	// EncryptedFile uses an AES-256-GCM encrypted file at ~/.querylex/credentials.json.enc
	// with scrypt key derivation. Used as fallback on headless Linux.
	EncryptedFile
	// EnvVar reads credentials from environment variables.
	EnvVar
)

// CredentialReference identifies a secret in the credential store.
// database.json stores this (not the password) to ensure zero secrets in JSON.
type CredentialReference struct {
	Provider   string `json:"provider"`
	Service    string `json:"service"`
	Account    string `json:"account"`
	SecretKind string `json:"secret_kind"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// CredentialStore is the interface for platform-specific credential storage.
// All implementations must ensure that secrets never leak to disk in
// plaintext form.
type CredentialStore interface {
	// Store saves a secret and returns the associated CredentialReference.
	Store(account string, secret string) (*CredentialReference, error)

	// Retrieve gets a secret by its CredentialReference.
	Retrieve(ref *CredentialReference) (string, error)

	// Delete removes a secret by account.
	Delete(account string) error

	// Available returns true if this store is usable on the current platform.
	Available() bool
}

// MarshalCredentialReferenceJSON is a helper for serializing credential
// references for storage in database.json.
func MarshalCredentialReferenceJSON(ref *CredentialReference) ([]byte, error) {
	return json.Marshal(ref)
}

// UnmarshalCredentialReferenceJSON deserializes a credential reference
// from its JSON representation.
func UnmarshalCredentialReferenceJSON(data []byte) (*CredentialReference, error) {
	var ref CredentialReference
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, err
	}
	return &ref, nil
}
