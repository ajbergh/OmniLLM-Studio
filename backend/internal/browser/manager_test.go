package browser

import (
	"context"
	"net"
	"strings"
	"testing"
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
