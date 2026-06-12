package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
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
	// encryptionKeyFile is the hex-encoded key file stored in the same directory
	// as the encrypted credentials file.
	encryptionKeyFile = ".encryption_key"
)

var (
	// ErrCredentialsNotFound is returned when the requested credential
	// account is not found in the encrypted file.
	ErrCredentialsNotFound = errors.New("credential not found in encrypted store")

	// ErrTamperedFile is returned when the encrypted file has been tampered
	// with (GCM authentication tag validation fails).
	ErrTamperedFile = errors.New("encrypted credential file is corrupted or tampered")
)

// EncryptedFileStore implements CredentialStore using an AES-256-GCM encrypted
// file at ~/.querylex/credentials.json.enc.
//
// The encryption key is a randomly generated AES-256 key (32 bytes) stored as
// a hex-encoded string in ~/.querylex/.encryption_key. On first use, the key
// is auto-generated and written with 0600 permissions. If an existing encrypted
// credentials file was encrypted with the legacy machine-ID-derived key, it is
// transparently migrated on read: the old key is used to decrypt, a new
// generated key is stored, and credentials are re-encrypted.
//
// File format:
//
//	[16-byte salt][12-byte GCM nonce][encrypted data (JSON)][16-byte GCM tag]
//
// The encrypted data is a JSON-serialized map[string]string (account -> secret).
type EncryptedFileStore struct {
	mu            sync.RWMutex
	filePath      string
	encryptionKey []byte
}

// NewEncryptedFileStore creates a new EncryptedFileStore with the given
// file path.
func NewEncryptedFileStore(filePath string) *EncryptedFileStore {
	if filePath == "" {
		home, _ := os.UserHomeDir()
		filePath = filepath.Join(home, ".querylex", "credentials.json.enc")
	}
	return &EncryptedFileStore{
		filePath: filePath,
	}
}

// Store encrypts and stores a secret in the encrypted file.
// The credentials are stored as a map[account]secret serialized to JSON
// then encrypted with AES-256-GCM.
func (e *EncryptedFileStore) Store(account string, secret string) (*CredentialReference, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	creds, err := e.readCredentials()
	if err != nil {
		if errors.Is(err, ErrTamperedFile) {
			// The file is corrupted or the encryption key has changed since
			// the credentials were written. Start fresh — the old data is
			// unrecoverable regardless.
			creds = make(map[string]string)
		} else {
			return nil, fmt.Errorf("encrypted store: %w", err)
		}
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
		if errors.Is(err, ErrTamperedFile) {
			// File is unreadable — treat as empty so delete succeeds
			// (the credential is already gone).
			return nil
		}
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

// GenerateKey generates a fresh random AES-256 encryption key and stores it.
// If existing encrypted credentials exist, they are re-encrypted with the new key.
func (e *EncryptedFileStore) GenerateKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Read existing credentials (may be empty).
	creds, err := e.readCredentials()
	if err != nil {
		if errors.Is(err, ErrTamperedFile) {
			creds = make(map[string]string)
		} else {
			return fmt.Errorf("generate key: %w", err)
		}
	}

	// Generate a fresh random 32-byte key.
	newKey := make([]byte, keyLen)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Write hex-encoded key to key file.
	keyPath := filepath.Join(filepath.Dir(e.filePath), encryptionKeyFile)
	hexKey := hex.EncodeToString(newKey)
	if err := os.WriteFile(keyPath, []byte(hexKey), 0600); err != nil {
		return fmt.Errorf("write encryption key: %w", err)
	}

	// Cache the new key.
	e.encryptionKey = newKey

	// Re-encrypt credentials with the new key (if any).
	if len(creds) > 0 {
		if err := e.writeCredentials(creds); err != nil {
			return fmt.Errorf("re-encrypt credentials: %w", err)
		}
	}

	return nil
}

// RotateKey generates a fresh random AES-256 encryption key and re-encrypts
// all existing credentials with it. The old key is replaced.
func (e *EncryptedFileStore) RotateKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Read existing credentials.
	creds, err := e.readCredentials()
	if err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}

	// Generate a fresh random 32-byte key.
	newKey := make([]byte, keyLen)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}

	// Write hex-encoded key to key file.
	keyPath := filepath.Join(filepath.Dir(e.filePath), encryptionKeyFile)
	hexKey := hex.EncodeToString(newKey)
	if err := os.WriteFile(keyPath, []byte(hexKey), 0600); err != nil {
		return fmt.Errorf("write encryption key: %w", err)
	}

	// Cache the new key.
	e.encryptionKey = newKey

	// Re-encrypt credentials with the new key.
	if err := e.writeCredentials(creds); err != nil {
		return fmt.Errorf("re-encrypt credentials: %w", err)
	}

	return nil
}

