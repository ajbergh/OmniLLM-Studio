package urlcontext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// blockedCIDRs are private, local, reserved, and metadata-address ranges that
// must not be reachable through URL context fetching.
var blockedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	}
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil {
			networks = append(networks, network)
		}
	}
	return networks
}()

func isBlockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, cidr := range blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeDialContext resolves a host exactly once, validates every returned
// address, and connects directly to one validated IP. Connecting to the original
// hostname after validation would permit DNS rebinding between the two lookups.
func ssrfSafeDialContext(allowPrivate bool) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS resolution failed: %w", err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("DNS resolution returned no addresses for %s", host)
		}

		validated := make([]net.IP, 0, len(ips))
		for _, resolved := range ips {
			if !allowPrivate && isBlockedIP(resolved.IP) {
				return nil, fmt.Errorf("%w: %s resolves to %s", ErrBlockedHost, host, resolved.IP)
			}
			validated = append(validated, resolved.IP)
		}
		if len(validated) == 0 {
			return nil, fmt.Errorf("no usable addresses for %s", host)
		}

		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		var lastErr error
		for _, ip := range validated {
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			lastErr = dialErr
		}
		return nil, fmt.Errorf("connect to validated address for %s: %w", host, lastErr)
	}
}

func validateFetchURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return ErrUnsupportedScheme
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrUnsupportedScheme
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: URL credentials are not allowed", ErrBlockedHost)
	}
	return nil
}

// newHTTPClient creates an SSRF-safe HTTP client. Redirects are bounded and the
// transport re-resolves, validates, and pins each redirect destination.
func newHTTPClient(timeout time.Duration, allowPrivate bool) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           ssrfSafeDialContext(allowPrivate),
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          20,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validateFetchURL(req.URL.String())
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

func NewFetcher(cfg *Config) *Fetcher {
	return &Fetcher{
		client:    newHTTPClient(cfg.FetchTimeout, cfg.AllowPrivateNetworks),
		userAgent: cfg.UserAgent,
		maxBytes:  cfg.MaxBytesPerSource,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (*FetchResult, error) {
	if err := validateFetchURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html, text/plain, application/json, */*")

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlockedHost) {
			return nil, ErrBlockedHost
		}
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return nil, ErrFetchTimeout
		}
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return nil, ErrGitHubRateLimited
	case http.StatusForbidden, http.StatusNotFound:
		if isGitHubURL(rawURL) {
			return nil, ErrGitHubPrivate
		}
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, truncated, err := readLimitedBody(resp.Body, f.maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
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

// FetchWithAuth fetches a URL using a Bearer token for GitHub API calls.
func (f *Fetcher) FetchWithAuth(ctx context.Context, rawURL, token string) (*FetchResult, error) {
	return f.fetchWithAuthAttempt(ctx, rawURL, token, true)
}

func (f *Fetcher) fetchWithAuthAttempt(ctx context.Context, rawURL, token string, allowRetry bool) (*FetchResult, error) {
	if err := validateFetchURL(rawURL); err != nil {
		return nil, err
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
	if cached, ok := f.etagCache.Load(rawURL); ok {
		entry := cached.(*etagCacheEntry)
		if entry.etag != "" {
			req.Header.Set("If-None-Match", entry.etag)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlockedHost) {
			return nil, ErrBlockedHost
		}
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if count, parseErr := strconv.Atoi(remaining); parseErr == nil && count < 10 {
			log.Printf("WARN urlcontext: GitHub rate limit low: %d requests remaining", count)
		}
	}

	switch resp.StatusCode {
	case http.StatusNotModified:
		if cached, ok := f.etagCache.Load(rawURL); ok {
			entry := cached.(*etagCacheEntry)
			return &FetchResult{Body: entry.body, ContentType: entry.contentType, FinalURL: rawURL}, nil
		}
		return nil, fmt.Errorf("HTTP 304 but no cached body for %s", rawURL)

	case http.StatusTooManyRequests:
		if allowRetry {
			waitSeconds := 5
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if parsed, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					waitSeconds = parsed
				}
			}
			if waitSeconds >= 0 && waitSeconds <= 10 {
				timer := time.NewTimer(time.Duration(waitSeconds) * time.Second)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return f.fetchWithAuthAttempt(ctx, rawURL, token, false)
			}
		}
		return nil, ErrGitHubRateLimited

	case http.StatusForbidden:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if strings.Contains(strings.ToLower(string(body)), "rate limit") {
			return nil, ErrGitHubRateLimited
		}
		return nil, ErrGitHubPrivate

	case http.StatusNotFound:
		return nil, ErrGitHubPrivate
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, truncated, err := readLimitedBody(resp.Body, f.maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if etag := resp.Header.Get("ETag"); etag != "" {
		f.etagCache.Store(rawURL, &etagCacheEntry{
			etag:        etag,
			body:        append([]byte(nil), body...),
			contentType: resp.Header.Get("Content-Type"),
		})
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

func readLimitedBody(reader io.Reader, configuredLimit int64) ([]byte, bool, error) {
	limit := configuredLimit
	if limit <= 0 {
		limit = 750_000
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, false, err
	}
	truncated := int64(len(body)) > limit
	if truncated {
		body = body[:limit]
	}
	return body, truncated, nil
}

func isGitHubURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".github.com")
}
