package format

import (
	"time"

	"github.com/google/uuid"
)

// ErrorCode is a stable error code string type used in response envelopes.
// (Re-exported from error.go for convenience.)

// Warning represents a non-fatal warning attached to a response.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ResponseMeta contains metadata about a command response.
type ResponseMeta struct {
	TraceID          string  `json:"trace_id"`
	ProtocolVersion  string  `json:"protocol_version"`
	ActiveDatabaseID *string `json:"active_database_id"`
	DurationMs       int64   `json:"duration_ms"`
}

// ErrorDetail describes an error in a failed response.
type ErrorDetail struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"`
	Details   any       `json:"details,omitempty"`
}

// Response is the generic JSON envelope for all Querylex command responses.
type Response[T any] struct {
	Success  bool          `json:"success"`
	Data     T             `json:"data,omitempty"`
	Error    *ErrorDetail  `json:"error,omitempty"`
	Warnings []Warning     `json:"warnings,omitempty"`
	Meta     ResponseMeta  `json:"meta"`
}

// NewSuccessResponse creates a success Response with the provided data and trace ID.
func NewSuccessResponse[T any](data T, traceID string, activeDBID *string) *Response[T] {
	return &Response[T]{
		Success: true,
		Data:    data,
		Meta: ResponseMeta{
			TraceID:          traceID,
			ProtocolVersion:  "1.0.0",
			ActiveDatabaseID: activeDBID,
		},
	}
}

// NewErrorResponse creates an error Response with the provided error information.
func NewErrorResponse[T any](code ErrorCode, message string, retryable bool, traceID string) *Response[T] {
	return &Response[T]{
		Success: false,
		Error: &ErrorDetail{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
		Meta: ResponseMeta{
			TraceID:         traceID,
			ProtocolVersion: "1.0.0",
		},
	}
}

// Complete sets the DurationMs field based on the elapsed time since startTime.
func (r *Response[T]) Complete(startTime time.Time) {
	r.Meta.DurationMs = time.Since(startTime).Milliseconds()
}

// GenerateTraceID returns a new UUID v4 string for use as a trace ID.
func GenerateTraceID() string {
	return uuid.New().String()
}
