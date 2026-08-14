package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/go-rod/rod/lib/launcher"
)

func TestValidateURLBlocksUnsafeSchemesAndPrivateAddresses(t *testing.T) {
	t.Parallel()

	cases := []string{
		"file:///etc/passwd",
		"chrome://version",
		"http://127.0.0.1:8080",
		"http://10.0.0.10",
		"http://192.168.1.10",
		"http://[::1]/",
	}

	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if err := validateURL(context.Background(), raw); err == nil {
				t.Fatalf("validateURL(%q) returned nil error", raw)
			}
		})
	}
}

func TestValidateURLAllowsPublicIPAddress(t *testing.T) {
	t.Parallel()

	if err := validateURL(context.Background(), "https://93.184.216.34/"); err != nil {
		t.Fatalf("expected public IP URL to pass, got %v", err)
	}
}

func TestIsBlockedIP(t *testing.T) {
	t.Parallel()

	blocked := []string{"127.0.0.1", "10.2.3.4", "172.16.0.1", "192.168.1.1", "::1", "fe80::1"}
	for _, raw := range blocked {
		if !isBlockedIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"}
	for _, raw := range allowed {
		if isBlockedIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be allowed", raw)
		}
	}
}

func TestDetectBotProtection(t *testing.T) {
	t.Parallel()

	blockedText := []string{
		"Checking your browser before accessing the site",
		"cf-browser-verification is running",
		"Verify you are human",
		"Access denied",
		"Please enable JavaScript and cookies",
	}
	for _, text := range blockedText {
		if !detectBotProtection(text) {
			t.Fatalf("expected bot-protection signal for %q", text)
		}
	}

	if detectBotProtection(strings.Repeat("ordinary article content ", 10)) {
		t.Fatal("ordinary article text was detected as bot protection")
	}
}

func TestRequestPerimeterBlocksRejectedSubresource(t *testing.T) {
	browserPath, found := launcher.LookPath()
	if !found {
		t.Skip("no local Chromium-compatible browser found")
	}

	var blockedRequests atomic.Int32
	blockedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blockedRequests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer blockedServer.Close()

	var blockedValidations atomic.Int32
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<html><body>safe<script>fetch(%q).catch(() => {})</script></body></html>`, blockedServer.URL+"/private")
	}))
	defer pageServer.Close()

	manager := NewManager(&config.Config{
		BrowserEnabled:     true,
		BrowserExecPath:    browserPath,
		BrowserCacheDir:    t.TempDir(),
		BrowserMaxSessions: 1,
		BrowserSessionTTL:  time.Minute,
	}, nil)
	manager.validate = func(_ context.Context, raw string) error {
		if strings.HasPrefix(raw, blockedServer.URL) {
			blockedValidations.Add(1)
			return errors.New("test policy rejected destination")
		}
		return nil
	}
	navigated := false
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Shutdown(shutdownCtx); err != nil && navigated {
			t.Errorf("shutdown browser: %v", err)
		}
	})

	if _, err := manager.NavigatePage(context.Background(), NavigateOptions{
		URL: pageServer.URL, Extract: "text",
	}); err != nil {
		if message := strings.ToLower(err.Error()); strings.Contains(message, "launch chromium") || strings.Contains(message, "create stealth page") {
			t.Skipf("local browser could not start: %v", err)
		}
		t.Fatalf("navigate test page: %v", err)
	}
	navigated = true
	if blockedValidations.Load() == 0 {
		t.Fatal("subresource URL did not reach the request perimeter")
	}
	if got := blockedRequests.Load(); got != 0 {
		t.Fatalf("blocked subresource reached its server %d time(s)", got)
	}
}
