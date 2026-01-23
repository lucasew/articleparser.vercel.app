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

func TestXSSSanitization(t *testing.T) {
	// Malicious HTML content
	maliciousHTML := `<html><head><title>Hacked</title></head><body>
		<p>Safe content</p>
		<img src=x onerror=alert('XSS')>
		<script>alert('Script XSS')</script>
		<a href="javascript:alert(1)">Click me</a>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(maliciousHTML))
	}))
	defer srv.Close()

	// Override httpClient
	oldClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = oldClient }()

	u, _ := url.Parse(srv.URL)
	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)

	article, err := fetchAndParse(ctx, u, req)
	if err != nil {
		t.Fatalf("fetchAndParse failed: %v", err)
	}

	// Test formatHTML logic (where template.HTML is used)
	contentBuf := &bytes.Buffer{}
	if err := article.RenderHTML(contentBuf); err != nil {
		t.Fatalf("RenderHTML failed: %v", err)
	}

	// Manually invoke the sanitizer like formatHTML does to test the effect
	// Note: We can't easily capture the output of formatHTML without mocking the writer,
	// but we can test the sanitizer variable if it was exported, or better,
	// we can actually call formatHTML and capture the output.

	recorder := httptest.NewRecorder()
	formatHTML(recorder, article, contentBuf)

	output := recorder.Body.String()

	// Verification
	if strings.Contains(output, "onerror") {
		t.Errorf("FAIL: Output contains 'onerror' attribute")
	}
	if strings.Contains(output, "<script>") {
		t.Errorf("FAIL: Output contains <script> tag")
	}
	if strings.Contains(output, "javascript:") {
		t.Errorf("FAIL: Output contains 'javascript:' href")
	}
	if !strings.Contains(output, "Safe content") {
		t.Errorf("FAIL: Output missing safe content")
	}

	// Test formatJSON as well
	jsonRecorder := httptest.NewRecorder()
	formatJSON(jsonRecorder, article, contentBuf)
	jsonOutput := jsonRecorder.Body.String()

	if strings.Contains(jsonOutput, "onerror") {
		t.Errorf("FAIL: JSON output contains 'onerror' attribute")
	}
}
