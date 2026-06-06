package credentials

import (
	"errors"
	"os"
)

// EnvStore implements CredentialStore by reading secrets from environment
// variables. This is a LAST RESORT fallback per D-03 — passwords stored in
// environment variables may leak via /proc on Linux.
//
// Supported environment variables:
//   - QUERYLEX_DB_PASSWORD: database password
//
// Store and Delete operations return errors because environment variables
// are read-only from the application's perspective.
type EnvStore struct{}

// NewEnvStore creates a new EnvStore.
func NewEnvStore() *EnvStore {
	return &EnvStore{}
}

const envVarDBPassword = "QUERYLEX_DB_PASSWORD"

// Store returns an error — environment variables cannot be set via this API.
func (e *EnvStore) Store(account string, secret string) (*CredentialReference, error) {
	return nil, errors.New("env store is read-only: cannot store credentials via this API")
}

// Retrieve reads a secret from the QUERYLEX_DB_PASSWORD environment variable.
// The account field in CredentialReference is informational — env vars
// contain raw secrets without account-level granularity.
func (e *EnvStore) Retrieve(ref *CredentialReference) (string, error) {
	switch ref.SecretKind {
	case "database-password", "":
		if val := os.Getenv(envVarDBPassword); val != "" {
			return val, nil
		}
	default:
		if val := os.Getenv(envVarDBPassword); val != "" {
			return val, nil
		}
	}

	if val := os.Getenv(envVarDBPassword); val != "" {
		return val, nil
	}

	return "", errors.New("env store: no secret found in environment variables")
}

// Delete returns an error — environment variables cannot be removed via this API.
func (e *EnvStore) Delete(account string) error {
	return errors.New("env store is read-only: cannot delete credentials via this API")
}

// Available returns true if QUERYLEX_DB_PASSWORD is set in the environment.
func (e *EnvStore) Available() bool {
	return os.Getenv(envVarDBPassword) != ""
}
