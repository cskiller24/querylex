package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/scrypt"
)

const (
	// scryptN is the CPU/memory cost parameter for scrypt key derivation
	// (N=32768, r=8, p=1 as specified in D-03).
	scryptN = 32768
	scryptR = 8
	scryptP = 1
	// keyLen is the derived key length in bytes (32 bytes = AES-256).
	keyLen = 32
	// saltLen is the length of the random salt prepended to the encrypted file.
	saltLen = 16
	// nonceLen is the GCM nonce length (12 bytes is standard for AES-GCM).
	nonceLen = 12
	// gcmTagLen is the GCM authentication tag length (16 bytes for AES-256-GCM).
	gcmTagLen = 16
	// headerOverhead is the total non-secret header: salt + nonce.
	headerOverhead = saltLen + nonceLen
)

var (
	// ErrCredentialsNotFound is returned when the requested credential
	// account is not found in the encrypted file.
	ErrCredentialsNotFound = errors.New("credential not found in encrypted store")

	// ErrTamperedFile is returned when the encrypted file has been tampered
	// with (GCM authentication tag validation fails).
	ErrTamperedFile = errors.New("encrypted credential file is corrupted or tampered")

	// ErrPassphraseRequired is returned when no passphrase has been set.
	ErrPassphraseRequired = errors.New("passphrase required for encrypted credential store")

	// ErrWrongPassphrase is returned when an incorrect passphrase is provided
	// to Unlock(), or when the encrypted file has been tampered with (GCM
	// authentication tag validation fails). The error message suggests using
	// QUERYLEX_DB_PASSWORD as an alternative.
	ErrWrongPassphrase = errors.New("wrong passphrase for encrypted credential store")
)

// EncryptedFileStore implements CredentialStore using an AES-256-GCM encrypted
// file at ~/.querylex/credentials.json.enc with scrypt key derivation.
//
// File format:
//
//	[16-byte salt][12-byte GCM nonce][encrypted data (JSON)][16-byte GCM tag]
//
// The encrypted data is a JSON-serialized map[string]string (account → secret).
// The key is derived from the user-provided passphrase using scrypt(N=32768,
// r=8, p=1). The derived key is cached in memory for the lifetime of the
// process to avoid repeated scrypt calls.
//
// Passphrase flow:
//   - Use Unlock(passphrase) to set and validate the passphrase. Unlock handles
//     first-use detection (no file exists), normal unlock (correct passphrase),
//     and wrong-passphrase/tampered-file errors.
//   - SetPassphrase() is deprecated — use Unlock() instead.
//   - The passphrase is held in memory for the process lifetime.
type EncryptedFileStore struct {
	mu         sync.RWMutex
	filePath   string
	passphrase string
	derivedKey []byte
}

// NewEncryptedFileStore creates a new EncryptedFileStore with the given
// file path. The passphrase must be set via SetPassphrase before use.
func NewEncryptedFileStore(filePath string) *EncryptedFileStore {
	if filePath == "" {
		home, _ := os.UserHomeDir()
		filePath = filepath.Join(home, ".querylex", "credentials.json.enc")
	}
	return &EncryptedFileStore{
		filePath: filePath,
	}
}

// Unlock attempts to unlock the encrypted store with the given passphrase.
// It handles three states:
//
//  1. First use (credentials.json.enc does not exist):
//     Sets the passphrase and returns nil. The store is then ready for Store()
//     calls, which will create the encrypted file on first write.
//
//  2. Normal unlock (file exists, decrypt succeeds):
//     Sets the passphrase, derives the key, and attempts to decrypt the
//     existing file. Returns nil on success.
//
//  3. Wrong passphrase or tampered file (file exists, decrypt fails):
//     Clears the passphrase from memory and returns ErrWrongPassphrase.
//
// The passphrase is held in memory for the process lifetime (not persisted
// to disk). Derived key caching avoids repeated scrypt calls.
func (e *EncryptedFileStore) Unlock(passphrase string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.passphrase = passphrase
	e.derivedKey = nil

	// readCredentials() handles three states:
	// - File doesn't exist → returns empty map, nil (first use)
	// - File exists, decrypt succeeds → returns credentials, nil
	// - File exists, decrypt fails (wrong passphrase/tampered) → ErrTamperedFile
	_, err := e.readCredentials()
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTamperedFile) {
		e.passphrase = ""
		return ErrWrongPassphrase
	}
	// Other errors (read failed, etc.)
	e.passphrase = ""
	return fmt.Errorf("encrypted store: %w", err)
}

