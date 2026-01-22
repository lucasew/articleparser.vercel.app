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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("expected User-Agent header")
		}
		if r.Header.Get("Accept-Language") == "" {
			t.Error("expected Accept-Language header")
		}
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

func TestSSRFProtection(t *testing.T) {
	// a dummy server that should never be reached
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dialer did not block private IP, connection was made")
	}))
	defer srv.Close()

	// get loopback address of the server
	// srv.URL will be something like http://127.0.0.1:54321
	// we want to test if the dialer blocks the connection to 127.0.0.1
	// so, we don't use the server's client, we use our own httpClient
	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = httpClient.Do(req)
	if err == nil {
		t.Fatal("expected an error when dialing a private IP, but got none")
	}
	// check if the error is the one we expect from our dialer
	// the error is wrapped, so we need to check for the substring
	if !strings.Contains(err.Error(), "refusing to connect to private network address") {
		t.Errorf("expected error to contain 'refusing to connect to private network address', but got: %v", err)
	}

	// Test Unspecified IP (0.0.0.0) bypass attempt
	// We manually construct a URL with 0.0.0.0 and a port (it doesn't need to be open for the check to fire)
	unspecifiedURL := "http://0.0.0.0:8080"
	reqUnspecified, _ := http.NewRequest("GET", unspecifiedURL, nil)
	_, err = httpClient.Do(reqUnspecified)
	if err == nil {
		t.Fatal("expected an error when dialing 0.0.0.0, but got none")
	}
	if !strings.Contains(err.Error(), "refusing to connect to private network address") {
		t.Errorf("expected error for 0.0.0.0 to contain 'refusing to connect to private network address', but got: %v", err)
	}
}
