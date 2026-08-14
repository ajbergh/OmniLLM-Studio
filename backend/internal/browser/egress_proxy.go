package browser

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	browserProxyDialTimeout           = 10 * time.Second
	browserProxyResponseHeaderTimeout = 30 * time.Second
)

type targetResolver func(context.Context, string, string) ([]net.IP, error)

type browserEgressProxy struct {
	listener net.Listener
	server   *http.Server
	resolve  targetResolver

	tunnelMu sync.Mutex
	tunnels  map[net.Conn]struct{}
	closed   bool
}

func startBrowserEgressProxy(resolve targetResolver) (*browserEgressProxy, error) {
	if resolve == nil {
		resolve = resolvePublicTarget
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for browser egress proxy: %w", err)
	}
	proxy := &browserEgressProxy{
		listener: listener,
		resolve:  resolve,
		tunnels:  make(map[net.Conn]struct{}),
	}
	proxy.server = &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		_ = proxy.server.Serve(listener)
	}()
	return proxy, nil
}

func (p *browserEgressProxy) URL() string {
	if p == nil || p.listener == nil {
		return ""
	}
	return "http://" + p.listener.Addr().String()
}

func (p *browserEgressProxy) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.tunnelMu.Lock()
	p.closed = true
	for conn := range p.tunnels {
		_ = conn.Close()
	}
	p.tunnels = make(map[net.Conn]struct{})
	p.tunnelMu.Unlock()
	return p.server.Shutdown(ctx)
}

func (p *browserEgressProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Method, http.MethodConnect) {
		p.serveConnect(w, r)
		return
	}
	p.serveForward(w, r)
}

func (p *browserEgressProxy) serveForward(w http.ResponseWriter, r *http.Request) {
	if r.URL == nil || r.URL.Host == "" {
		http.Error(w, "absolute proxy URL is required", http.StatusBadRequest)
		return
	}
	if r.URL.User != nil {
		http.Error(w, "URL credentials are not allowed", http.StatusForbidden)
		return
	}
	scheme := strings.ToLower(r.URL.Scheme)
	transportScheme := scheme
	switch scheme {
	case "http", "https":
	case "ws":
		transportScheme = "http"
	case "wss":
		transportScheme = "https"
	default:
		http.Error(w, "unsupported proxy URL scheme", http.StatusForbidden)
		return
	}
	host, port, err := targetHostPort(r.URL.Host, transportScheme)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ips, err := p.resolveWithTimeout(r.Context(), host, port)
	if err != nil {
		http.Error(w, "destination blocked", http.StatusForbidden)
		return
	}

	targetURL := *r.URL
	targetURL.Scheme = transportScheme
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            pinnedDialer(ips, port),
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      false,
		TLSHandshakeTimeout:    browserProxyDialTimeout,
		ResponseHeaderTimeout:  browserProxyResponseHeaderTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 1 << 20,
	}
	defer transport.CloseIdleConnections()
	reverseProxy := &httputil.ReverseProxy{
		Director: func(out *http.Request) {
			out.URL = &targetURL
			out.Host = targetURL.Host
			out.RequestURI = ""
			out.Header.Del("Proxy-Authorization")
			out.Header.Del("Proxy-Connection")
		},
		Transport: transport,
		ErrorHandler: func(rw http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(rw, "browser proxy upstream failure", http.StatusBadGateway)
		},
	}
	reverseProxy.ServeHTTP(w, r)
}

func (p *browserEgressProxy) serveConnect(w http.ResponseWriter, r *http.Request) {
	host, port, err := splitAuthority(r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ips, err := p.resolveWithTimeout(r.Context(), host, port)
	if err != nil {
		http.Error(w, "destination blocked", http.StatusForbidden)
		return
	}
	upstream, err := pinnedDialer(ips, port)(r.Context(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		http.Error(w, "browser proxy upstream failure", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "proxy tunneling is unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	if !p.registerTunnels(client, upstream) {
		_ = client.Close()
		_ = upstream.Close()
		return
	}
	defer func() {
		p.unregisterTunnels(client, upstream)
		_ = client.Close()
		_ = upstream.Close()
	}()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	if buffered.Reader.Buffered() > 0 {
		if _, err := io.CopyN(upstream, buffered, int64(buffered.Reader.Buffered())); err != nil {
			return
		}
	}
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
	_ = client.Close()
	_ = upstream.Close()
	<-done
}

func (p *browserEgressProxy) resolveWithTimeout(parent context.Context, host, port string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(parent, browserRequestValidationTimeout)
	defer cancel()
	return p.resolve(ctx, host, port)
}

func (p *browserEgressProxy) registerTunnels(conns ...net.Conn) bool {
	p.tunnelMu.Lock()
	defer p.tunnelMu.Unlock()
	if p.closed {
		return false
	}
	for _, conn := range conns {
		p.tunnels[conn] = struct{}{}
	}
	return true
}

func (p *browserEgressProxy) unregisterTunnels(conns ...net.Conn) {
	p.tunnelMu.Lock()
	defer p.tunnelMu.Unlock()
	for _, conn := range conns {
		delete(p.tunnels, conn)
	}
}

func resolvePublicTarget(ctx context.Context, host, _ string) ([]net.IP, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("DNS resolution returned no IPs")
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		if isBlockedIP(address.IP) {
			return nil, fmt.Errorf("blocked private/internal IP %s resolved from %s", address.IP, host)
		}
		ips = append(ips, append(net.IP(nil), address.IP...))
	}
	return ips, nil
}

func pinnedDialer(ips []net.IP, port string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var lastErr error
		for _, ip := range ips {
			dialer := net.Dialer{Timeout: browserProxyDialTimeout, KeepAlive: 30 * time.Second}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("destination resolved to no dialable addresses")
		}
		return nil, lastErr
	}
}

func targetHostPort(authority, scheme string) (string, string, error) {
	if host, port, err := net.SplitHostPort(authority); err == nil {
		if strings.TrimSpace(host) == "" {
			return "", "", fmt.Errorf("destination host is required")
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		return strings.TrimSpace(host), port, nil
	}
	host := strings.Trim(strings.TrimSpace(authority), "[]")
	if host == "" {
		return "", "", fmt.Errorf("destination host is required")
	}
	switch scheme {
	case "http":
		return host, "80", nil
	case "https":
		return host, "443", nil
	default:
		return "", "", fmt.Errorf("destination port is required")
	}
}

func splitAuthority(authority string) (string, string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(authority))
	if err != nil || strings.TrimSpace(host) == "" {
		return "", "", fmt.Errorf("CONNECT authority must include a host and port")
	}
	if err := validatePort(port); err != nil {
		return "", "", err
	}
	return host, port, nil
}

func validatePort(port string) error {
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return fmt.Errorf("destination port is invalid")
	}
	return nil
}
