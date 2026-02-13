package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

/**
 * TestSSRFProtection confirms that the custom dialer correctly blocks connections
 * to private and loopback IP addresses.
 *
 * This is a critical security control to prevent the application from being used
 * as a proxy to attack internal infrastructure (SSRF).
 */
func TestSSRFProtection(t *testing.T) {
	client := NewSafeClient()
	// a dummy server that should never be reached
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("dialer did not block private IP, connection was made")
	}))
	defer srv.Close()

	// get loopback address of the server
	// srv.URL will be something like http://127.0.0.1:54321
	// we want to test if the dialer blocks the connection to 127.0.0.1
	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = client.Do(req)
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
	_, err = client.Do(reqUnspecified)
	if err == nil {
		t.Fatal("expected an error when dialing 0.0.0.0, but got none")
	}
	if !strings.Contains(err.Error(), "refusing to connect to private network address") {
		t.Errorf("expected error for 0.0.0.0 to contain 'refusing to connect to private network address', but got: %v", err)
	}
}
