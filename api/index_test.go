package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeAndValidateURL(t *testing.T) {
	tests := []struct {
		raw       string
		want      string // expected host (with scheme)
		shouldErr bool
	}{
		{"", "", true},
		{"example.com", "https://example.com", false},
		{"http://foo.bar", "http://foo.bar", false},
		{"https:/go.dev/play", "https://go.dev", false},
		{"http:/example.com", "http://example.com", false},
		{"ftp://foo.bar", "", true},
	}
	for _, tt := range tests {
		u, err := normalizeAndValidateURL(tt.raw)
		if tt.shouldErr {
			if err == nil {
				t.Errorf("normalizeAndValidateURL(%q) expected error, got none", tt.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeAndValidateURL(%q) unexpected error: %v", tt.raw, err)
			continue
		}
		got := u.Scheme + "://" + u.Host
		if got != tt.want {
			t.Errorf("normalizeAndValidateURL(%q) = %q; want %q", tt.raw, got, tt.want)
		}
	}
}

func TestFetchAndParse(t *testing.T) {
	// Serve a minimal HTML page
	htmlBody := `<html><head><title>Test Title</title></head><body><p>Hello World</p></body></html>`
	srv, cleanup := setupTestServer(t, htmlBody)
	defer cleanup()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	ctx := t.Context()
	req := httptest.NewRequest("GET", "/", nil)
	art, err := fetchAndParse(ctx, u, req)
	if err != nil {
		t.Fatalf("fetchAndParse returned error: %v", err)
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

func TestFetchAndParseRejectsOversizedBody(t *testing.T) {
	// Body larger than maxBodySize must error, not parse a truncated page.
	oversized := strings.Repeat("x", int(maxBodySize)+1)
	htmlBody := "<html><head><title>Big</title></head><body><p>" + oversized + "</p></body></html>"
	srv, cleanup := setupTestServer(t, htmlBody)
	defer cleanup()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	_, err = fetchAndParse(t.Context(), u, req)
	if err == nil {
		t.Fatal("fetchAndParse: expected error for oversized body, got nil")
	}
}

func setupTestServer(t *testing.T, htmlBody string) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(htmlBody)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))

	oldClient := httpClient
	httpClient = srv.Client()
	return srv, func() {
		httpClient = oldClient
		srv.Close()
	}
}
