package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestXSSVulnerability(t *testing.T) {
	// Malicious HTML content with multiple vectors
	htmlBody := `
	<!DOCTYPE html>
	<html>
	<head><title>Hacked Title</title></head>
	<body>
		<p>Normal text</p>
		<script>alert('XSS')</script>
		<img src="x" onerror="alert(1)">
		<a href="javascript:alert(1)">Click me</a>
		<iframe src="javascript:alert(1)"></iframe>
	</body>
	</html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	// Override httpClient
	oldClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = oldClient }()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}

	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)

	// Fetch and parse
	art, err := fetchAndParse(ctx, u, req)
	if err != nil {
		t.Fatalf("fetchAndParse returned error: %v", err)
	}

	// Render to buffer (raw vulnerable content)
	var contentBuf bytes.Buffer
	if err := art.RenderHTML(&contentBuf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	// Capture output of formatHTML (which should be sanitized)
	rr := httptest.NewRecorder()
	formatHTML(rr, art, &contentBuf)

	sanitizedOutput := rr.Body.String()

	// Check for vectors in the SANITIZED output
	vectors := []string{
		"<script>",
		"onerror=",
		"javascript:",
		"iframe",
	}

	for _, v := range vectors {
		if strings.Contains(sanitizedOutput, v) {
			t.Errorf("VULNERABLE: Found malicious vector %q in output content", v)
			t.Logf("Output content snippet: %s", sanitizedOutput)
		}
	}
}
