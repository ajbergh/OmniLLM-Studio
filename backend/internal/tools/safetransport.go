package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// blockedCIDRs contains private, loopback, link-local, and cloud-metadata IP ranges.
var blockedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918 private
		"172.16.0.0/12",  // RFC1918 private
		"192.168.0.0/16", // RFC1918 private
		"169.254.0.0/16", // link-local / cloud metadata
		"0.0.0.0/8",      // "this" network
		"100.64.0.0/10",  // carrier-grade NAT
		"192.0.0.0/24",   // IETF protocol assignments
		"198.18.0.0/15",  // benchmarking
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA
		"fe80::/10",      // IPv6 link-local
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, _ := net.ParseCIDR(c)
		if n != nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// isBlockedIP checks whether an IP falls within any blocked CIDR range.
func isBlockedIP(ip net.IP) bool {
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeDialer returns a custom dialer that blocks connections to internal/private IPs.
func ssrfSafeDialer() *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
	}
}

// ssrfSafeDialContext resolves the address and rejects connections to blocked IP ranges.
func ssrfSafeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	// Resolve DNS first to check the actual IP
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}

	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr.IP) {
			return nil, fmt.Errorf("blocked: request to private/internal IP %s (resolved from %s)", ipAddr.IP, host)
		}
	}

	// All resolved IPs are safe — connect using the first one
	dialer := ssrfSafeDialer()
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// NewSSRFSafeClient creates an HTTP client that blocks requests to private/internal networks.
func NewSSRFSafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: ssrfSafeDialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			// Re-validate redirect target host
			host := req.URL.Hostname()
			ips, err := net.LookupIP(host)
			if err != nil {
				return fmt.Errorf("redirect DNS resolution failed: %w", err)
			}
			for _, ip := range ips {
				if isBlockedIP(ip) {
					return fmt.Errorf("blocked: redirect to private/internal IP %s (resolved from %s)", ip, host)
				}
			}
			return nil
		},
	}
}
