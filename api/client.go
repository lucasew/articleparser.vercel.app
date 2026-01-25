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

var (
	// httpClient used for fetching remote articles with timeouts and redirect policy
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: newSafeDialer().DialContext,
		},
		Timeout: httpClientTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
)

func newSafeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
		Control: func(network, address string, c syscall.RawConn) error {
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
