package format

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewSuccessResponse_JSON(t *testing.T) {
	traceID := "test-trace-id"
	activeDB := "test-db"
	resp := NewSuccessResponse(map[string]string{"hello": "world"}, traceID, &activeDB)
	resp.Complete(time.Now().Add(-5 * time.Millisecond))

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check success flag
	var success bool
	if err := json.Unmarshal(parsed["success"], &success); err != nil {
		t.Fatal("missing success field")
	}
	if !success {
		t.Fatal("expected success to be true")
	}

	// Check data field
	var dataParsed map[string]string
	if err := json.Unmarshal(parsed["data"], &dataParsed); err != nil {
		t.Fatal("missing or invalid data field")
	}
	if dataParsed["hello"] != "world" {
		t.Fatalf("expected hello=world, got %v", dataParsed)
	}

	// Check meta field
	var meta ResponseMeta
	if err := json.Unmarshal(parsed["meta"], &meta); err != nil {
		t.Fatal("missing or invalid meta field")
	}
	if meta.TraceID != traceID {
		t.Fatalf("expected trace_id=%s, got %s", traceID, meta.TraceID)
	}
	if meta.ProtocolVersion != "1.0.0" {
		t.Fatalf("expected protocol_version=1.0.0, got %s", meta.ProtocolVersion)
	}
	if meta.ActiveDatabaseID == nil || *meta.ActiveDatabaseID != activeDB {
		t.Fatalf("expected active_database_id=%s", activeDB)
	}
	if meta.DurationMs <= 0 {
		t.Fatalf("expected duration_ms > 0, got %d", meta.DurationMs)
	}

	// Check error is absent
	if parsed["error"] != nil {
		// The key may be omitted via omitempty, so it should be missing from parsed
		var hasError bool
		if raw, ok := parsed["error"]; ok && string(raw) != "null" {
			hasError = true
		}
		if hasError {
			t.Fatal("expected no error field in success response")
		}
	}
}

func TestNewSuccessResponse_NilActiveDB(t *testing.T) {
	traceID := "test-trace-id"
	resp := NewSuccessResponse("data", traceID, nil)

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	var meta ResponseMeta
	if err := json.Unmarshal(parsed["meta"], &meta); err != nil {
		t.Fatal("missing meta field")
	}
	if meta.ActiveDatabaseID != nil {
		t.Fatal("expected active_database_id to be null")
	}
}

func TestNewErrorResponse_JSON(t *testing.T) {
	traceID := "error-trace-id"
	resp := NewErrorResponse[string](ErrCodeConnectionFailed, "connection to database failed", true, traceID)
	resp.Complete(time.Now().Add(-2 * time.Millisecond))

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Check success flag
	var success bool
	if err := json.Unmarshal(parsed["success"], &success); err != nil {
		t.Fatal("missing success field")
	}
	if success {
		t.Fatal("expected success to be false")
	}

	// Check error field
	var errDetail ErrorDetail
	if err := json.Unmarshal(parsed["error"], &errDetail); err != nil {
		t.Fatal("missing or invalid error field")
	}
	if errDetail.Code != ErrCodeConnectionFailed {
		t.Fatalf("expected code=CONNECTION_FAILED, got %s", errDetail.Code)
	}
	if errDetail.Message != "connection to database failed" {
		t.Fatalf("unexpected message: %s", errDetail.Message)
	}
	if !errDetail.Retryable {
		t.Fatal("expected retryable to be true")
	}

	// Check data is absent
	if raw, ok := parsed["data"]; ok && string(raw) != "null" {
		t.Fatal("expected data to be absent in error response")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	codes := []ErrorCode{
		ErrCodeInvalidArgument,
		ErrCodeConnectionFailed,
		ErrCodeCredentialUnavailable,
		ErrCodeWorkspaceStateInvalid,
		ErrCodeLockAcquisitionTimeout,
		ErrCodeUnsupportedDatabase,
		ErrCodePermissionDenied,
		ErrCodeInternalError,
	}

	for _, code := range codes {
		if code == "" {
			t.Error("found empty ErrorCode constant")
		}
	}

	if len(codes) < 8 {
		t.Fatalf("expected at least 8 error codes, got %d", len(codes))
	}
}

func TestErrorCodeSerialization(t *testing.T) {
	errDetail := ErrorDetail{
		Code:      ErrCodePermissionDenied,
		Message:   "access denied",
		Retryable: false,
	}

	data, err := json.Marshal(errDetail)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ErrorDetail
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Code != ErrCodePermissionDenied {
		t.Fatalf("expected PERMISSION_DENIED, got %s", parsed.Code)
	}
}

func TestResponse_Warnings(t *testing.T) {
	traceID := "warn-trace-id"
	resp := NewSuccessResponse("data", traceID, nil)
	resp.Warnings = []Warning{
		{
			Code:    "TEST_WARNING",
			Message: "this is a warning",
			Details: map[string]string{"key": "value"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	var warnings []Warning
	if err := json.Unmarshal(parsed["warnings"], &warnings); err != nil {
		t.Fatal("missing or invalid warnings field")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Code != "TEST_WARNING" {
		t.Fatalf("expected TEST_WARNING, got %s", warnings[0].Code)
	}
}

func TestGenerateTraceID(t *testing.T) {
	id := GenerateTraceID()
	if id == "" {
		t.Fatal("expected non-empty trace ID")
	}
	// UUID v4 format: 8-4-4-4-12 hex digits
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %d", len(id))
	}
}

func TestComplete_SetsDuration(t *testing.T) {
	start := time.Now()
	resp := NewSuccessResponse("test", "tid", nil)
	resp.Complete(start)

	if resp.Meta.DurationMs < 0 {
		t.Fatal("expected non-negative duration")
	}
}
