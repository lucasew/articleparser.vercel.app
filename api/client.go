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
	MAX_REDIRECTS       = 5
	HTTP_CLIENT_TIMEOUT = 10 * time.Second
	DIALER_TIMEOUT      = 30 * time.Second
	DIALER_KEEP_ALIVE   = 30 * time.Second
)

var (
	// httpClient used for fetching remote articles with timeouts and redirect policy
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: NewSafeDialer().DialContext,
		},
		Timeout: HTTP_CLIENT_TIMEOUT,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= MAX_REDIRECTS {
				return fmt.Errorf("stopped after %d redirects", MAX_REDIRECTS)
			}
			return nil
		},
	}
)

func NewSafeDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   DIALER_TIMEOUT,
		KeepAlive: DIALER_KEEP_ALIVE,
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
