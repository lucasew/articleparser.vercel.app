package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXSSPrevention(t *testing.T) {
	// 1. Mock a server serving malicious HTML
	maliciousHTML := `
		<!DOCTYPE html>
		<html>
		<head><title>Malicious Page</title></head>
		<body>
			<h1>Hello</h1>
			<p>Normal text</p>
			<script>alert('XSS')</script>
			<img src="x" onerror="alert('Img XSS')">
			<a href="javascript:alert('Link XSS')">Click me</a>
		</body>
		</html>
	`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(maliciousHTML)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer srv.Close()

	// 2. Override the global httpClient to allow connection to local test server
	//    This bypasses the SSRF protection which would block 127.0.0.1
	oldClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = oldClient }()

	// 3. Create a request to our handler
	//    The handler expects a 'url' query parameter pointing to the target
	req := httptest.NewRequest("GET", "/api?url="+srv.URL, nil)
	w := httptest.NewRecorder()

	// 4. Invoke the handler
	Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %v", resp.Status)
	}

	// 5. Check the response body for XSS vectors
	body := w.Body.String()

	// List of forbidden strings that indicate XSS vulnerability
	forbidden := []string{
		"<script>",
		"onerror=",
		"javascript:",
		"alert('XSS')",
		"alert('Img XSS')",
	}

	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Errorf("Response contains forbidden XSS vector %q. Body excerpt: %s", f, body[:min(len(body), 500)])
		}
	}

	// 6. Ensure valid content is still present
	if !strings.Contains(body, "Normal text") {
		t.Errorf("Response missing valid content 'Normal text'")
	}
}
