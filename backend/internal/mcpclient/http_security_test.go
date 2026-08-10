package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPClientBlocksPrivateNetworkByDefault(t *testing.T) {
	server := newTestHTTPServer(t, nil)
	defer server.Close()

	config := testMCPServer(server.URL)
	config.AllowPrivateNetwork = false
	client := NewHTTPClient(config)
	err := client.Start(context.Background())
	if err == nil {
		t.Fatal("expected private loopback MCP server to be blocked")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "blocked") {
		t.Fatalf("expected blocked-address error, got %v", err)
	}
}

func TestHTTPClientAllowsPrivateNetworkOnlyWhenExplicit(t *testing.T) {
	server := newTestHTTPServer(t, nil)
	defer server.Close()

	config := testMCPServer(server.URL)
	config.AllowPrivateNetwork = true
	client := NewHTTPClient(config)
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("explicit private-network MCP server should start: %v", err)
	}
}

func TestHTTPClientDoesNotFollowRedirects(t *testing.T) {
	var targetHits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	config := testMCPServer(redirector.URL)
	config.AllowPrivateNetwork = true
	config.Headers = map[string]string{"Authorization": "Bearer must-not-forward"}
	client := NewHTTPClient(config)
	err := client.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") {
		t.Fatalf("expected redirect response to be rejected, got %v", err)
	}
	if targetHits.Load() != 0 {
		t.Fatalf("redirect target received %d request(s); expected none", targetHits.Load())
	}
}

func TestValidateHTTPServerURL(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "https", url: "https://example.com/mcp", ok: true},
		{name: "http", url: "http://example.com/mcp", ok: true},
		{name: "ftp", url: "ftp://example.com/mcp"},
		{name: "userinfo", url: "https://user:pass@example.com/mcp"},
		{name: "fragment", url: "https://example.com/mcp#secret"},
		{name: "missing host", url: "https:///mcp"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateHTTPServerURL(test.url)
			if test.ok && err != nil {
				t.Fatalf("expected valid URL, got %v", err)
			}
			if !test.ok && err == nil {
				t.Fatalf("expected URL %q to be rejected", test.url)
			}
		})
	}
}
