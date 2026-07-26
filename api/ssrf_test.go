package handler

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsForbiddenDialIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		// Public — allowed
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false}, // Google Public DNS (global unicast)

		// Classic private / loopback / link-local
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"0.0.0.0", true},
		{"169.254.169.254", true}, // cloud metadata (link-local)
		{"::1", true},
		{"fc00::1", true}, // ULA
		{"fe80::1", true}, // link-local

		// Gaps in net.IP.IsPrivate that we must still block
		{"100.64.0.1", true},      // CGNAT
		{"100.127.255.254", true}, // CGNAT upper edge
		{"100.63.255.255", false}, // just below CGNAT
		{"100.128.0.0", false},    // just above CGNAT
		{"198.18.0.1", true},      // benchmarking
		{"192.0.2.1", true},       // TEST-NET-1
		{"198.51.100.1", true},    // TEST-NET-2
		{"203.0.113.1", true},     // TEST-NET-3

		// nil / garbage
		{"", true},
	}

	for _, tt := range tests {
		var ip net.IP
		if tt.ip != "" {
			ip = net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("ParseIP(%q) returned nil", tt.ip)
			}
		}
		got := isForbiddenDialIP(ip)
		if got != tt.blocked {
			t.Errorf("isForbiddenDialIP(%q) = %v; want %v", tt.ip, got, tt.blocked)
		}
	}
}

func TestSSRFBlocksCGNAT(t *testing.T) {
	// 100.64.0.0/10 is not IsPrivate in Go; the dialer must still refuse it.
	// No listener needed: Control rejects before connect.
	req, err := http.NewRequest("GET", "http://100.64.0.1:80/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = httpClient.Do(req)
	if err == nil {
		t.Fatal("expected error dialing CGNAT 100.64.0.1, got nil")
	}
	if !errors.Is(err, ErrPrivateNetwork) {
		t.Errorf("error = %v; want errors.Is(..., ErrPrivateNetwork)", err)
	}
}

/**
 * TestSSRFProtection confirms that the custom dialer correctly blocks connections
 * to private and loopback IP addresses.
 *
 * This is a critical security control to prevent the application from being used
 * as a proxy to attack internal infrastructure (SSRF).
 */
func TestSSRFProtection(t *testing.T) {
	// a dummy server that should never be reached
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
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
	// Control error is wrapped by the dial stack; use errors.Is.
	if !errors.Is(err, ErrPrivateNetwork) {
		t.Errorf("expected errors.Is(..., ErrPrivateNetwork), got: %v", err)
	}

	// Test Unspecified IP (0.0.0.0) bypass attempt
	// We manually construct a URL with 0.0.0.0 and a port (it doesn't need to be open for the check to fire)
	unspecifiedURL := "http://0.0.0.0:8080"
	reqUnspecified, err := http.NewRequest("GET", unspecifiedURL, nil)
	if err != nil {
		t.Fatalf("failed to create request for unspecified IP: %v", err)
	}
	_, err = httpClient.Do(reqUnspecified)
	if err == nil {
		t.Fatal("expected an error when dialing 0.0.0.0, but got none")
	}
	if !errors.Is(err, ErrPrivateNetwork) {
		t.Errorf("expected errors.Is(..., ErrPrivateNetwork) for 0.0.0.0, got: %v", err)
	}
}
