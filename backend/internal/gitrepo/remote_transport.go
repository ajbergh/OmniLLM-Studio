package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const maxRemoteAdvertisementBytes int64 = 4 << 20

var errRemoteResponseTooLarge = errors.New("remote Git response exceeds the configured limit")

var blockedRemoteCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.168.0.0/16",
		"198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4", "::/128", "::1/128",
		"fc00::/7", "fe80::/10", "ff00::/8",
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil {
			out = append(out, network)
		}
	}
	return out
}()

func isBlockedRemoteIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, network := range blockedRemoteCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// remoteSafeDialContext resolves once, rejects every private/local/reserved
// result, then dials one of the already-validated IPs directly. This prevents a
// validate-then-redial DNS-rebinding gap.
func remoteSafeDialContext() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid remote Git address")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("remote Git DNS resolution failed")
		}
		validated := make([]net.IP, 0, len(ips))
		for _, resolved := range ips {
			if isBlockedRemoteIP(resolved.IP) {
				return nil, fmt.Errorf("remote Git destination is blocked by egress policy")
			}
			validated = append(validated, resolved.IP)
		}
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		for _, ip := range validated {
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return connection, nil
			}
		}
		return nil, fmt.Errorf("remote Git connection failed")
	}
}

type boundedRemoteRoundTripper struct {
	base http.RoundTripper
	max  int64
}

func (t boundedRemoteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if response.ContentLength > t.max {
		_ = response.Body.Close()
		return nil, errRemoteResponseTooLarge
	}
	response.Body = &boundedRemoteReadCloser{ReadCloser: response.Body, remaining: t.max}
	return response, nil
}

type boundedRemoteReadCloser struct {
	io.ReadCloser
	remaining int64
}

func (r *boundedRemoteReadCloser) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		probe := make([]byte, 1)
		n, err := r.ReadCloser.Read(probe)
		if n > 0 {
			return 0, errRemoteResponseTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	return n, err
}

// newRemoteStatusTransport creates a dedicated go-git HTTPS transport for
// advertised-reference inspection. It does not mutate http.DefaultTransport or
// go-git's global protocol registry, does not use environment proxies, and does
// not follow redirects.
func newRemoteStatusTransport() transport.Transport {
	base := &http.Transport{
		DialContext:           remoteSafeDialContext(),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: boundedRemoteRoundTripper{base: base, max: maxRemoteAdvertisementBytes},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("remote Git redirects are disabled")
		},
	}
	return githttp.NewClientWithOptions(client, &githttp.ClientOptions{RedirectPolicy: githttp.NoFollowRedirects})
}
