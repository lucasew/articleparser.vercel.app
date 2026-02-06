package handler

import (
	"net/http/httptest"
	"testing"
)

/**
 * TestIsLLM verifies the detection of Large Language Model (LLM) bots.
 *
 * This ensures that when an LLM (like GPTBot) accesses the service, it
 * automatically receives Markdown content, which is more token-efficient
 * and easier for the model to process than full HTML.
 */
func TestIsLLM(t *testing.T) {
	tests := []struct {
		ua   string
		want bool
	}{
		{"Mozilla/5.0 (compatible; GPTBot/1.0; +https://openai.com/gptbot)", true},
		{"ChatGPT-User/1.0", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", tt.ua)
		if got := isLLM(req); got != tt.want {
			t.Errorf("isLLM(UA=%q) = %v; want %v", tt.ua, tt.want, got)
		}
	}
}

func TestGetFormat(t *testing.T) {
	tests := []struct {
		urlStr string
		ua     string
		accept string
		want   string
	}{
		{"/api?url=...&format=json", "", "", "json"},
		{"/api?url=...", "ChatGPT-User/1.0", "", "md"},
		{"/api?url=...", "Mozilla/5.0", "", "html"},
		{"/api?url=...", "Mozilla/5.0", "application/json", "json"},
		{"/api?url=...", "Mozilla/5.0", "text/markdown", "md"},
		{"/api?url=...", "Mozilla/5.0", "text/plain", "text"},
		// Query param should override Accept
		{"/api?url=...&format=txt", "Mozilla/5.0", "application/json", "txt"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.urlStr, nil)
		req.Header.Set("User-Agent", tt.ua)
		req.Header.Set("Accept", tt.accept)
		if got := getFormat(req); got != tt.want {
			t.Errorf("getFormat(%q, UA=%q, Accept=%q) = %q; want %q", tt.urlStr, tt.ua, tt.accept, got, tt.want)
		}
	}
}
