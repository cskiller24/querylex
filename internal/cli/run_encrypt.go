package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/format"
)

// EncryptData is the response data for the encrypt command.
type EncryptData struct {
	KeyGenerated bool `json:"key_generated"`
	KeyRotated   bool `json:"key_rotated"`
}

// RunEncrypt generates or rotates the encryption key for the encrypted
// credential store. If rotate is true, the key is rotated (re-encrypt all
// credentials with a fresh key). Otherwise, a new key is generated.
// If force is false, an interactive confirmation prompt is displayed.
func RunEncrypt(rotate bool, force bool) *format.Response[EncryptData] {
	traceID := uuid.New().String()

	home, err := os.UserHomeDir()
	if err != nil {
		return format.NewErrorResponse[EncryptData](
			format.ErrCodeInternalError,
			fmt.Sprintf("Cannot determine home directory: %v", err),
			false,
			traceID,
		)
	}

	querylexDir := filepath.Join(home, ".querylex")
	filePath := filepath.Join(querylexDir, "credentials.json.enc")

	if !force {
		var message string
		if rotate {
			message = "This will generate a new encryption key and re-encrypt all credentials in the encrypted store. The old key will be replaced. This cannot be undone. Continue?"
		} else {
			message = "This will generate a new encryption key for the encrypted credential store. If existing encrypted credentials exist, they will be re-encrypted. Continue?"
		}

		confirmed, err := PromptConfirm(message, false)
		if err != nil {
			return format.NewErrorResponse[EncryptData](
				format.ErrCodeInvalidArgument,
				fmt.Sprintf("Failed to get confirmation: %v", err),
				false,
				traceID,
			)
		}
		if !confirmed {
			resp := format.NewSuccessResponse(EncryptData{
				KeyGenerated: false,
				KeyRotated:   false,
			}, traceID, nil)
			warningCode := "ENCRYPTION_CANCELLED"
			if rotate {
				warningCode = "ENCRYPTION_ROTATION_CANCELLED"
			}
			resp.Warnings = []format.Warning{
				{Code: warningCode, Message: "Operation cancelled by user. No changes made."},
			}
			return resp
		}
	}

	store := credentials.NewEncryptedFileStore(filePath)

	if rotate {
		if err := store.RotateKey(); err != nil {
			return format.NewErrorResponse[EncryptData](
				format.ErrCodeInternalError,
				fmt.Sprintf("Failed to rotate encryption key: %v", err),
				false,
				traceID,
			)
		}
	} else {
		if err := store.GenerateKey(); err != nil {
			return format.NewErrorResponse[EncryptData](
				format.ErrCodeInternalError,
				fmt.Sprintf("Failed to generate encryption key: %v", err),
				false,
				traceID,
			)
		}
	}

	data := EncryptData{
		KeyGenerated: !rotate,
		KeyRotated:   rotate,
	}

	return format.NewSuccessResponse(data, traceID, nil)
}

// RenderEncryptHuman renders the encrypt command output as a human-readable message.
func RenderEncryptHuman(w io.Writer, data EncryptData) {
	if data.KeyRotated {
		fmt.Fprintln(w, "Encryption key rotated successfully. All credentials re-encrypted.")
	} else if data.KeyGenerated {
		fmt.Fprintln(w, "Encryption key generated successfully.")
	} else {
		fmt.Fprintln(w, "Operation cancelled. No changes made.")
	}
}
