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

	// ErrCodeInvalidSQL indicates the provided SQL could not be validated.
	ErrCodeInvalidSQL ErrorCode = "INVALID_SQL"
	// ErrCodeUnsafeSQL indicates DML/DCL statements were rejected.
	ErrCodeUnsafeSQL ErrorCode = "UNSAFE_SQL"
	// ErrCodeTableNotFound indicates a referenced table does not exist.
	ErrCodeTableNotFound ErrorCode = "TABLE_NOT_FOUND"
	// ErrCodeColumnNotFound indicates a referenced column does not exist.
	ErrCodeColumnNotFound ErrorCode = "COLUMN_NOT_FOUND"
	// ErrCodeNoMatchingTables indicates resolve found no matches.
	ErrCodeNoMatchingTables ErrorCode = "NO_MATCHING_TABLES"
	// ErrCodeExplainFailed indicates explain plan extraction failed.
	ErrCodeExplainFailed ErrorCode = "EXPLAIN_FAILED"
	// ErrCodeJoinPathNotFound indicates no join path exists between tables.
	ErrCodeJoinPathNotFound ErrorCode = "JOIN_PATH_NOT_FOUND"
	// ErrCodeStatsUnavailable indicates table statistics are unavailable.
	ErrCodeStatsUnavailable ErrorCode = "STATS_UNAVAILABLE"
	// ErrCodeIndexAnalysisFailed indicates index extraction failed.
	ErrCodeIndexAnalysisFailed ErrorCode = "INDEX_ANALYSIS_FAILED"
	// ErrCodeSchemaParseError indicates schema data could not be parsed.
	ErrCodeSchemaParseError ErrorCode = "SCHEMA_PARSE_ERROR"
	// ErrCodeTerminologyParse indicates the terminologies.md YAML block could not be parsed.
	ErrCodeTerminologyParse ErrorCode = "TERMINOLOGY_PARSE_ERROR"

	// ErrCodeMemoryIndexStale indicates the memory index is out of sync with memory.sqlite.
	ErrCodeMemoryIndexStale ErrorCode = "MEMORY_INDEX_STALE"
	// ErrCodeMemoryStoreUnavailable indicates the memory subsystem is unavailable.
	ErrCodeMemoryStoreUnavailable ErrorCode = "MEMORY_STORE_UNAVAILABLE"
	// ErrCodeMemoryWriteFailed indicates a memory write operation failed.
	ErrCodeMemoryWriteFailed ErrorCode = "MEMORY_WRITE_FAILED"
	// ErrCodeExplainCacheStale indicates an explain cache entry is stale.
	ErrCodeExplainCacheStale ErrorCode = "EXPLAIN_CACHE_STALE"

	// ErrCodeAIServiceUnavailable indicates the AI service is unreachable.
	ErrCodeAIServiceUnavailable ErrorCode = "AI_SERVICE_UNAVAILABLE"
	// ErrCodeAIConfigMissing indicates AI provider is not configured.
	ErrCodeAIConfigMissing ErrorCode = "AI_CONFIG_MISSING"
	// ErrCodeAIGenerationFailed indicates AI failed to generate a valid response.
	ErrCodeAIGenerationFailed ErrorCode = "AI_GENERATION_FAILED"
	// ErrCodeAIContextOverflow indicates the context exceeds the model token limit.
	ErrCodeAIContextOverflow ErrorCode = "AI_CONTEXT_OVERFLOW"
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

	ErrCodeInvalidSQL:             "Provided SQL could not be validated against the database schema.",
	ErrCodeUnsafeSQL:              "DML/DCL statements (INSERT, UPDATE, DELETE, DROP, ALTER, etc.) are not permitted.",
	ErrCodeTableNotFound:          "Referenced table does not exist in the database schema.",
	ErrCodeColumnNotFound:         "Referenced column does not exist in the table.",
	ErrCodeNoMatchingTables:       "No matching tables found for the given input.",
	ErrCodeExplainFailed:          "Execution plan extraction failed.",
	ErrCodeJoinPathNotFound:       "No join path exists between the specified tables.",
	ErrCodeStatsUnavailable:       "Table statistics are unavailable for the specified tables.",
	ErrCodeIndexAnalysisFailed:    "Index metadata extraction failed.",
	ErrCodeSchemaParseError:       "Schema data could not be parsed or is in an unexpected format.",
	ErrCodeTerminologyParse:       "The terminologies.md file contains a malformed or missing querylex-terms YAML block.",

	ErrCodeMemoryIndexStale:        "Memory index metadata is stale relative to memory.sqlite.",
	ErrCodeMemoryStoreUnavailable:  "Memory subsystem is unavailable.",
	ErrCodeMemoryWriteFailed:       "Unable to write memory entry.",
	ErrCodeExplainCacheStale:       "Explain cache entry is stale — fingerprint mismatch or TTL expired.",

	ErrCodeAIServiceUnavailable:    "AI service is unreachable or not configured.",
	ErrCodeAIConfigMissing:         "AI provider not configured. Run 'querylex ai-config' to set up.",
	ErrCodeAIGenerationFailed:      "AI failed to generate a valid response.",
	ErrCodeAIContextOverflow:       "Context exceeds model token limit. Try narrowing the query scope.",
}
