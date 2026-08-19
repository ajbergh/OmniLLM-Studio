package sandbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBrokeredHTTPTimeout       = 30 * time.Second
	defaultBrokeredHTTPResponseBytes = int64(1 << 20)
	maxBrokeredHTTPRequestBytes      = 1 << 20
	maxBrokeredHTTPRedirects         = 5
)

// BrokeredHTTPRequest is trusted host-side egress executed on behalf of a
// sandbox/tool invocation. GrantID is always resolved against the exact owner.
// CredentialHandleID, when present, is redeemed only through a destination-bound
// credential consumer; raw secret material is never returned in this request or
// a BrokeredHTTPResponse.
type BrokeredHTTPRequest struct {
	GrantID            string
	Method             string
	URL                string
	Headers            map[string]string
	Body               []byte
	CredentialHandleID string
}

// BrokeredHTTPResponse is deliberately bounded. Headers that can carry cookies
// or authentication state are omitted before the response leaves trusted host
// code.
type BrokeredHTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       []byte              `json:"body,omitempty"`
}

// HTTPCredentialConsumer binds one credential service to exact DNS hostnames and
// one host-side header injection rule. The model cannot register consumers.
type HTTPCredentialConsumer struct {
	Service string
	Domains []string
	Header  string
	Prefix  string
}

type brokeredHTTPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// BrokeredHTTPClient provides the first destination-enforced egress path for
// sandbox workflows without granting arbitrary sandbox processes socket access.
// It resolves every connection itself, rejects non-public addresses, disables
// ambient HTTP proxies, revalidates redirects, and pins each dial to one of the
// IP addresses returned by the immediately preceding DNS resolution.
type BrokeredHTTPClient struct {
	grants      *NetworkGrantStore
	credentials *CredentialBroker
	resolver    brokeredHTTPResolver
	dialer      *net.Dialer
	timeout     time.Duration
	maxResponse int64

	mu        sync.RWMutex
	consumers map[string]HTTPCredentialConsumer
}

func NewBrokeredHTTPClient(grants *NetworkGrantStore, credentials *CredentialBroker) *BrokeredHTTPClient {
	if grants == nil {
		grants = DefaultNetworkGrantStore()
	}
	if credentials == nil {
		credentials = DefaultCredentialBroker()
	}
	return &BrokeredHTTPClient{
		grants:      grants,
		credentials: credentials,
		resolver:    net.DefaultResolver,
		dialer:      &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
		timeout:     defaultBrokeredHTTPTimeout,
		maxResponse: defaultBrokeredHTTPResponseBytes,
		consumers:   make(map[string]HTTPCredentialConsumer),
	}
}

