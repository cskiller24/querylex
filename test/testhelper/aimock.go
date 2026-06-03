// Package testhelper provides reusable test utilities for QueryLex E2E tests.
package testhelper

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// AIMockServer wraps an httptest.Server that mimics the OpenAI-compatible
// /v1/chat/completions endpoint. It records all received requests for
// post-test assertion.
type AIMockServer struct {
	Server   *httptest.Server
	mode     string // "success", "error", "retry"
	mu       sync.Mutex
	requests []AIMockRequest
}

// AIMockRequest represents a captured HTTP request to the mock server.
type AIMockRequest struct {
	Body    []byte
	Headers map[string][]string
}

// StartAIMockServer creates and starts an httptest.Server that mocks the
// OpenAI /v1/chat/completions endpoint. The server is automatically closed
// via t.Cleanup. It sets QUERYLEX_AI_ENDPOINT and QUERYLEX_AI_API_KEY=fake
// environment variables so the querylex subprocess points to the mock.
//
// Modes:
//
//	"success" — returns HTTP 200 with a realistic chat completion JSON
//	"error"   — returns HTTP 500 with an OpenAI-format error JSON
//	"retry"   — returns HTTP 429 on first request, HTTP 200 on subsequent
func StartAIMockServer(t *testing.T, mode string) *AIMockServer {
	t.Helper()

	mock := &AIMockServer{mode: mode}
	mock.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate HTTP method
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Validate Content-Type
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		// Read request body
		body, _ := io.ReadAll(r.Body)

		// Record request under mutex lock
		mock.mu.Lock()
		mock.requests = append(mock.requests, AIMockRequest{
			Body:    body,
			Headers: r.Header.Clone(),
		})
		reqCount := len(mock.requests)
		mock.mu.Unlock()

		// Route by mode
		switch mock.mode {
		case "error":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Internal server error",
					"type":    "server_error",
					"code":    "internal_error",
				},
			})
		case "retry":
			if reqCount == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"message": "Rate limit exceeded",
						"type":    "rate_limit_error",
						"code":    "rate_limit_exceeded",
					},
				})
				return
			}
			// Fall through to success on retry
			writeSuccessResponse(w)
		default: // "success" and unknown mode
			writeSuccessResponse(w)
		}
	}))

	t.Cleanup(mock.Server.Close)

	// Set env vars for the querylex subprocess
	t.Setenv("QUERYLEX_AI_ENDPOINT", mock.Server.URL)
	t.Setenv("QUERYLEX_AI_API_KEY", "fake")
	t.Setenv("TEST_AI_MOCK_MODE", mode)

	return mock
}

// writeSuccessResponse writes a realistic OpenAI ChatCompletion success
// response with a valid SQL query to the HTTP response writer.
func writeSuccessResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"id":      "chatcmpl-mock-001",
		"object":  "chat.completion",
		"created": 1677652288,
		"model":   "gpt-4o-mock",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "SELECT * FROM actor LIMIT 5;",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     100,
			"completion_tokens": 20,
			"total_tokens":      120,
		},
	})
}

// LastRequest returns the body of the most recent request received by the
// mock server. Returns nil if no requests have been received.
func (m *AIMockServer) LastRequest() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return nil
	}
	return m.requests[len(m.requests)-1].Body
}

// AllRequests returns a copy of all requests received by the mock server.
// The returned slice is safe to use without holding the mutex.
func (m *AIMockServer) AllRequests() []AIMockRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]AIMockRequest, len(m.requests))
	copy(result, m.requests)
	return result
}
