package article

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestFetch(t *testing.T) {
	// Serve a minimal HTML page
	htmlBody := `<html><head><title>Test Title</title></head><body><p>Hello World</p></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(htmlBody)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	ctx := context.Background()
	req := httptest.NewRequest("GET", "/", nil)

	// Use server's client which is configured to talk to the test server
	art, err := Fetch(ctx, u, req, srv.Client())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if art.Title() != "Test Title" {
		t.Errorf("Article.Title() = %q; want %q", art.Title(), "Test Title")
	}

	var content strings.Builder
	err = art.RenderHTML(&content)
	if err != nil {
		t.Fatalf("failed to render article content: %v", err)
	}

	if !strings.Contains(content.String(), "<p>Hello World") {
		t.Errorf("Article.Content missing expected paragraph, got: %q", content.String())
	}
}