// RegisterCredentialConsumer installs a trusted service-specific binding. The
// same service name must already be registered with CredentialBroker before a
// handle can be issued. Consumers may only target exact DNS hostnames; wildcard
// credential destinations are intentionally rejected.
func (c *BrokeredHTTPClient) RegisterCredentialConsumer(consumer HTTPCredentialConsumer) error {
	service := normalizeCredentialService(consumer.Service)
	if service == "" {
		return fmt.Errorf("credential consumer service is required")
	}
	header := http.CanonicalHeaderKey(strings.TrimSpace(consumer.Header))
	if header != "Authorization" && header != "X-Api-Key" {
		return fmt.Errorf("credential consumer header must be Authorization or X-Api-Key")
	}
	if len(consumer.Domains) == 0 || len(consumer.Domains) > 16 {
		return fmt.Errorf("credential consumer requires 1-16 exact domains")
	}
	seen := map[string]struct{}{}
	domains := make([]string, 0, len(consumer.Domains))
	for _, raw := range consumer.Domains {
		domain := normalizeBrokeredHTTPDomain(raw)
		if domain == "" || net.ParseIP(domain) != nil || !dnsNamePattern.MatchString(domain) {
			return fmt.Errorf("invalid credential consumer domain %q", raw)
		}
		if strings.Contains(domain, "*") {
			return fmt.Errorf("credential consumer domains must be exact")
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	consumer.Service = service
	consumer.Header = header
	consumer.Domains = domains
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.consumers[service]; exists {
		return fmt.Errorf("credential consumer %q is already registered", service)
	}
	c.consumers[service] = consumer
	return nil
}

func (c *BrokeredHTTPClient) Do(ctx context.Context, owner OwnerScope, request BrokeredHTTPRequest) (*BrokeredHTTPResponse, error) {
	if c == nil || c.grants == nil || c.credentials == nil || c.resolver == nil || c.dialer == nil {
		return nil, fmt.Errorf("brokered HTTP client is not configured")
	}
	grant, err := c.grants.Resolve(owner, request.GrantID)
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil, fmt.Errorf("unsupported brokered HTTP method %q", method)
	}
	if len(request.Body) > maxBrokeredHTTPRequestBytes {
		return nil, fmt.Errorf("brokered HTTP request body exceeds %d bytes", maxBrokeredHTTPRequestBytes)
	}
	parsed, err := url.Parse(strings.TrimSpace(request.URL))
	if err != nil {
		return nil, fmt.Errorf("parse brokered HTTP URL: %w", err)
	}
	if err := validateBrokeredHTTPURL(parsed, grant); err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(request.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range request.Headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if canonical == "" || brokeredHTTPHeaderForbidden(canonical) {
			return nil, fmt.Errorf("brokered HTTP header %q is not permitted", key)
		}
		if strings.ContainsAny(value, "\r\n\x00") {
			return nil, fmt.Errorf("brokered HTTP header %q contains invalid characters", key)
		}
		httpRequest.Header.Set(canonical, value)
	}

	credentialHost := ""
	if handleID := strings.TrimSpace(request.CredentialHandleID); handleID != "" {
		consumer, secret, err := c.resolveCredentialConsumer(ctx, owner, handleID, parsed)
		if err != nil {
			return nil, err
		}
		httpRequest.Header.Set(consumer.Header, consumer.Prefix+secret)
		credentialHost = normalizeBrokeredHTTPDomain(parsed.Hostname())
	}

	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: c.timeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		host, portText, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid brokered HTTP dial address: %w", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			return nil, fmt.Errorf("invalid brokered HTTP dial port")
		}
		if err := requireGrantDestination(grant, host, port); err != nil {
			return nil, err
		}
		ips, err := c.resolvePublicIPs(dialCtx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := c.dialer.DialContext(dialCtx, network, net.JoinHostPort(ip.String(), portText))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("no public addresses available")
		}
		return nil, lastErr
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{Transport: transport, Timeout: c.timeout}
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxBrokeredHTTPRedirects {
			return fmt.Errorf("brokered HTTP redirect limit exceeded")
		}
		if err := validateBrokeredHTTPURL(next.URL, grant); err != nil {
			return err
		}
		if credentialHost != "" {
			if next.URL.Scheme != "https" || normalizeBrokeredHTTPDomain(next.URL.Hostname()) != credentialHost {
				return fmt.Errorf("credential-bearing brokered HTTP requests cannot redirect to another origin")
			}
		}
		return nil
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("brokered HTTP request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, c.maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read brokered HTTP response: %w", err)
	}
	if int64(len(body)) > c.maxResponse {
		return nil, fmt.Errorf("brokered HTTP response exceeds %d bytes", c.maxResponse)
	}
	return &BrokeredHTTPResponse{
		StatusCode: response.StatusCode,
		Headers:    safeBrokeredHTTPResponseHeaders(response.Header),
		Body:       body,
	}, nil
}

func (c *BrokeredHTTPClient) resolveCredentialConsumer(ctx context.Context, owner OwnerScope, handleID string, target *url.URL) (HTTPCredentialConsumer, string, error) {
	service, secret, err := c.credentials.RedeemForService(ctx, owner, handleID)
	if err != nil {
		return HTTPCredentialConsumer{}, "", err
	}
	c.mu.RLock()
	consumer, ok := c.consumers[service]
	c.mu.RUnlock()
	if !ok {
		return HTTPCredentialConsumer{}, "", fmt.Errorf("credential service %q has no brokered HTTP consumer", service)
	}
	if target.Scheme != "https" {
		return HTTPCredentialConsumer{}, "", fmt.Errorf("credential-bearing brokered HTTP requests require HTTPS")
	}
	host := normalizeBrokeredHTTPDomain(target.Hostname())
	allowed := false
	for _, domain := range consumer.Domains {
		if host == domain {
			allowed = true
			break
		}
	}
	if !allowed {
		return HTTPCredentialConsumer{}, "", fmt.Errorf("credential service %q is not permitted for destination %q", service, host)
	}
	return consumer, secret, nil
}

