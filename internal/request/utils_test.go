package request

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

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
		if got := IsLLM(req); got != tt.want {
			t.Errorf("IsLLM(UA=%q) = %v; want %v", tt.ua, tt.want, got)
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
		if got := GetFormat(req); got != tt.want {
			t.Errorf("GetFormat(%q, UA=%q, Accept=%q) = %q; want %q", tt.urlStr, tt.ua, tt.accept, got, tt.want)
		}
	}
}

func TestReconstructTargetURL(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple url",
			query:    "?url=http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "url with encoded params",
			query:    "?url=http%3A%2F%2Fexample.com%3Ffoo%3Dbar",
			expected: "http://example.com?foo=bar",
		},
		{
			name:     "split params",
			query:    "?url=http://example.com&foo=bar&baz=qux",
			expected: "http://example.com?foo=bar&baz=qux",
		},
		{
			name:     "split params with existing params",
			query:    "?url=http://example.com?a=b&c=d",
			expected: "http://example.com?a=b&c=d",
		},
		{
			name:     "mixed params",
			query:    "?url=http%3A%2F%2Fexample.com%3Fa%3Db&c=d",
			expected: "http://example.com?a=b&c=d",
		},
		{
			name:     "ignore format param",
			query:    "?url=http://example.com&format=json&foo=bar",
			expected: "http://example.com?foo=bar",
		},
		{
			name:     "empty url",
			query:    "?format=json",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("http://localhost/api" + tt.query)
			r := &http.Request{URL: u}
			got := ReconstructURL(r)

			if got == "" && tt.expected == "" {
				return
			}

			gotU, _ := url.Parse(got)
			expU, _ := url.Parse(tt.expected)

			if gotU == nil || expU == nil {
				if got != tt.expected {
					t.Errorf("ReconstructURL() = %v, want %v", got, tt.expected)
				}
				return
			}

			if gotU.Scheme != expU.Scheme || gotU.Host != expU.Host || gotU.Path != expU.Path {
				t.Errorf("ReconstructURL() base mismatch = %v, want %v", got, tt.expected)
			}

			gotQ := gotU.Query()
			expQ := expU.Query()

			if len(gotQ) != len(expQ) {
				t.Errorf("ReconstructURL() query length mismatch = %v, want %v", gotQ, expQ)
			}

			for k, v := range expQ {
				if !reflect.DeepEqual(gotQ[k], v) {
					t.Errorf("ReconstructURL() param %s mismatch = %v, want %v", k, gotQ[k], v)
				}
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
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
		u, err := NormalizeURL(tt.raw)
		if tt.shouldErr {
			if err == nil {
				t.Errorf("NormalizeURL(%q) expected error, got none", tt.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeURL(%q) unexpected error: %v", tt.raw, err)
			continue
		}
		got := u.Scheme + "://" + u.Host
		if got != tt.want {
			t.Errorf("NormalizeURL(%q) = %q; want %q", tt.raw, got, tt.want)
		}
	}
}
