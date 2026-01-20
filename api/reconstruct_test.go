package handler

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

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
			got := reconstructTargetURL(r)

			if got == "" && tt.expected == "" {
				return
			}

			gotU, _ := url.Parse(got)
			expU, _ := url.Parse(tt.expected)

            if gotU == nil || expU == nil {
                if got != tt.expected {
                     t.Errorf("reconstructTargetURL() = %v, want %v", got, tt.expected)
                }
                return
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
