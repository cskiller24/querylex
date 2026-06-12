package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
)

// GenerateEncryptionData is the response data for the generate-encryption command.
type GenerateEncryptionData struct {
	KeyGenerated           bool `json:"key_generated"`
	CredentialsReencrypted bool `json:"credentials_reencrypted"`
}

// RunGenerateEncryption generates a new encryption key for the encrypted
// credential store. If existing encrypted credentials exist, they are
// re-encrypted with the new key. Requires interactive confirmation.
func RunGenerateEncryption() *format.Response[GenerateEncryptionData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[GenerateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")

	// Hard gate: prompt for confirmation.
	confirmed, err := PromptConfirm("This will generate a new encryption key for the encrypted credential store. If existing encrypted credentials exist, they will be re-encrypted. Continue?", false)
	if err != nil {
		return format.NewErrorResponse[GenerateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Confirmation required: %v", err),
			false,
			traceID,
		)
	}
	if !confirmed {
		resp := format.NewSuccessResponse(GenerateEncryptionData{
			KeyGenerated:           false,
			CredentialsReencrypted: false,
		}, traceID, nil)
		resp.Warnings = []format.Warning{
			{Code: "ENCRYPTION_KEY_NOT_GENERATED", Message: "Key generation cancelled by user."},
		}
		return resp
	}

	filePath := filepath.Join(querylexDir, "credentials.json.enc")
	store := credentials.NewEncryptedFileStore(filePath)

	if err := store.GenerateKey(); err != nil {
		return format.NewErrorResponse[GenerateEncryptionData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Failed to generate encryption key: %v", err),
			false,
			traceID,
		)
	}

	data := GenerateEncryptionData{
		KeyGenerated:           true,
		CredentialsReencrypted: true,
	}

	return format.NewSuccessResponse(data, traceID, nil)
}
