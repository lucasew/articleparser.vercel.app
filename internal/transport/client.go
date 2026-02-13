package transport

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
 * NewSafeClient creates a custom http.Client that prevents Server-Side Request Forgery (SSRF).
 *
 * It uses a custom dialer that validates the resolved IP address before connecting, ensuring that it is not:
 * - A private network address (e.g., 192.168.x.x, 10.x.x.x)
 * - A loopback address (e.g., 127.0.0.1)
 * - An unspecified address (e.g., 0.0.0.0)
 */
func NewSafeClient() *http.Client {
	return &http.Client{
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
}

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
