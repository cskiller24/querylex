package format

import (
	"testing"
)

func TestNewErrorCodesExist(t *testing.T) {
	// Verify that all 10 new error codes from Phase 2 are defined.
	newCodes := []ErrorCode{
		ErrCodeInvalidSQL,
		ErrCodeUnsafeSQL,
		ErrCodeTableNotFound,
		ErrCodeColumnNotFound,
		ErrCodeNoMatchingTables,
		ErrCodeExplainFailed,
		ErrCodeJoinPathNotFound,
		ErrCodeStatsUnavailable,
		ErrCodeIndexAnalysisFailed,
		ErrCodeSchemaParseError,
	}

	for _, code := range newCodes {
		if code == "" {
			t.Error("found empty ErrorCode constant among new codes")
		}
		desc, ok := ErrorCodeDescriptions[code]
		if !ok {
			t.Errorf("ErrorCode %s has no description in ErrorCodeDescriptions", code)
		}
		if desc == "" {
			t.Errorf("ErrorCode %s has empty description", code)
		}
	}
}

func TestAllErrorCodesMapped(t *testing.T) {
	// Every defined ErrorCode constant should have a description.
	// This test catches new codes added without updating ErrorCodeDescriptions.
	allCodes := []ErrorCode{
		ErrCodeInvalidArgument,
		ErrCodeConnectionFailed,
		ErrCodeCredentialUnavailable,
		ErrCodeWorkspaceStateInvalid,
		ErrCodeLockAcquisitionTimeout,
		ErrCodeUnsupportedDatabase,
		ErrCodePermissionDenied,
		ErrCodeInternalError,
		ErrCodeInvalidSQL,
		ErrCodeUnsafeSQL,
		ErrCodeTableNotFound,
		ErrCodeColumnNotFound,
		ErrCodeNoMatchingTables,
		ErrCodeExplainFailed,
		ErrCodeJoinPathNotFound,
		ErrCodeStatsUnavailable,
		ErrCodeIndexAnalysisFailed,
		ErrCodeSchemaParseError,
	}

	for _, code := range allCodes {
		if _, ok := ErrorCodeDescriptions[code]; !ok {
			t.Errorf("ErrorCode %s is missing from ErrorCodeDescriptions", code)
		}
	}

	// Verify count
	if len(allCodes) != 18 {
		t.Fatalf("expected 18 total error codes, got %d", len(allCodes))
	}
}
