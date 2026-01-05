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
	if art.Title != "Test Title" {
		t.Errorf("Article.Title = %q; want %q", art.Title, "Test Title")
	}
	if !strings.Contains(art.Content, "<p>Hello World") {
		t.Errorf("Article.Content missing expected paragraph, got: %q", art.Content)
	}
}

func TestFetchAndParse_SSRFRedirect(t *testing.T) {
	// privateTarget is the handler that should NOT be reached.
	privateTarget := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Request was made to the private target, SSRF exploit is possible")
		w.WriteHeader(http.StatusOK)
	})
	privateServer := httptest.NewServer(privateTarget)
	defer privateServer.Close()

	// redirecter serves a redirect to the private server.
	redirecter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, privateServer.URL, http.StatusFound)
	})
	publicServer := httptest.NewServer(redirecter)
	defer publicServer.Close()

	// Override httpClient to use the test server's client.
	oldClient := httpClient
	httpClient = publicServer.Client()
	// Disable redirects in the test client to mimick the real client's behavior
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { httpClient = oldClient }()

	u, err := url.Parse(publicServer.URL)
	if err != nil {
		t.Fatalf("failed to parse public server URL: %v", err)
	}

	ctx := context.Background()
	_, err = fetchAndParse(ctx, u)

	if err == nil {
		t.Fatal("fetchAndParse did not return an error for a redirect to a private IP")
	}

	// Check if the error message indicates that the redirect was blocked.
	expectedErrorSubstring := "refusing private network address"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("fetchAndParse error message = %q; want substring %q", err.Error(), expectedErrorSubstring)
	}
}