// readCredentials reads, decrypts, and unmarshals the credentials map from
// the encrypted file. If the file doesn't exist, returns an empty map.
// The caller MUST hold at least a read lock.
func (e *EncryptedFileStore) readCredentials() (map[string]string, error) {
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

	key, err := e.getEncryptionKey(salt)
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

	key, err := e.getEncryptionKey(salt)
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

// getEncryptionKey returns the encryption key to use for encrypting/decrypting
// credentials. It resolves the key using the following priority:
//
//  1. Return cached e.encryptionKey if set.
//  2. Read the hex-encoded key from ~/.querylex/.encryption_key. If present,
//     decode hex to 32 bytes, cache it, and return it.
//  3. If the key file is absent AND the credentials file exists: try legacy
//     machine-ID-derived key for backward compatibility. If successful,
//     auto-migrate: generate a new random key, store it, re-encrypt, and
//     return the new key.
//  4. If the key file is absent AND no credentials file: generate a random
//     32-byte key, write it hex-encoded to the key file, cache it, return it.
func (e *EncryptedFileStore) getEncryptionKey(salt []byte) ([]byte, error) {
	// 1. Return cached key if set.
	if e.encryptionKey != nil && len(e.encryptionKey) == keyLen {
		return e.encryptionKey, nil
	}

	keyPath := filepath.Join(filepath.Dir(e.filePath), encryptionKeyFile)

	// 2. Read the key file if it exists.
	if keyData, err := os.ReadFile(keyPath); err == nil {
		key, hexErr := hex.DecodeString(string(keyData))
		if hexErr != nil || len(key) != keyLen {
			return nil, fmt.Errorf("encryption key file corrupted: delete %s and run generate-encryption", keyPath)
		}
		e.encryptionKey = key
		return key, nil
	}

	// 3. Key file absent — check if credentials file exists for legacy
	//    backward-compatible migration.
	_, statErr := os.Stat(e.filePath)
	if statErr == nil {
		// Try decrypting the existing file with the legacy key to verify.
		data, readErr := os.ReadFile(e.filePath)
		if readErr != nil {
			return nil, fmt.Errorf("read encrypted file for migration: %w", readErr)
		}
		if len(data) >= headerOverhead {
			fileSalt := data[:saltLen]
			fileNonce := data[saltLen : saltLen+nonceLen]
			fileCiphertext := data[saltLen+nonceLen:]

			// Use the specific salt from the file for key derivation.
			fileKey, derErr := deriveMachineKey(fileSalt)
			if derErr != nil {
				return nil, fmt.Errorf("legacy machine key: %w", derErr)
			}

			block, blkErr := aes.NewCipher(fileKey)
			if blkErr != nil {
				return nil, fmt.Errorf("aes cipher (legacy): %w", blkErr)
			}
			gcmBlock, gcmErr := cipher.NewGCM(block)
			if gcmErr != nil {
				return nil, fmt.Errorf("gcm (legacy): %w", gcmErr)
			}

			plaintext, gcmOpenErr := gcmBlock.Open(nil, fileNonce, fileCiphertext, nil)
			if gcmOpenErr != nil {
				// Legacy key failed — file may have been encrypted with a
				// different key. Return ErrTamperedFile so callers can handle.
				return nil, ErrTamperedFile
			}

			var creds map[string]string
			if jsonErr := json.Unmarshal(plaintext, &creds); jsonErr != nil {
				return nil, ErrTamperedFile
			}

			// Legacy decryption succeeded — auto-migrate to stored key.
			fmt.Fprintf(os.Stderr, "Warning: Migrated credentials to stored encryption key. Old machine-ID-derived key is no longer used.\n")

			// Generate a new random key.
			newKey := make([]byte, keyLen)
			if _, randErr := rand.Read(newKey); randErr != nil {
				return nil, fmt.Errorf("generate new key for migration: %w", randErr)
			}

			// Store the new key.
			hexKey := hex.EncodeToString(newKey)
			if writeErr := os.WriteFile(keyPath, []byte(hexKey), 0600); writeErr != nil {
				return nil, fmt.Errorf("write encryption key for migration: %w", writeErr)
			}

			e.encryptionKey = newKey

			// Re-encrypt credentials with the new key.
			if writeErr := e.writeCredentials(creds); writeErr != nil {
				return nil, fmt.Errorf("re-encrypt credentials for migration: %w", writeErr)
			}

			return newKey, nil
		}
	}

	// 4. Key file absent AND no credentials file (or credentials file too
	//    short) — generate a fresh key.
	newKey := make([]byte, keyLen)
	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}

	hexKey := hex.EncodeToString(newKey)
	if err := os.WriteFile(keyPath, []byte(hexKey), 0600); err != nil {
		return nil, fmt.Errorf("write encryption key: %w", err)
	}

	e.encryptionKey = newKey
	return newKey, nil
}