func (c *BrokeredHTTPClient) resolvePublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	host = normalizeBrokeredHTTPDomain(host)
	if host == "" || net.ParseIP(host) != nil {
		return nil, fmt.Errorf("brokered HTTP destination must be a DNS hostname")
	}
	resolved, err := c.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve brokered HTTP destination %q: %w", host, err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("brokered HTTP destination %q resolved to no addresses", host)
	}
	seen := map[string]struct{}{}
	ips := make([]net.IP, 0, len(resolved))
	for _, item := range resolved {
		ip := item.IP
		if !isPublicBrokeredHTTPIP(ip) {
			return nil, fmt.Errorf("brokered HTTP destination %q resolved to prohibited address %q", host, ip.String())
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ips = append(ips, append(net.IP(nil), ip...))
	}
	return ips, nil
}

func validateBrokeredHTTPURL(target *url.URL, grant *NetworkGrant) error {
	if target == nil || grant == nil {
		return fmt.Errorf("brokered HTTP URL and grant are required")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("brokered HTTP supports only http and https")
	}
	if target.User != nil {
		return fmt.Errorf("brokered HTTP URL userinfo is not permitted")
	}
	if target.Fragment != "" {
		return fmt.Errorf("brokered HTTP URL fragments are not permitted")
	}
	host := normalizeBrokeredHTTPDomain(target.Hostname())
	if host == "" || net.ParseIP(host) != nil || !dnsNamePattern.MatchString(host) {
		return fmt.Errorf("brokered HTTP destination must be an approved DNS hostname")
	}
	port := 443
	if target.Scheme == "http" {
		port = 80
	}
	if explicit := target.Port(); explicit != "" {
		parsed, err := strconv.Atoi(explicit)
		if err != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("invalid brokered HTTP destination port")
		}
		port = parsed
	}
	return requireGrantDestination(grant, host, port)
}

func requireGrantDestination(grant *NetworkGrant, host string, port int) error {
	if grant == nil {
		return fmt.Errorf("network grant is required")
	}
	host = normalizeBrokeredHTTPDomain(host)
	hostAllowed := false
	for _, domain := range grant.Domains {
		if host == normalizeBrokeredHTTPDomain(domain) {
			hostAllowed = true
			break
		}
	}
	if !hostAllowed {
		return fmt.Errorf("destination %q is outside the approved network grant", host)
	}
	for _, allowedPort := range grant.Ports {
		if port == allowedPort {
			return nil
		}
	}
	return fmt.Errorf("destination port %d is outside the approved network grant", port)
}

func isPublicBrokeredHTTPIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return ip.IsGlobalUnicast()
}

func brokeredHTTPHeaderForbidden(header string) bool {
	switch http.CanonicalHeaderKey(header) {
	case "Authorization", "Proxy-Authorization", "Cookie", "Host", "Connection", "Proxy-Connection", "Upgrade", "Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto":
		return true
	default:
		return false
	}
}

func safeBrokeredHTTPResponseHeaders(headers http.Header) map[string][]string {
	allowed := map[string]struct{}{
		"Content-Type": {}, "Content-Length": {}, "Content-Encoding": {}, "Etag": {},
		"Last-Modified": {}, "Cache-Control": {}, "Location": {}, "Retry-After": {},
	}
	out := make(map[string][]string)
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if _, ok := allowed[canonical]; !ok {
			continue
		}
		copyValues := append([]string(nil), values...)
		for i := range copyValues {
			if len(copyValues[i]) > 4096 {
				copyValues[i] = copyValues[i][:4096]
			}
		}
		out[canonical] = copyValues
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeBrokeredHTTPDomain(raw string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
}

var defaultBrokeredHTTPClient = NewBrokeredHTTPClient(nil, nil)

func DefaultBrokeredHTTPClient() *BrokeredHTTPClient { return defaultBrokeredHTTPClient }
