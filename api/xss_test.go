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
	// Malicious HTML payload
	htmlBody := `
<!DOCTYPE html>
<html>
<head><title>XSS Test</title></head>
<body>
	<h1>XSS Test</h1>
	<p>Safe content</p>
	<script>alert('XSS')</script>
	<img src="x" onerror="alert('img XSS')">
	<a href="javascript:alert('link XSS')">Click me</a>
</body>
</html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(htmlBody)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
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
		t.Fatalf("fetchAndParse failed: %v", err)
	}

	contentBuf := &bytes.Buffer{}
	if err := art.RenderHTML(contentBuf); err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	// We must test the output of formatHTML, which applies the sanitization
	rec := httptest.NewRecorder()
	formatHTML(rec, art, contentBuf)

	resp := rec.Result()
	bodyBuf := new(bytes.Buffer)
	bodyBuf.ReadFrom(resp.Body)
	content := bodyBuf.String()

	// Check for XSS vectors
	t.Logf("Rendered content: %s", content)

	if strings.Contains(content, "<script>") {
		t.Error("VULNERABILITY: <script> tag not stripped")
	}
	if strings.Contains(content, "onerror") {
		t.Error("VULNERABILITY: onerror handler not stripped")
	}
	if strings.Contains(content, "javascript:") {
		t.Error("VULNERABILITY: javascript: URI not stripped")
	}
}
