package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXSSPrevention(t *testing.T) {
	// Bypass SSRF protection for local test server
	originalClient := httpClient
	httpClient = &http.Client{
		Timeout: httpClientTimeout,
	}
	defer func() { httpClient = originalClient }()

	// 1. Setup malicious server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `
			<html>
				<body>
					<article>
						<h1>Test Article</h1>
						<p>Safe text.</p>
						<img src="x" onerror="alert('XSS')">
						<a href="javascript:alert(1)">Click me</a>
					</article>
				</body>
			</html>
		`)
	}))
	defer ts.Close()

	// 2. Create request to our handler
	// We need to pass the target URL as a query param
	req := httptest.NewRequest("GET", "/?url="+ts.URL, nil)
	w := httptest.NewRecorder()

	// 3. Call the handler
	Handler(w, req)

	// 4. Check results
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body := w.Body.String()

	// 5. Assertions
	if strings.Contains(body, "onerror") {
		t.Errorf("Response contains 'onerror' attribute (XSS vector): %s", body)
	}
	if strings.Contains(body, "javascript:") {
		t.Errorf("Response contains 'javascript:' link (XSS vector): %s", body)
	}
	if !strings.Contains(body, "Safe text") {
		t.Errorf("Response missing safe content")
	}
}
