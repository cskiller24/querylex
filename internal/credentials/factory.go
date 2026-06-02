package credentials

import (
	"fmt"
	"os"
	"path/filepath"
)

// SelectCredentialStore returns the best available credential store for the
// current platform, following priority: OS Keychain → Encrypted File → EnvVar.
//
// Priority order:
//  1. OS Keychain (macOS Keychain, Windows Credential Manager, Linux Secret
//     Service via D-Bus). Detects headless Linux via Available() returning false
//     when D-Bus Secret Service is unavailable.
//  2. Encrypted File (AES-256-GCM encrypted file at ~/.querylex/credentials.json.enc
//     with scrypt key derivation). Universal fallback — works on all platforms
//     when the filesystem is writable.
//  3. Environment Variables (QUERYLEX_DB_PASSWORD, QUERYLEX_AI_KEY). Last resort
//     for CI/non-interactive environments.
//
// Returns an error only when no credential backend is available AND no
// environment variable fallback is set. On a system with a writable home
// directory, the EncryptedFileStore should always be available, so this
// error path is primarily for constrained environments.
func SelectCredentialStore() (CredentialStore, error) {
	// 1. OS Keychain (macOS Keychain, Windows Credential Manager, Linux libsecret)
	keychain := NewKeychainStore()
	if keychain.Available() {
		return keychain, nil
	}

	// 2. Encrypted File (AES-256-GCM, universal fallback)
	home, err := os.UserHomeDir()
	if err == nil {
		encFile := filepath.Join(home, ".querylex", "credentials.json.enc")
		encStore := NewEncryptedFileStore(encFile)
		if encStore.Available() {
			return encStore, nil
		}
	}

	// 3. Environment Variables (last resort)
	envStore := NewEnvStore()
	if envStore.Available() {
		return envStore, nil
	}

	return nil, fmt.Errorf("no credential store available")
}
