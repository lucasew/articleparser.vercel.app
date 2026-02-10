package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roundTripFunc helper to mock http.Client
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestInvalidFormatEarlyReturn(t *testing.T) {
	// Mock httpClient to ensure no request is made
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("httpClient.Do called but should have been skipped due to invalid format")
			return nil, nil
		}),
	}

	// Capture logs to verify injection prevention
	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	// Create request with invalid format containing a newline (Log Injection attempt)
	// We use %0a to simulate newline in query param
	req := httptest.NewRequest("GET", "/?url=http://example.com&format=invalid%0ainjection", nil)
	w := httptest.NewRecorder()

	// Call the unexported handler
	handler(w, req)

	// 1. Verify that the handler returned 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}

	// 2. Verify log injection is prevented
	logOutput := logBuf.String()

	// We expect the log to contain the escaped version "invalid\ninjection" (quoted)
	expectedLogPart := `"invalid\ninjection"`
	if !strings.Contains(logOutput, expectedLogPart) {
		t.Errorf("log output does not contain safe/escaped format string.\nGot: %s\nExpected to contain: %s", logOutput, expectedLogPart)
	}
}

func TestValidFormatLogInjection(t *testing.T) {
	// Mock httpClient to allow fetching
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("<html></html>")),
			}, nil
		}),
	}

	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	// Valid format, invalid URL with newline
	req := httptest.NewRequest("GET", "/?url=http://example.com/foo%0abar&format=html", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	logOutput := logBuf.String()

	// Check for escaped newline in URL
	// The literal string in log should be: "http://example.com/foo\nbar"
	if !strings.Contains(logOutput, `foo\nbar`) {
		t.Errorf("log output does not contain escaped newline in URL.\nGot: %s", logOutput)
	}
}
