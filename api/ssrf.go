package handler

import (
	"errors"
	"net"
	"syscall"
)

// ErrPrivateNetwork is returned by the SSRF dial Control when the resolved
// dial target is private, loopback, CGNAT, or other special-use space.
// Callers can use errors.Is to detect this refusal without string matching.
var ErrPrivateNetwork = errors.New("refusing to connect to private network address")

// Special-use IPv4 ranges that Go's net.IP.IsPrivate does not cover, but that
// must not be reachable via SSRF (CGNAT, benchmarking, documentation).
var forbiddenIPv4Nets = []net.IPNet{
	// RFC 6598 — Shared Address Space (carrier-grade NAT), e.g. some cloud / VPN overlays
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)},
	// RFC 2544 — Network Interconnect Device Benchmark Testing
	{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)},
	// RFC 5737 — documentation (TEST-NET-1/2/3)
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)},
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)},
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)},
	// RFC 6890 — IETF protocol assignments
	{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)},
}

// isForbiddenDialIP reports whether connecting to ip would be an SSRF risk.
//
// Go's Dialer.Control is invoked with the concrete dial target (IP:port) after
// DNS resolution. We parse that IP (fail closed if it is not an IP) and reject:
// non-global-unicast, RFC1918/ULA private, and common special-use ranges that
// IsPrivate does not cover (notably CGNAT 100.64.0.0/10).
func isForbiddenDialIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Normalize IPv4-mapped IPv6 so To4-based nets match.
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	// IsGlobalUnicast is false for loopback, link-local, multicast, unspecified, etc.
	// It is still true for RFC1918/ULA and CGNAT, so those need extra checks.
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return true
	}
	for i := range forbiddenIPv4Nets {
		if forbiddenIPv4Nets[i].Contains(ip) {
			return true
		}
	}
	return false
}

/**
 * newSafeDialer creates a custom net.Dialer that prevents Server-Side Request Forgery (SSRF).
 *
 * It validates the concrete dial IP (post-DNS) before connecting, ensuring it is not:
 * - Private / ULA (RFC 1918, RFC 4193)
 * - Loopback, link-local, multicast, or unspecified
 * - CGNAT shared space (RFC 6598 100.64.0.0/10)
 * - Documentation / benchmarking special-use ranges
 *
 * Validation runs after DNS resolution and before the socket connects, so DNS
 * rebinding cannot swap a public A record for a private IP after the check.
 */
func newSafeDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   dialerTimeout,
		KeepAlive: dialerKeepAlive,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			// Dialer.Control receives the resolved dial target as IP:port (not a hostname).
			// ParseIP fail-closed avoids a second DNS lookup that could diverge.
			ip := net.ParseIP(host)
			if isForbiddenDialIP(ip) {
				return ErrPrivateNetwork
			}
			return nil
		},
	}
}