// SetPassphrase sets the passphrase used for key derivation. Call this before
// any Store/Retrieve/Delete operations. The derived key is cached to avoid
// repeated scrypt calls within the same process lifetime.
//
// Deprecated: use Unlock() instead, which validates the passphrase against
// the existing encrypted file and handles first-use detection.
func (e *EncryptedFileStore) SetPassphrase(passphrase string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.passphrase = passphrase
	e.derivedKey = nil // invalidate cached key
}

// Store encrypts and stores a secret in the encrypted file.
// The credentials are stored as a map[account]secret serialized to JSON
// then encrypted with AES-256-GCM.
func (e *EncryptedFileStore) Store(account string, secret string) (*CredentialReference, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	creds, err := e.readCredentials()
	if err != nil {
		return nil, fmt.Errorf("encrypted store: %w", err)
	}

	creds[account] = secret

	if err := e.writeCredentials(creds); err != nil {
		return nil, fmt.Errorf("encrypted store: %w", err)
	}

	return &CredentialReference{
		Provider:   "encrypted-file",
		Service:    "querylex",
		Account:    account,
		SecretKind: "database-password",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// Retrieve decrypts and reads a secret from the encrypted file.
func (e *EncryptedFileStore) Retrieve(ref *CredentialReference) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	creds, err := e.readCredentials()
	if err != nil {
		return "", fmt.Errorf("encrypted retrieve: %w", err)
	}

	secret, ok := creds[ref.Account]
	if !ok {
		return "", fmt.Errorf("encrypted retrieve: %w", ErrCredentialsNotFound)
	}
	return secret, nil
}

// Delete removes a secret from the encrypted file.
func (e *EncryptedFileStore) Delete(account string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	creds, err := e.readCredentials()
	if err != nil {
		return fmt.Errorf("encrypted delete: %w", err)
	}

	delete(creds, account)

	if err := e.writeCredentials(creds); err != nil {
		return fmt.Errorf("encrypted delete: %w", err)
	}
	return nil
}

// Available checks whether the encrypted file store can be used.
// It checks that the directory $HOME/.querylex/ is writable.
func (e *EncryptedFileStore) Available() bool {
	dir := filepath.Dir(e.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	// Try creating a test file to verify write permission.
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// readCredentials reads, decrypts, and unmarshals the credentials map from
// the encrypted file. If the file doesn't exist, returns an empty map.
// The caller MUST hold at least a read lock.
func (e *EncryptedFileStore) readCredentials() (map[string]string, error) {
	if e.passphrase == "" {
		return nil, ErrPassphraseRequired
	}

	data, err := os.ReadFile(e.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// First use: return empty credentials map.
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("read encrypted file: %w", err)
	}

	if len(data) < headerOverhead {
		return nil, ErrTamperedFile
	}

	// Extract salt and nonce from the header.
	salt := data[:saltLen]
	nonce := data[saltLen : saltLen+nonceLen]
	ciphertext := data[saltLen+nonceLen:]

	key, err := e.getDerivedKey(salt)
	if err != nil {
		return nil, fmt.Errorf("key derivation: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	// Decrypt and authenticate. GCM will return an error if the ciphertext
	// has been tampered with (authentication tag mismatch).
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrTamperedFile
	}

	var creds map[string]string
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("decrypt: malformed credential data: %w", err)
	}

	return creds, nil
}

// writeCredentials encrypts and writes the credentials map to the encrypted
// file. The caller MUST hold a write lock.
func (e *EncryptedFileStore) writeCredentials(creds map[string]string) error {
	if e.passphrase == "" {
		return ErrPassphraseRequired
	}

	plaintext, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Generate random salt and nonce.
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	key, err := e.getDerivedKey(salt)
	if err != nil {
		return fmt.Errorf("key derivation: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}

	// Encrypt and authenticate. Seal appends the authentication tag.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Assemble: salt || nonce || ciphertext (which includes GCM tag).
	output := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	// Ensure the directory exists.
	dir := filepath.Dir(e.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}

	// Write atomically via temp file + rename.
	tmpPath := e.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, output, 0600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write encrypted file: %w", err)
	}

	if err := os.Rename(tmpPath, e.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename encrypted file: %w", err)
	}

	return nil
}

// getDerivedKey derives or returns the cached scrypt-derived key.
// The caller MUST NOT hold a write lock that would cause a deadlock with
// the SetPassphrase mutex usage.
func (e *EncryptedFileStore) getDerivedKey(salt []byte) ([]byte, error) {
	if e.derivedKey != nil && len(e.derivedKey) == keyLen {
		return e.derivedKey, nil
	}

	key, err := scrypt.Key([]byte(e.passphrase), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, err
	}

	e.derivedKey = key
	return key, nil
}
