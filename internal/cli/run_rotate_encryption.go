package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
)

// RotateEncryptionData is the response data for the rotate-encryption command.
type RotateEncryptionData struct {
	KeyRotated bool `json:"key_rotated"`
}

// RunRotateEncryption generates a new encryption key and re-encrypts all
// credentials in the encrypted store. The old key is replaced. Requires
// interactive confirmation.
func RunRotateEncryption() *format.Response[RotateEncryptionData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[RotateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")

	// Hard gate: prompt for confirmation.
	confirmed, err := PromptConfirm("This will generate a new encryption key and re-encrypt all credentials in the encrypted store. The old key will be replaced. This cannot be undone. Continue?", false)
	if err != nil {
		return format.NewErrorResponse[RotateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Confirmation required: %v", err),
			false,
			traceID,
		)
	}
	if !confirmed {
		resp := format.NewSuccessResponse(RotateEncryptionData{
			KeyRotated: false,
		}, traceID, nil)
		resp.Warnings = []format.Warning{
			{Code: "ENCRYPTION_KEY_NOT_ROTATED", Message: "Key rotation cancelled by user."},
		}
		return resp
	}

	filePath := filepath.Join(querylexDir, "credentials.json.enc")
	store := credentials.NewEncryptedFileStore(filePath)

	if err := store.RotateKey(); err != nil {
		return format.NewErrorResponse[RotateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to rotate encryption key: %v", err),
			false,
			traceID,
		)
	}

	data := RotateEncryptionData{
		KeyRotated: true,
	}

	return format.NewSuccessResponse(data, traceID, nil)
}
