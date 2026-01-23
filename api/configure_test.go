package handler

import (
	"net/http/httptest"
	"testing"
)

func TestConfigureRequest(t *testing.T) {
	// Create a dummy original request
	originalReq := httptest.NewRequest("GET", "http://example.com/api", nil)
	originalReq.Header.Set("Accept-Language", "fr-FR")

	// Create a target request
	targetReq := httptest.NewRequest("GET", "http://target.com/article", nil)

	// Call the helper
	configureRequest(targetReq, originalReq)

	// Verify headers
	if targetReq.Header.Get("User-Agent") == "" {
		t.Error("User-Agent header not set")
	}

	if got := targetReq.Header.Get("Accept-Language"); got != "fr-FR" {
		t.Errorf("Accept-Language = %q, want %q", got, "fr-FR")
	}

	if targetReq.Header.Get("Sec-Fetch-Dest") != "document" {
		t.Error("Sec-Fetch-Dest header not set correctly")
	}
}

func TestConfigureRequestDefaultLanguage(t *testing.T) {
	// Create a dummy original request without Accept-Language
	originalReq := httptest.NewRequest("GET", "http://example.com/api", nil)

	// Create a target request
	targetReq := httptest.NewRequest("GET", "http://target.com/article", nil)

	// Call the helper
	configureRequest(targetReq, originalReq)

	// Verify default language
	if got := targetReq.Header.Get("Accept-Language"); got != "en-US,en;q=0.9" {
		t.Errorf("Accept-Language = %q, want %q", got, "en-US,en;q=0.9")
	}
}
