package browser

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBrowserEgressProxyBlocksPrivateHTTP(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	proxy := newTestBrowserEgressProxy(t, resolvePublicTarget)
	client := proxyClient(t, proxy)
	response, err := client.Get(target.URL + "/private")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("private destination received %d request(s)", got)
	}
}

func TestBrowserEgressProxyPinsResolvedHTTPAddress(t *testing.T) {
	var observedHost string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedHost = r.Host
		_, _ = io.WriteString(w, "ok")
	}))
	defer target.Close()
	_, port := testTargetHostPort(t, target.URL)

	proxy := newTestBrowserEgressProxy(t, pinnedTestResolver(t, "public.example", port))
	response, err := proxyClient(t, proxy).Get("http://public.example:" + port + "/resource")
	if err != nil {
		t.Fatalf("request through pinned proxy: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("response = %d %q, want 200 ok", response.StatusCode, body)
	}
	if observedHost != "public.example:"+port {
		t.Fatalf("upstream Host = %q", observedHost)
	}
}

func TestBrowserEgressProxyRevalidatesRedirectDestination(t *testing.T) {
	var privateHits atomic.Int32
	privateTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		privateHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer privateTarget.Close()

	publicTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, privateTarget.URL+"/private", http.StatusFound)
	}))
	defer publicTarget.Close()
	_, publicPort := testTargetHostPort(t, publicTarget.URL)

	proxy := newTestBrowserEgressProxy(t, func(ctx context.Context, host, port string) ([]net.IP, error) {
		if host == "public.example" && port == publicPort {
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		return resolvePublicTarget(ctx, host, port)
	})
	response, err := proxyClient(t, proxy).Get("http://public.example:" + publicPort + "/redirect")
	if err != nil {
		t.Fatalf("redirect request through proxy: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("redirect response = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if got := privateHits.Load(); got != 0 {
		t.Fatalf("private redirect destination received %d request(s)", got)
	}
}

func TestBrowserEgressProxyPinsConnectTunnel(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for tunnel target: %v", err)
	}
	defer target.Close()
	_, port, err := net.SplitHostPort(target.Addr().String())
	if err != nil {
		t.Fatalf("split target address: %v", err)
	}
	targetDone := make(chan error, 1)
	go func() {
		conn, acceptErr := target.Accept()
		if acceptErr != nil {
			targetDone <- acceptErr
			return
		}
		defer conn.Close()
		payload := make([]byte, 4)
		if _, readErr := io.ReadFull(conn, payload); readErr != nil {
			targetDone <- readErr
			return
		}
		_, writeErr := conn.Write(payload)
		targetDone <- writeErr
	}()

	proxy := newTestBrowserEgressProxy(t, pinnedTestResolver(t, "public.example", port))
	proxyAddress := strings.TrimPrefix(proxy.URL(), "http://")
	conn, err := net.DialTimeout("tcp", proxyAddress, 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "CONNECT public.example:%s HTTP/1.1\r\nHost: public.example:%s\r\n\r\n", port, port); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status = %d, want 200", response.StatusCode)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write tunnel payload: %v", err)
	}
	payload := make([]byte, 4)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Fatalf("read tunnel payload: %v", err)
	}
	if string(payload) != "ping" {
		t.Fatalf("tunnel payload = %q", payload)
	}
	if err := <-targetDone; err != nil {
		t.Fatalf("tunnel target: %v", err)
	}
}

func TestBrowserEgressProxyBlocksPrivateConnect(t *testing.T) {
	proxy := newTestBrowserEgressProxy(t, resolvePublicTarget)
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(proxy.URL(), "http://"), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if _, err := fmt.Fprint(conn, "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n"); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("CONNECT status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

func TestBrowserEgressProxyForwardsWebSocketUpgrade(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "upgrade required", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
			return
		}
		conn, buffered, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		_ = buffered.Flush()
	}))
	defer target.Close()
	_, port := testTargetHostPort(t, target.URL)

	proxy := newTestBrowserEgressProxy(t, pinnedTestResolver(t, "public.example", port))
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(proxy.URL(), "http://"), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "GET ws://public.example:%s/socket HTTP/1.1\r\nHost: public.example:%s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n", port, port); err != nil {
		t.Fatalf("write websocket request: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatalf("read websocket response: %v", err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("websocket status = %d, want 101", response.StatusCode)
	}
}

func newTestBrowserEgressProxy(t *testing.T, resolve targetResolver) *browserEgressProxy {
	t.Helper()
	proxy, err := startBrowserEgressProxy(resolve)
	if err != nil {
		t.Fatalf("start browser egress proxy: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proxy.Close(ctx); err != nil {
			t.Errorf("close browser egress proxy: %v", err)
		}
	})
	return proxy
}

func proxyClient(t *testing.T, proxy *browserEgressProxy) *http.Client {
	t.Helper()
	parsed, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatalf("parse proxy URL: %v", err)
	}
	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(parsed), DisableKeepAlives: true},
		Timeout:   10 * time.Second,
	}
}

func pinnedTestResolver(t *testing.T, expectedHost, expectedPort string) targetResolver {
	t.Helper()
	return func(_ context.Context, host, port string) ([]net.IP, error) {
		if host != expectedHost || port != expectedPort {
			return nil, fmt.Errorf("unexpected target %s:%s", host, port)
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
}

func testTargetHostPort(t *testing.T, raw string) (string, string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse target URL: %v", err)
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split target URL: %v", err)
	}
	return host, port
}
