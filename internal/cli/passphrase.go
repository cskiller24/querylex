package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/querylex/querylex/internal/credentials"
	"golang.org/x/term"
)

const (
	envKeychainPassphrase = "QUERYLEX_KEYCHAIN_PASSPHRASE"
)

// promptEncryptedFilePassphrase prompts the user for the encrypted credential
// store passphrase, or reads it from the QUERYLEX_KEYCHAIN_PASSPHRASE environment
// variable for CI/non-interactive use.
//
// kind is "database" or "ai" — determines which alternative environment variable
// to mention in error messages (QUERYLEX_DB_PASSWORD or QUERYLEX_AI_API_KEY).
//
// The flow is:
//  1. If QUERYLEX_KEYCHAIN_PASSPHRASE is set → use it directly
//  2. If running in a terminal → prompt interactively via survey.Password
//  3. If not a terminal and no env var → return clear error with alternatives
func promptEncryptedFilePassphrase(store *credentials.EncryptedFileStore, kind string) error {
	// Determine the alternative env var name for error messages
	altEnvVar := "QUERYLEX_DB_PASSWORD"
	if kind == "ai" {
		altEnvVar = "QUERYLEX_AI_API_KEY"
	}

	// 1. CI/non-interactive path: env var
	if pass := os.Getenv(envKeychainPassphrase); pass != "" {
		if err := store.Unlock(pass); err != nil {
			if errors.Is(err, credentials.ErrWrongPassphrase) {
				return fmt.Errorf("wrong passphrase from %s. Set %s as an alternative.", envKeychainPassphrase, altEnvVar)
			}
			return fmt.Errorf("%s: %w", envKeychainPassphrase, err)
		}
		return nil
	}

	// 2. Interactive path: survey password prompt
	if term.IsTerminal(int(os.Stdin.Fd())) {
		passphrase := ""
		prompt := &survey.Password{
			Message: "Enter master passphrase for encrypted credentials:",
		}
		if err := survey.AskOne(prompt, &passphrase); err != nil {
			return fmt.Errorf("passphrase prompt cancelled: %w", err)
		}

		if err := store.Unlock(passphrase); err != nil {
			if errors.Is(err, credentials.ErrWrongPassphrase) {
				fmt.Fprintln(os.Stderr, "Wrong passphrase. You can use", altEnvVar, "as an alternative.")
				return err
			}
			return fmt.Errorf("unlock encrypted store: %w", err)
		}
		return nil
	}

	// 3. No terminal and no env var
	return fmt.Errorf(
		"no terminal available and %s not set. Use %s env var or set %s.",
		envKeychainPassphrase, envKeychainPassphrase, altEnvVar,
	)
}
