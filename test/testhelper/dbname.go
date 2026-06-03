// Package testhelper provides reusable test utilities for QueryLex E2E tests.
package testhelper

import (
	"crypto/rand"
	"fmt"
)

// GenerateDBName generates a unique per-test database name with e2e_ prefix.
// Uses crypto/rand to produce 32 hex characters, resulting in names like
// e2e_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6.
func GenerateDBName() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return fmt.Sprintf("e2e_%x", buf)
}
