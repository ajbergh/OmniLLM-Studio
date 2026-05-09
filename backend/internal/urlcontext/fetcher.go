package urlcontext

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// blockedCIDRs are private/internal IP ranges that must not be reachable via URL fetch.
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

func isBlockedIP(ip net.IP) bool {
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func ssrfSafeDialContext(allowPrivate bool) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}

		if !allowPrivate {
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed: %w", err)
			}
			for _, ipAddr := range ips {
				if isBlockedIP(ipAddr.IP) {
					return nil, fmt.Errorf("%w: %s resolves to %s", ErrBlockedHost, host, ipAddr.IP)
				}
			}
		}

		dialer := &net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}
}

// newHTTPClient creates an SSRF-safe HTTP client.
func newHTTPClient(timeout time.Duration, allowPrivate bool) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: ssrfSafeDialContext(allowPrivate),
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if !allowPrivate {
				host := req.URL.Hostname()
				ips, err := net.LookupIP(host)
				if err != nil {
					return fmt.Errorf("redirect DNS resolution failed: %w", err)
				}
				for _, ip := range ips {
					if isBlockedIP(ip) {
						return fmt.Errorf("%w: redirect to %s (%s)", ErrBlockedHost, host, ip)
					}
				}
			}
			return nil
		},
	}
}

// FetchResult holds the raw result of a URL fetch.
type FetchResult struct {
	Body        []byte
	ContentType string
	FinalURL    string
	Truncated   bool
}

// etagCacheEntry stores a response body and its ETag for conditional requests.
type etagCacheEntry struct {
	etag        string
	body        []byte
	contentType string
}

// Fetcher handles raw HTTP fetching with SSRF protection.
type Fetcher struct {
	client    *http.Client
	userAgent string
	maxBytes  int64
	etagCache sync.Map // map[string]*etagCacheEntry — keyed by URL
}

// NewFetcher creates a Fetcher from config.
func NewFetcher(cfg *Config) *Fetcher {
	return &Fetcher{
		client:    newHTTPClient(cfg.FetchTimeout, cfg.AllowPrivateNetworks),
		userAgent: cfg.UserAgent,
		maxBytes:  cfg.MaxBytesPerSource,
	}
}

// Fetch fetches a URL and returns raw bytes + metadata.
// Returns typed errors (ErrBlockedHost, ErrFetchTimeout, ErrGitHubPrivate, etc.).
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*FetchResult, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, ErrUnsupportedScheme
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html, text/plain, application/json, */*")

	resp, err := f.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "blocked:") {
			return nil, ErrBlockedHost
		}
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
			return nil, ErrFetchTimeout
		}
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 429:
		return nil, ErrGitHubRateLimited
	case 403, 404:
		if strings.Contains(rawURL, "github.com") || strings.Contains(rawURL, "api.github.com") {
			return nil, ErrGitHubPrivate
		}
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	limit := f.maxBytes
	if limit <= 0 {
		limit = 750_000
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	truncated := false
	if int64(len(body)) > limit {
		body = body[:limit]
		truncated = true
	}

	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	return &FetchResult{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    finalURL,
		Truncated:   truncated,
	}, nil
}

// FetchWithAuth fetches a URL using a Bearer token (for GitHub API calls).
// It uses ETag/If-None-Match for conditional requests and retries once on 429
// if Retry-After is ≤ 10 seconds.
func (f *Fetcher) FetchWithAuth(ctx context.Context, rawURL, token string) (*FetchResult, error) {
	return f.fetchWithAuthAttempt(ctx, rawURL, token, true)
}

func (f *Fetcher) fetchWithAuthAttempt(ctx context.Context, rawURL, token string, allowRetry bool) (*FetchResult, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, ErrUnsupportedScheme
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Conditional request: send If-None-Match when we have a cached ETag.
	if cached, ok := f.etagCache.Load(rawURL); ok {
		entry := cached.(*etagCacheEntry)
		if entry.etag != "" {
			req.Header.Set("If-None-Match", entry.etag)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "blocked:") {
			return nil, ErrBlockedHost
		}
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// Warn when rate limit headroom is getting low.
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if n, parseErr := strconv.Atoi(remaining); parseErr == nil && n < 10 {
			log.Printf("WARN urlcontext: GitHub rate limit low: %d requests remaining", n)
		}
	}

	switch resp.StatusCode {
	case 304:
		// Not Modified — return cached body.
		if cached, ok := f.etagCache.Load(rawURL); ok {
			entry := cached.(*etagCacheEntry)
			return &FetchResult{
				Body:        entry.body,
				ContentType: entry.contentType,
				FinalURL:    rawURL,
			}, nil
		}
		// Cache miss with 304 is unexpected; fall through to error.
		return nil, fmt.Errorf("HTTP 304 but no cached body for %s", rawURL)

	case 429:
		if allowRetry {
			waitSecs := 5
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, parseErr := strconv.Atoi(ra); parseErr == nil {
					waitSecs = n
				}
			}
			if waitSecs <= 10 {
				select {
				case <-time.After(time.Duration(waitSecs) * time.Second):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return f.fetchWithAuthAttempt(ctx, rawURL, token, false)
			}
		}
		return nil, ErrGitHubRateLimited

	case 403:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if strings.Contains(string(body), "rate limit") {
			return nil, ErrGitHubRateLimited
		}
		return nil, ErrGitHubPrivate

	case 404:
		return nil, ErrGitHubPrivate
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	limit := f.maxBytes
	if limit <= 0 {
		limit = 750_000
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	truncated := int64(len(body)) > limit
	if truncated {
		body = body[:limit]
	}

	// Cache the ETag for future conditional requests.
	if etag := resp.Header.Get("ETag"); etag != "" {
		f.etagCache.Store(rawURL, &etagCacheEntry{
			etag:        etag,
			body:        body,
			contentType: resp.Header.Get("Content-Type"),
		})
	}

	return &FetchResult{
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    rawURL,
		Truncated:   truncated,
	}, nil
}
