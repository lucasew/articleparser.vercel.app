package handler

import (
	"context"
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
		{"ftp://foo.bar", "", true},
		{"127.0.0.1", "", true},
		{"192.168.0.5/path", "", true},
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(htmlBody))
	}))
	defer srv.Close()

	// Override httpClient to use server's client
	oldClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = oldClient }()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	ctx := context.Background()
	art, err := fetchAndParse(ctx, u)
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
