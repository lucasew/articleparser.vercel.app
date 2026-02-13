package handler

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

/**
 * TestReconstructTargetURL verifies the logic for reassembling URLs that have been
 * split by Vercel's rewrite rules.
 *
 * When Vercel rewrites a request like `/api?url=http://example.com?foo=bar`,
 * it parses the query string *before* passing it to the Go handler. This often
 * results in `url=http://example.com` and `foo=bar` being treated as separate
 * parameters, rather than `foo=bar` being part of the `url` value.
 *
 * The reconstruction logic detects these "stray" parameters and merges them
 * back into the target URL to ensure the fetcher requests the correct resource.
 */
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
			u, err := url.Parse("http://localhost/api" + tt.query)
			if err != nil {
				t.Fatalf("failed to parse test URL: %v", err)
			}
			r := &http.Request{URL: u}
			got := reconstructTargetURL(r)

			if got == "" && tt.expected == "" {
				return
			}

			gotU, err := url.Parse(got)
			if err != nil {
				t.Fatalf("failed to parse result URL %q: %v", got, err)
			}
			expU, err := url.Parse(tt.expected)
			if err != nil {
				t.Fatalf("failed to parse expected URL %q: %v", tt.expected, err)
			}

			if gotU.Scheme != expU.Scheme || gotU.Host != expU.Host || gotU.Path != expU.Path {
				t.Errorf("reconstructTargetURL() base mismatch = %v, want %v", got, tt.expected)
			}

			gotQ := gotU.Query()
			expQ := expU.Query()

			if len(gotQ) != len(expQ) {
				t.Errorf("reconstructTargetURL() query length mismatch = %v, want %v", gotQ, expQ)
			}

			for k, v := range expQ {
				if !reflect.DeepEqual(gotQ[k], v) {
					t.Errorf("reconstructTargetURL() param %s mismatch = %v, want %v", k, gotQ[k], v)
				}
			}
		})
	}
}
