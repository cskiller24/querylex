package format

// ErrorCode is a stable error code string type used in response envelopes.
type ErrorCode string

const (
	// ErrCodeInvalidArgument indicates invalid command arguments.
	ErrCodeInvalidArgument ErrorCode = "INVALID_ARGUMENT"
	// ErrCodeConnectionFailed indicates a database connection failure.
	ErrCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"
	// ErrCodeCredentialUnavailable indicates a credential could not be retrieved.
	ErrCodeCredentialUnavailable ErrorCode = "CREDENTIAL_UNAVAILABLE"
	// ErrCodeWorkspaceStateInvalid indicates workspace state is missing or malformed.
	ErrCodeWorkspaceStateInvalid ErrorCode = "WORKSPACE_STATE_INVALID"
	// ErrCodeLockAcquisitionTimeout indicates a file lock could not be acquired.
	ErrCodeLockAcquisitionTimeout ErrorCode = "LOCK_ACQUISITION_TIMEOUT"
	// ErrCodeUnsupportedDatabase indicates the database type is not supported.
	ErrCodeUnsupportedDatabase ErrorCode = "UNSUPPORTED_DATABASE"
	// ErrCodePermissionDenied indicates the credentials lack required permissions.
	ErrCodePermissionDenied ErrorCode = "PERMISSION_DENIED"
	// ErrCodeInternalError indicates an unexpected failure.
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// ErrorCodeDescriptions maps each ErrorCode to a human-readable description.
var ErrorCodeDescriptions = map[ErrorCode]string{
	ErrCodeInvalidArgument:        "Invalid command arguments.",
	ErrCodeConnectionFailed:       "Database connection failed.",
	ErrCodeCredentialUnavailable:  "Credential could not be retrieved from the keychain.",
	ErrCodeWorkspaceStateInvalid:  "Querylex workspace state is missing, malformed, or internally inconsistent.",
	ErrCodeLockAcquisitionTimeout: "A file lock could not be acquired within the timeout period.",
	ErrCodeUnsupportedDatabase:    "Database type is not supported.",
	ErrCodePermissionDenied:       "Active credentials lack required permissions.",
	ErrCodeInternalError:          "An unexpected internal error occurred.",
}
