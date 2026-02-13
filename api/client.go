package handler

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

const (
	maxRedirects      = 5
	httpClientTimeout = 10 * time.Second
	dialerTimeout     = 30 * time.Second
	dialerKeepAlive   = 30 * time.Second
)

/**
 * newSafeDialer creates a custom net.Dialer that prevents Server-Side Request Forgery (SSRF).
 *
 * It validates the resolved IP address before connecting, ensuring that it is not:
 * - A private network address (e.g., 192.168.x.x, 10.x.x.x)
 * - A loopback address (e.g., 127.0.0.1)
 * - An unspecified address (e.g., 0.0.0.0)
 *
 * This validation happens *after* DNS resolution but *before* the connection is established.
 * This prevents Time-of-Check Time-of-Use (TOCTOU) attacks where a domain could
 * resolve to a safe IP during check but switch to a private IP during connection.
 *
 * This is critical for preventing the application from accessing internal services or metadata services
 * (like AWS EC2 metadata) running on the same network.
 */
func newSafeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ips, err := net.LookupIP(host)
			if err != nil {
				return err
			}
			for _, ip := range ips {
				if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
					return errors.New("refusing to connect to private network address")
				}
			}
			return nil
		},
	}
	return dialer
}

// httpClient used for fetching remote articles with timeouts and redirect policy
var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: newSafeDialer().DialContext,
	},
	Timeout: httpClientTimeout,
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	},
}
